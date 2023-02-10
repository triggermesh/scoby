// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"context"
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

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	recbase "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base"

	"github.com/triggermesh/scoby/pkg/reconciler/resources"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

const (
	defaultReplicas = 1

	ConditionTypeDeploymentReady = "DeploymentReady"
	ConditionTypeServiceReady    = "ServiceReady"
	ConditionTypeReady           = "Ready"
)

// func NewComponentReconciler(ctx context.Context, crd *reccommon.Registered, reg common.Registration, mgr manager.Manager) (ComponentReconciler, error) {
func NewComponentReconciler(ctx context.Context, base recbase.Reconciler, mgr manager.Manager) (reconcile.Reconciler, error) {
	log := mgr.GetLogger().WithName(base.RegisteredGetName())
	log.Info("Creating deployment styled reconciler")

	r := &reconciler{
		log:  log,
		base: base,
	}

	// If a service associated to the deployment needs to be rendered, add the
	// status conditions and the parameters for the service.
	statusConditions := []string{ConditionTypeDeploymentReady}
	dff := base.RegisteredGetWorkload().FormFactor.Deployment
	if dff != nil && dff.Service != nil {
		statusConditions = append(statusConditions, ConditionTypeServiceReady)
		r.serviceOptions = dff.Service
	}

	base.StatusConfigureManagerConditions(recbase.ConditionTypeReady, statusConditions...)

	log.V(1).Info("Reconciler configured, adding to controller manager")

	if err := builder.ControllerManagedBy(mgr).
		For(base.NewReconciledObject().AsKubeObject()).
		Owns(resources.NewDeployment("", "")).
		Owns(resources.NewService("", "")).
		Complete(r); err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", base.RegisteredGetName(), err)
	}

	return r, nil
}

type reconciler struct {
	base           recbase.Reconciler
	serviceOptions *apicommon.DeploymentService

	client client.Client
	log    logr.Logger
}

var _ reconcile.Reconciler = (*reconciler)(nil)

func (r *reconciler) ReconcileX(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.V(1).Info("reconciling request", "request", req)

	obj := r.base.NewReconciledObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj.AsKubeObject()); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	r.log.V(5).Info("Object retrieved", "object", obj)

	if !obj.GetDeletionTimestamp().IsZero() {
		// Return and let the ownership clean resources.
		return reconcile.Result{}, nil
	}

	// // create a copy, we will compare after reconciling and decide if we need to
	// // update or not.
	cp := obj.AsKubeObject().DeepCopyObject()

	res, err := r.reconcileObjectInstance(ctx, obj)

	// Update status if needed.
	// TODO find a better expression for this
	if !semantic.Semantic.DeepEqual(
		obj.AsKubeObject().(*unstructured.Unstructured).Object["status"],
		cp.(*unstructured.Unstructured).Object["status"]) {
		if uperr := r.client.Status().Update(ctx, obj.AsKubeObject()); uperr != nil {
			if err == nil {
				return reconcile.Result{}, err
			}
			r.log.Error(err, "")
		}

	}

	return res, err
}

func (r *reconciler) reconcileObjectInstance(ctx context.Context, obj recbase.ReconciledObject) (reconcile.Result, error) {
	r.log.V(1).Info("reconciling object instance", "object", obj)

	// Update generation if needed

	if g := obj.GetGeneration(); g != obj.StatusGetObservedGeneration() {
		r.log.V(1).Info("updating observed generation", "generation", g)
		obj.StatusSetObservedGeneration(g)
	}

	r.log.V(1).Info("reconciling deployment", "object", obj)
	d, err := r.reconcileDeployment(ctx, obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	r.log.V(1).Info("updating deployment status", "object", obj)
	r.updateDeploymentStatus(obj, d)

	if r.serviceOptions != nil {
		r.log.V(1).Info("reconciling service", "object", obj)
		s, err := r.reconcileService(ctx, obj)
		if err != nil {
			return reconcile.Result{}, err
		}

		r.log.V(1).Info("updating deployment status", "object", obj)
		r.updateServiceStatus(obj, s)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) reconcileDeployment(ctx context.Context, obj recbase.ReconciledObject) (*appsv1.Deployment, error) {
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

	return desired, nil
}

func (r *reconciler) updateDeploymentStatus(obj recbase.ReconciledObject, d *appsv1.Deployment) {
	if d == nil {
		return
	}

	desired := &apicommon.Condition{
		Type:               ConditionTypeDeploymentReady,
		Reason:             "DeploymentUnknown",
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
	}

	for _, c := range d.Status.Conditions {
		if c.Type != appsv1.DeploymentAvailable {
			continue
		}
		switch c.Status {
		case corev1.ConditionTrue:
			desired.Status = metav1.ConditionTrue
			desired.Reason = c.Reason

		case corev1.ConditionFalse:
			desired.Status = metav1.ConditionFalse
			desired.Reason = c.Reason
			desired.Message = c.Message
		}
	}

	obj.StatusSetCondition(desired)
}

func (r *reconciler) createDeploymentFromRegistered(obj recbase.ReconciledObject) (*appsv1.Deployment, error) {
	// TODO generate names

	ps, err := obj.RenderPodSpecOptions()
	if err != nil {
		return nil, err
	}

	wkl := r.base.RegisteredGetWorkload()

	replicas := defaultReplicas
	if ffd := wkl.FormFactor.Deployment; ffd != nil {
		replicas = ffd.Replicas
	}

	return resources.NewDeployment(obj.GetNamespace(), obj.GetName(),
		resources.DeploymentWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, r.base.RegisteredGetName()),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, recbase.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, recbase.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, recbase.ManagedBy),

			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.DeploymentSetReplicas(int32(replicas)),
		resources.DeploymentAddSelectorForTemplate(resources.AppNameLabel, r.base.RegisteredGetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppInstanceLabel, obj.GetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppComponentLabel, recbase.ComponentWorkload),

		resources.DeploymentWithTemplateSpecOptions(
			resources.PodTemplateSpecWithPodSpecOptions(ps...),
		)), nil
}

func (r *reconciler) reconcileService(ctx context.Context, obj recbase.ReconciledObject) (*corev1.Service, error) {
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

	return desired, nil
}

func (r *reconciler) updateServiceStatus(obj recbase.ReconciledObject, s *corev1.Service) {
	desired := &apicommon.Condition{
		Type:               ConditionTypeServiceReady,
		Reason:             "ServiceExist",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}

	if s == nil {
		desired.Status = metav1.ConditionFalse
		desired.Reason = "ServiceDoesNotExist"
	}

	obj.StatusSetCondition(desired)
}

func (r *reconciler) createServiceFromRegistered(obj recbase.ReconciledObject) (*corev1.Service, error) {
	// TODO generate names

	return resources.NewService(obj.GetNamespace(), obj.GetName(),
		resources.ServiceWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, r.base.RegisteredGetName()),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, recbase.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, recbase.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, recbase.ManagedBy),
			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.ServiceAddSelectorLabel(resources.AppNameLabel, r.base.RegisteredGetName()),
		resources.ServiceAddSelectorLabel(resources.AppInstanceLabel, obj.GetName()),
		resources.ServiceAddSelectorLabel(resources.AppComponentLabel, recbase.ComponentWorkload),
		resources.ServiceAddPort("", r.serviceOptions.Port, r.serviceOptions.TargetPort),
	), nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
