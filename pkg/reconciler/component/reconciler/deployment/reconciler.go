// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	rcrd "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/crd"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/render"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

const (
	defaultReplicas = 1

	ConditionTypeDeploymentReady = "DeploymentReady"
	ConditionTypeServiceReady    = "ServiceReady"
	ConditionTypeReady           = "Ready"
)

type ComponentReconciler interface {
	reconcile.Reconciler
	NewReconciled() ReconciledObject
}

func NewComponentReconciler(ctx context.Context, crd *rcrd.Registered, reg common.Registration, mgr manager.Manager) (ComponentReconciler, error) {
	gvk := crd.GetGVK()
	log := mgr.GetLogger().WithName(gvk.String())

	smf := rcrd.NewStatusManagerFactory(crd.GetStatusFlag(), "Ready", []string{ConditionTypeDeploymentReady, ConditionTypeServiceReady, ConditionTypeReady}, log)

	r := &reconciler{
		log:          log,
		crd:          crd,
		smf:          smf,
		registration: reg,
		psr:          render.NewPodSpecRenderer("adapter", reg.GetWorkload().FromImage.Repo),
	}

	if err := builder.ControllerManagedBy(mgr).
		For(r.NewReconciled().GetObject()).
		Owns(resources.NewDeployment("", "")).
		Owns(resources.NewService("", "")).
		Complete(r); err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", gvk.String(), err)
	}

	return r, nil
}

type reconciler struct {
	crd          *rcrd.Registered
	registration common.Registration
	psr          render.PodSpecRenderer
	smf          rcrd.StatusManagerFactory

	client client.Client
	log    logr.Logger
}

var _ ComponentReconciler = (*reconciler)(nil)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.V(1).Info("reconciling request", "request", req)

	ro := r.NewReconciled()
	obj := ro.GetObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		// Return and let the ownership clean
		// owned resources.
		return reconcile.Result{}, nil
	}

	if r.crd.GetStatusFlag().AllowObservedGeneration() {
		r.log.V(1).Info("updating observed generation")
		ro.SetStatusObservedGeneration(obj.GetGeneration())
		if err := r.client.Status().Update(ctx, obj); err != nil {
			return reconcile.Result{}, err
		}
		r.log.V(1).Info("updated observed generation")
	}

	d, err := r.reconcileDeployment(ctx, obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	if r.crd.GetStatusFlag().AllowConditions() {
		reason := "DeploymentUnknown"
		status := metav1.ConditionUnknown
		message := ""
		for _, c := range d.Status.Conditions {
			if c.Type != appsv1.DeploymentAvailable {
				continue
			}
			switch c.Status {
			case corev1.ConditionTrue:
				status = metav1.ConditionTrue
				reason = c.Reason

			case corev1.ConditionFalse:
				status = metav1.ConditionFalse
				reason = c.Reason
				message = c.Message

			}

		}

		ro.SetStatusCondition(ConditionTypeDeploymentReady, status, reason, message)
		if err := r.client.Status().Update(ctx, obj); err != nil {
			return reconcile.Result{}, err
		}
		r.log.V(1).Info("updated conditions")
	}

	if r.registration.GetWorkload().FormFactor.Deployment.Service != nil {
		_, err = r.reconcileService(ctx, obj)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) reconcileDeployment(ctx context.Context, obj client.Object) (*appsv1.Deployment, error) {
	desired, err := r.createDeploymentFromRegistered(obj)
	if err != nil {
		return nil, fmt.Errorf("could not render deployment object: %w", err)
	}

	r.log.V(5).Info("desired deployment object", "object", *desired)

	existing := &appsv1.Deployment{}
	err = r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		if semantic.Semantic.DeepEqual(desired, existing) {
			return existing, nil
		}

		r.log.Info("existing deployment does not match the expected", "object", desired)
		r.log.V(5).Info("mismatched deployment", "desired", *desired, "existing", *existing)

		// resourceVersion must be returned to the API server unmodified for
		// optimistic concurrency, as per Kubernetes API conventions
		desired.SetResourceVersion(existing.GetResourceVersion())

		if err = r.client.Update(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not update deployment object: %+w", err)
		}

	case apierrs.IsNotFound(err):
		r.log.Info("creating deployment", "object", desired)
		r.log.V(5).Info("desired deployment", "object", *desired)
		if err = r.client.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not create deployment object: %w", err)
		}

	default:
		return nil, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	// TODO update status
	return desired, nil
}

