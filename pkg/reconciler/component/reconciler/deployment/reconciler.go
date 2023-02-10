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

// type ComponentReconciler interface {
// 	reconcile.Reconciler
// 	NewReconciled() ReconciledObject
// }

// func NewComponentReconciler(ctx context.Context, crd *reccommon.Registered, reg common.Registration, mgr manager.Manager) (ComponentReconciler, error) {
func NewComponentReconciler(ctx context.Context, base recbase.Reconciler, mgr manager.Manager) (reconcile.Reconciler, error) {
	log := mgr.GetLogger().WithName(base.RegisteredGetName())
	log.Info("Creating deployment styled reconciler")

	// rof := reccommon.NewReconciledObjectFactory()

	// smf := reccommon.NewStatusManagerFactory(crd.GetStatusFlag(), "Ready", []string{ConditionTypeDeploymentReady, ConditionTypeServiceReady, ConditionTypeReady}, log)

	r := &reconciler{
		log:  log,
		base: base,
		// crd:          crd,
		// smf:          smf,
		// registration: reg,
		// psr:          render.NewPodSpecRenderer("adapter", reg.GetWorkload().FromImage.Repo),
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
	base recbase.Reconciler
	// objectFactory reccommon.ReconciledObjectFactory
	// crd           *reccommon.Registered
	// registration  common.Registration
	// psr           render.PodSpecRenderer
	// smf           reccommon.StatusManagerFactory
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
	// obj := ro.GetObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj.AsKubeObject()); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// existing := obj.DeepCopyObject()

	r.log.V(5).Info("Object retrieved", "object", obj)

	if !obj.GetDeletionTimestamp().IsZero() {
		// Return and let the ownership clean resources.
		return reconcile.Result{}, nil
	}

	// // create a copy, we will compare after reconciling and decide if we need to
	// // update or not.
	// cp := obj.DeepCopyObject()
	// cp.

	res, err := r.reconcileObjectInstance(ctx, obj)

	// TODO
	// TODO compare statuses and update them if needed

	// // Update status if needed.
	// //
	// // We need to compare the internal status, which is covered by the semantic
	// // comparer library
	// if !semantic.Semantic.DeepEqual(&cr.Status.Status, &existing.Status.Status) {
	// 	// The err variable is newly defined, if the update is unsuccessful
	// 	// the error returned will be the update operation error.
	// 	if err := r.Status().Update(ctx, cr); err != nil {
	// 		return ctrl.Result{}, err
	// 	}
	// }

	return res, err
}

func (r *reconciler) reconcileObjectInstance(ctx context.Context, obj recbase.ReconciledObject) (reconcile.Result, error) {
	// obj := ro.GetObject()
	r.log.V(1).Info("reconciling object instance", "object", obj)

	// // Update generation if needed
	// if r.base.StatusGetSupportFlag().AllowObservedGeneration() {
	// 	if g := obj.GetGeneration(); g != obj.StatusGetObservedGeneration() {
	// 		r.log.V(1).Info("updating observed generation")
	// 		obj.StatusSetObservedGeneration(g)
	// 	}

	// 	// if err := r.client.Status().Update(ctx, obj); err != nil {
	// 	// 	return reconcile.Result{}, err
	// 	// }
	// 	// r.log.V(1).Info("updated observed generation")
	// }

	// Update generation if needed

	if g := obj.GetGeneration(); g != obj.StatusGetObservedGeneration() {
		r.log.V(1).Info("updating observed generation", "generation", g)
		obj.StatusSetObservedGeneration(g)
	}

	// if err := r.client.Status().Update(ctx, obj); err != nil {
	// 	return reconcile.Result{}, err
	// }
	// r.log.V(1).Info("updated observed generation")

	r.log.V(1).Info("reconciling deployment", "object", obj)
	d, err := r.reconcileDeployment(ctx, obj)
	if err != nil {
		r.log.V(1).Info("DEBUG DELETEME error deployment", "err", err)
		return reconcile.Result{}, err
	}

	r.log.V(1).Info("updating deployment status", "object", obj)
	r.updateDeploymentStatus(obj, d)

	// if r.crd.GetStatusFlag().AllowConditions() {
	// 	reason := "DeploymentUnknown"
	// 	status := metav1.ConditionUnknown
	// 	message := ""
	// 	for _, c := range d.Status.Conditions {
	// 		if c.Type != appsv1.DeploymentAvailable {
	// 			continue
	// 		}
	// 		switch c.Status {
	// 		case corev1.ConditionTrue:
	// 			status = metav1.ConditionTrue
	// 			reason = c.Reason

	// 		case corev1.ConditionFalse:
	// 			status = metav1.ConditionFalse
	// 			reason = c.Reason
	// 			message = c.Message

	// 		}

	// 	}

	// 	ro.SetStatusCondition(ConditionTypeDeploymentReady, status, reason, message)
	// 	if err := r.client.Status().Update(ctx, obj); err != nil {
	// 		return reconcile.Result{}, err
	// 	}
	// 	r.log.V(1).Info("updated conditions")
	// }

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

	// existing := obj.StatusGetCondition(ConditionTypeDeploymentReady)
	// // do not compare the last transition time
	// existing.LastTransitionTime = desired.LastTransitionTime
	// if semantic.Semantic.DeepEqual(existing, desired) {
	// 	return
	// }

	obj.StatusSetCondition(desired)
}

// ro.SetStatusCondition(ConditionTypeDeploymentReady, status, reason, message)
// if err := r.client.Status().Update(ctx, obj); err != nil {
// 	return reconcile.Result{}, err
// }
// r.log.V(1).Info("updated conditions")

// 	existing := &appsv1.Deployment{}
// 	err = r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
// 	switch {
// 	case err == nil:
// 		if semantic.Semantic.DeepEqual(desired, existing) {
// 			return existing, nil
// 		}

// 		r.log.Info("existing deployment does not match the expected", "object", desired)
// 		r.log.V(5).Info("mismatched deployment", "desired", *desired, "existing", *existing)

// 		// resourceVersion must be returned to the API server unmodified for
// 		// optimistic concurrency, as per Kubernetes API conventions
// 		desired.SetResourceVersion(existing.GetResourceVersion())

// 		if err = r.client.Update(ctx, desired); err != nil {
// 			return nil, fmt.Errorf("could not update deployment object: %+w", err)
// 		}

// 	case apierrs.IsNotFound(err):
// 		r.log.Info("creating deployment", "object", desired)
// 		r.log.V(5).Info("desired deployment", "object", *desired)
// 		if err = r.client.Create(ctx, desired); err != nil {
// 			return nil, fmt.Errorf("could not create deployment object: %w", err)
// 		}

// 	default:
// 		return nil, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
// 	}

// 	// TODO update status
// 	return desired, nil
// }

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

	if s != nil {
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

// func (r *reconciler) InjectLogger(l logr.Logger) error {
// 	r.log = l.WithName("dynrecl")
// 	l.V(2).Info("logger injected into dynamic component reconciler")
// 	return nil
// }

// type ReconciledObject interface {
// 	GetObject() client.Object
// 	SetStatusObservedGeneration(generation int64)
// 	SetStatusCondition(typ string, status metav1.ConditionStatus, reason, message string)
// }

// type reconciledObject struct {
// 	unstructured *unstructured.Unstructured
// 	sm           rcrd.StatusManager
// }

// func (ro *reconciledObject) SetStatusObservedGeneration(generation int64) {
// 	ro.sm.SetObservedGeneration(generation)
// }

// func (ro *reconciledObject) SetStatusCondition(typ string, status metav1.ConditionStatus, reason, message string) {
// 	ro.sm.SetCondition(typ, status, reason, message)
// }

// func (ro *reconciledObject) GetObject() client.Object {
// 	return ro.unstructured
// }

// func (ro *reconciledObject) StatusEqual(reconciledObject ReconciledObject) bool {
// 	uIn := reconciledObject.GetObject().(*unstructured.Unstructured)
// 	// uIn := objIn.(*unstructured.Unstructured)
// 	stIn, okIn := uIn.Object["status"]
// 	st, ok := ro.unstructured.Object["status"]

// 	if okIn != ok {
// 		return false
// 	}

// 	return !semantic.Semantic.DeepEqual(&stIn, &st)
// }

// // func (ro *reconciledObject) DeepCopy() ReconciledObject {
// // 	u := ro.unstructured.DeepCopy()

// // 	return &reconciledObject{

// // 	}
// // }

// func (r *reconciler) NewReconciled() ReconciledObject {
// 	u := &unstructured.Unstructured{}
// 	u.SetGroupVersionKind(r.crd.GetGVK())
// 	ro := &reconciledObject{
// 		unstructured: u,
// 		sm:           r.smf.ForObject(u),
// 	}

// 	return ro
// }
