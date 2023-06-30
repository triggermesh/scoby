// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package crd

import (
	"context"
	"time"

	"github.com/go-logr/logr"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	scobyv1alpha1 "github.com/triggermesh/scoby/pkg/apis/scoby/v1alpha1"
	"github.com/triggermesh/scoby/pkg/registration/registry"
	"github.com/triggermesh/scoby/pkg/utils/resolver"
	"github.com/triggermesh/scoby/pkg/utils/semantic"
)

const (
	crdFinalizer = "scoby.triggermesh.io/finalizer"
)

//+kubebuilder:rbac:groups=scoby.triggermesh.io,resources=crdregistrations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=scoby.triggermesh.io,resources=crdregistrations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=scoby.triggermesh.io,resources=crdregistrations/finalizers,verbs=update

// CRD registration reconciler is a simple ControllerManagedBy example implementation.
type Reconciler struct {
	registry registry.ComponentRegistry
	resolver resolver.Resolver

	log    logr.Logger
	client client.Client
}

func New(client client.Client, registry registry.ComponentRegistry, resolver resolver.Resolver, log logr.Logger) *Reconciler {
	return &Reconciler{
		log:      log,
		client:   client,
		registry: registry,
		resolver: resolver,
	}
}

func (r *Reconciler) On(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	r.log.V(1).Info("reconciling CRD registration", "request", req)

	existing := &scobyv1alpha1.CRDRegistration{}
	if err := r.client.Get(ctx, req.NamespacedName, existing); err != nil {
		// Return error (unless resource was deleted).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.log.V(5).Info("CRD registration retrieved", "object", *existing)

	if !existing.DeletionTimestamp.IsZero() {
		return r.reconcileDeletion(ctx, existing)
	}

	// create a copy, we will compare after reconciling and decide if we need to
	// update or not.
	cr := existing.DeepCopy()

	res, err := r.reconcileRegistration(ctx, cr)

	// Update status if needed.
	//
	// We need to compare the internal status, which is covered by the semantic
	// comparer library
	if !semantic.Semantic.DeepEqual(&cr.Status.Status, &existing.Status.Status) {
		// The err variable is newly defined, if the update is unsuccessful
		// the error returned will be the update operation error.
		if err := r.client.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	return res, err
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (ctrl.Result, error) {
	r.log.V(1).Info("reconciling CRD registration", "request", req)

	existing := &scobyv1alpha1.CRDRegistration{}
	if err := r.client.Get(ctx, req.NamespacedName, existing); err != nil {
		// Return error (unless resource was deleted).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.log.V(5).Info("CRD registration retrieved", "object", *existing)

	if !existing.DeletionTimestamp.IsZero() {
		return r.reconcileDeletion(ctx, existing)
	}

	// create a copy, we will compare after reconciling and decide if we need to
	// update or not.
	cr := existing.DeepCopy()

	res, err := r.reconcileRegistration(ctx, cr)

	// Update status if needed.
	//
	// We need to compare the internal status, which is covered by the semantic
	// comparer library
	if !semantic.Semantic.DeepEqual(&cr.Status.Status, &existing.Status.Status) {
		// The err variable is newly defined, if the update is unsuccessful
		// the error returned will be the update operation error.
		if err := r.client.Status().Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	return res, err
}

func (r *Reconciler) reconcileDeletion(ctx context.Context, cr *scobyv1alpha1.CRDRegistration) (ctrl.Result, error) {
	// clean resources
	r.registry.RemoveComponentController(cr)

	if !controllerutil.ContainsFinalizer(cr, crdFinalizer) {
		return ctrl.Result{}, nil
	}

	// Removing the finalizer must succeed so that
	// the registration is deleted.
	controllerutil.RemoveFinalizer(cr, crdFinalizer)
	return ctrl.Result{}, r.client.Update(ctx, cr)
}

func (r *Reconciler) reconcileRegistration(ctx context.Context, cr *scobyv1alpha1.CRDRegistration) (ctrl.Result, error) {
	// Set the finalizer if it is not present
	if !controllerutil.ContainsFinalizer(cr, crdFinalizer) {
		controllerutil.AddFinalizer(cr, crdFinalizer)
		if err := r.client.Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}

		// Let the update trigger the next reconciliation.
		return ctrl.Result{}, nil
	}

	// Retrieve the status manager (also initializes it)
	sm := cr.GetStatusManager()
	sm.SetObservedGeneration(cr.Generation)

	// Lookup the CRD for the registration.
	key := types.NamespacedName{Name: cr.Spec.CRD}
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.client.Get(ctx, key, crd, &client.GetOptions{}); err != nil {
		sm.MarkConditionFalse(scobyv1alpha1.CRDRegistrationConditionCRDExists, "CRDERROR", err.Error())
		// TODO replace requeueAfter with a watch
		// TODO if the component controller is running, stop it.
		return ctrl.Result{RequeueAfter: time.Second * 15}, err
	}
	sm.MarkConditionTrue(scobyv1alpha1.CRDRegistrationConditionCRDExists, "CRDEXIST")

	// If hook configured, parse reference
	if cr.Spec.Hook != nil {
		u, err := r.resolver.ResolveDestination(ctx, &cr.Spec.Hook.Address)
		if err != nil {
			sm.MarkConditionFalse(scobyv1alpha1.CRDRegistrationConditionControllerReady,
				"HOOKFAILED", err.Error())
			return ctrl.Result{}, err
		}

		// u should never be nil when the resolver succeeds, but let's make sure.
		if u == nil {

			sm.MarkConditionFalse(scobyv1alpha1.CRDRegistrationConditionControllerReady,
				"HOOKFAILED", "Hook address resolution returned no URL")
			return ctrl.Result{}, err
		}

		// use url for annotation
		sm.SetAnnotation(commonv1alpha1.CRDRegistrationAnnotationHookURL, *u)
	}

	// Make sure the CRD controller is running
	err := r.registry.EnsureComponentController(cr, crd)
	if err != nil {
		sm.MarkConditionFalse(scobyv1alpha1.CRDRegistrationConditionControllerReady,
			"CONTROLLERFAILED", err.Error())
	}

	sm.MarkConditionTrue(scobyv1alpha1.CRDRegistrationConditionControllerReady, "CONTROLLERSTARTED")

	return ctrl.Result{}, err
}
