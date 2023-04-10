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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
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
	if err := c.Watch(&source.Kind{Type: obj.AsKubeObject()}, &handler.EnqueueRequestForObject{}); err != nil {
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

	obj := b.objectManager.NewObject()
	if err := b.client.Get(ctx, req.NamespacedName, obj.AsKubeObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	b.log.V(5).Info("Object retrieved", "object", obj)

	if !obj.GetDeletionTimestamp().IsZero() {
		if b.hookReconciler != nil {
			_ = b.hookReconciler.Finalize(ctx, obj)
		}

		// Return and let the ownership clean resources.
		return ctrl.Result{}, nil
	}

	// create a copy, we will compare after reconciling
	// and decide if we need to update or not.
	cp := obj.AsKubeObject().DeepCopyObject()

	if b.hookReconciler != nil {
		err := b.hookReconciler.Reconcile(ctx, obj)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling hook: %w", err)
		}
	}

	// Render using the object data and configuration
	if err := b.objectManager.GetRenderer().Render(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	res, err := b.formFactorReconciler.Reconcile(ctx, obj)

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
