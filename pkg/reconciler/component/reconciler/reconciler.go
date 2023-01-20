// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render/deployment"
)

type ComponentReconciler interface {
	reconcile.Reconciler
	NewObject() client.Object
}

func NewComponentReconciler(ctx context.Context, gvk schema.GroupVersionKind, reg common.Registration, mgr manager.Manager) (ComponentReconciler, error) {
	log := mgr.GetLogger().WithName(gvk.GroupKind().String())

	r := &reconciler{
		log:          log,
		gvk:          gvk,
		registration: reg,
	}

	w := reg.GetWorkload()

	switch {
	case w.FormFactor.KnativeService != nil:
		// TODO
		r.renderer = nil
	default:
		// Default to deployment. The renderer will be able to
		// deal with an empty deployment form factor.
		r.renderer = deployment.New(reg, log)
	}

	if err := builder.ControllerManagedBy(mgr).
		For(r.NewObject()).
		Complete(r); err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", gvk, err)
	}
	return r, nil
}

type reconciler struct {
	gvk          schema.GroupVersionKind
	registration common.Registration
	renderer     render.Renderer

	client client.Client
	log    logr.Logger
}

var _ ComponentReconciler = (*reconciler)(nil)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.Info("Reconciling request", "request", req)

	obj := r.NewObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		return reconcile.Result{}, err
	}

	r.log.Info("Object read", "obj", obj)

	objs, err := r.renderer.RenderControlledObjects(obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, desired := range objs {
		r.log.V(1).Info("rendered desired object", "object", desired)

		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(desired.GetObjectKind().GroupVersionKind())
		err := r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)

		switch {
		case err == nil:
			// Compare
			// If same, that is ok
			// If not same, versioning is not supported, fail.

		case apierrs.IsNotFound(err):
			r.log.Info("Creating CRD", "object", desired)
			if err = r.client.Create(ctx, desired); err != nil {
				// TODO Propagate error to status
				return reconcile.Result{}, fmt.Errorf("could not create controlled object: %w", err)
			}

		default:
			return reconcile.Result{}, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
		}

		// update status

	}

	return reconcile.Result{}, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectLogger(l logr.Logger) error {
	r.log = l.WithName("dynrecl")
	l.V(5).Info("logger injected into dynamic component reconciler")
	return nil
}

func (r *reconciler) NewObject() client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   r.gvk.Group,
		Kind:    r.gvk.Kind,
		Version: r.gvk.Version,
	})
	return obj
}
