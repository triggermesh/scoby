// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package crd

import (
	"context"

	"github.com/go-logr/logr"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/v1alpha1"
	"github.com/triggermesh/scoby/pkg/reconciler/component/registry"
)

// CRD registration reconciler is a simple ControllerManagedBy example implementation.
type Reconciler struct {
	log logr.Logger
	client.Client

	Registry registry.ComponentRegistry
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.V(1).Info("reconciling CRD registration", "request", req)

	// TODO deletion case

	cr := &v1alpha1.CRDRegistration{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrs.IsNotFound(err) {
			// TODO for the deletion case, this is all good
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	r.log.V(5).Info("CRD registration retrieved", "object", cr)

	// lookup the information for the CRD registration.
	key := types.NamespacedName{Name: cr.Spec.CRD}
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.Client.Get(ctx, key, crd, &client.GetOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			r.log.V(5).Info("CRD not found", "object", cr, "crd", cr.Spec.CRD)
			// TODO for the deletion case, this is all good
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if err := r.Registry.EnsureComponentController(crd, cr); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}

func (r *Reconciler) InjectLogger(l logr.Logger) error {
	r.log = l.WithName("crdregistration")
	l.V(5).Info("logger injected into CRD reconciler")
	return nil
}