func (r *reconciler) createDeploymentFromRegistered(obj client.Object) (*appsv1.Deployment, error) {
	// TODO generate names

	// use parameter options to define parameters policy
	// use obj to gather
	ps, err := r.psr.Render(obj)
	if err != nil {
		return nil, err
	}

	replicas := defaultReplicas
	if ffd := r.registration.GetWorkload().FormFactor.Deployment; ffd != nil {
		replicas = ffd.Replicas
	}

	return resources.NewDeployment(obj.GetNamespace(), obj.GetName(),
		resources.DeploymentWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, r.registration.GetName()),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, render.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, render.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, render.ManagedBy),

			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.DeploymentSetReplicas(int32(replicas)),
		resources.DeploymentAddSelectorForTemplate(resources.AppNameLabel, r.registration.GetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppInstanceLabel, obj.GetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppComponentLabel, render.ComponentWorkload),

		resources.DeploymentWithTemplateSpecOptions(
			resources.PodTemplateSpecWithPodSpecOptions(ps...),
		)), nil
}

func (r *reconciler) reconcileService(ctx context.Context, obj client.Object) (*corev1.Service, error) {
	desired, err := r.createServiceFromRegistered(obj)
	if err != nil {
		return nil, fmt.Errorf("could not render service object: %w", err)
	}

	r.log.V(5).Info("desired service object", "object", *desired)

	existing := &corev1.Service{}
	err = r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		if semantic.Semantic.DeepEqual(desired, existing) {
			return desired, nil
		}

		r.log.Info("existing service does not match the expected", "object", desired)
		r.log.V(5).Info("mismatched service", "desired", *desired, "existing", *existing)

		// resourceVersion must be returned to the API server unmodified for
		// optimistic concurrency, as per Kubernetes API conventions
		desired.SetResourceVersion(existing.GetResourceVersion())

		if err = r.client.Update(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not update service object: %+w", err)
		}

	case apierrs.IsNotFound(err):
		r.log.Info("creating service", "object", desired)
		r.log.V(5).Info("desired service", "object", *desired)
		if err = r.client.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not create service object: %w", err)
		}
	default:
		return nil, fmt.Errorf("could not retrieve controlled service %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	// TODO update status
	return desired, nil
}

func (r *reconciler) createServiceFromRegistered(obj client.Object) (*corev1.Service, error) {
	// TODO generate names

	if r.registration.GetWorkload().FormFactor.Deployment == nil ||
		r.registration.GetWorkload().FormFactor.Deployment.Service == nil {
		return nil, errors.New("there is no service specification at the registration form factor")
	}
	ffscv := r.registration.GetWorkload().FormFactor.Deployment.Service

	return resources.NewService(obj.GetNamespace(), obj.GetName(),
		resources.ServiceWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, r.registration.GetName()),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, render.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, render.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, render.ManagedBy),
			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.ServiceAddSelectorLabel(resources.AppNameLabel, r.registration.GetName()),
		resources.ServiceAddSelectorLabel(resources.AppInstanceLabel, obj.GetName()),
		resources.ServiceAddSelectorLabel(resources.AppComponentLabel, render.ComponentWorkload),
		resources.ServiceAddPort("", ffscv.Port, ffscv.TargetPort),
	), nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectLogger(l logr.Logger) error {
	r.log = l.WithName("dynrecl")
	l.V(2).Info("logger injected into dynamic component reconciler")
	return nil
}

type ReconciledObject interface {
	GetObject() client.Object
	SetStatusObservedGeneration(generation int64)
	SetStatusCondition(typ string, status metav1.ConditionStatus, reason, message string)
}

type reconciledObject struct {
	unstructured *unstructured.Unstructured
	sm           rcrd.StatusManager
}

func (ro *reconciledObject) SetStatusObservedGeneration(generation int64) {
	ro.sm.SetObservedGeneration(generation)
}

func (ro *reconciledObject) SetStatusCondition(typ string, status metav1.ConditionStatus, reason, message string) {
	ro.sm.SetCondition(typ, status, reason, message)
}

func (ro *reconciledObject) GetObject() client.Object {
	return ro.unstructured
}

func (r *reconciler) NewReconciled() ReconciledObject {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(r.crd.GetGVK())
	ro := &reconciledObject{
		unstructured: u,
		sm:           r.smf.ForObject(u),
	}

	return ro
}
