// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
	"github.com/triggermesh/scoby/pkg/utils/semantic"
)

const (
	componentFinalizer = "scoby.triggermesh.io/finalizer"
)

func NewController(
	om reconciler.ObjectManager,
	reg commonv1alpha1.Registration,
	ffr reconciler.FormFactorReconciler,
	hr reconciler.HookReconciler,
	mgr ctrl.Manager,
	log logr.Logger) (controller.Controller, error) {

	r := &base{
		objectManager:        om,
		formFactorReconciler: ffr,
		hookReconciler:       hr,
		client:               mgr.GetClient(),
		log:                  log,
	}

	c, err := controller.NewUnmanaged(reg.GetName(), mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", reg.GetName(), err)
	}

	obj := om.NewObject()
	if err := c.Watch(source.Kind(mgr.GetCache(), obj.AsKubeObject()), &handler.EnqueueRequestForObject{}); err != nil {
		return nil, fmt.Errorf("could not set watcher on registered object %q: %w", reg.GetName(), err)
	}

	if err := ffr.SetupController(reg.GetName(), c, om.NewObject()); err != nil {
		return nil, fmt.Errorf("could not setup form factor controller for %q: %w", reg.GetName(), err)
	}

	return c, nil
}

var _ reconciler.Base = (*base)(nil)

type base struct {
	objectManager        reconciler.ObjectManager
	formFactorReconciler reconciler.FormFactorReconciler
	hookReconciler       reconciler.HookReconciler
	client               client.Client
	log                  logr.Logger
}

func (b *base) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	b.log.V(1).Info("Reconciling request", "request", req)

	// If the object does not exist, skip reconciliation.
	obj := b.objectManager.NewObject()
	if err := b.client.Get(ctx, req.NamespacedName, obj.AsKubeObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	b.log.V(5).Info("Object retrieved", "object", obj)

	// create a copy, we will compare after reconciling
	// and decide if we need to update or not.
	cp := obj.AsKubeObject().DeepCopyObject()

	// Initialize status according to the form factor if needed.
	obj.GetStatusManager().SanitizeConditions()

	var res ctrl.Result
	var err error

	if obj.GetDeletionTimestamp().IsZero() {
		res, err = b.manageReconciliation(ctx, obj)

	} else {
		res, err = b.manageDeletion(ctx, obj)
	}

	// If there are changes to status, update it.
	// Update status if needed.
	if !semantic.Semantic.DeepEqual(
		obj.AsKubeObject().(*unstructured.Unstructured).Object["status"],
		cp.(*unstructured.Unstructured).Object["status"]) {
		if uperr := b.client.Status().Update(ctx, obj.AsKubeObject()); uperr != nil {
			if err == nil {
				return ctrl.Result{}, uperr
			}
			b.log.Error(uperr, "could not update the object status")
		}
	}

	return res, err
}

func (b *base) manageDeletion(ctx context.Context, obj reconciler.Object) (ctrl.Result, error) {
	if b.hookReconciler == nil ||
		!b.hookReconciler.IsFinalizer() {
		return ctrl.Result{}, nil
	}

	// When hooks are configured we need to call Finalize on the hook and
	// then remove the finalizer attribute at the object.

	if err := b.hookReconciler.Finalize(ctx, obj); err != nil {
		if !err.IsContinue() {
			return ctrl.Result{Requeue: !err.IsPermanent()}, err
		}
	}

	if !controllerutil.ContainsFinalizer(obj, componentFinalizer) {
		return ctrl.Result{}, nil
	}

	controllerutil.RemoveFinalizer(obj, componentFinalizer)

	return ctrl.Result{}, b.client.Update(ctx, obj.AsKubeObject())
}

func (b *base) manageReconciliation(ctx context.Context, obj reconciler.Object) (ctrl.Result, error) {
	// Render using the object data and configuration
	if err := b.objectManager.GetRenderer().Render(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	candidates, err := b.formFactorReconciler.PreRender(ctx, obj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("pre-rendering form factor children candidates: %w", err)
	}

	if b.hookReconciler != nil {
		if b.hookReconciler.IsPreReconciler() {
			if err := b.hookReconciler.PreReconcile(ctx, obj, &candidates); err != nil {
				if !err.IsContinue() {
					return ctrl.Result{Requeue: !err.IsPermanent()}, fmt.Errorf("reconciling hook: %w", err)
				}
			}
		}
		if b.hookReconciler.IsFinalizer() {
			// Set the finalizer if it is not present
			objk := obj.AsKubeObject()
			if !controllerutil.ContainsFinalizer(objk, componentFinalizer) {
				controllerutil.AddFinalizer(objk, componentFinalizer)
				if err := b.client.Update(ctx, objk); err != nil {
					return ctrl.Result{}, err
				}

			}
		}
	}

	// Update generation if needed
	if g := obj.GetGeneration(); g != obj.GetStatusManager().GetObservedGeneration() {
		b.log.V(1).Info("updating observed generation", "generation", g)
		obj.GetStatusManager().SetObservedGeneration(g)
	}

	// Pass the children candidates to the form factor for the reconcile routine.
	return b.formFactorReconciler.Reconcile(ctx, obj, candidates)
}
