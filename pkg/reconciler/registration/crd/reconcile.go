// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package crd

import (
	"context"

	"github.com/go-logr/logr"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/v1alpha1"
	"github.com/triggermesh/scoby/pkg/component"
	"github.com/triggermesh/scoby/pkg/reconciler/registration/base"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func SetupReconciler(m manager.Manager, br *base.Reconciler) error {

	// skip, this should only be done once.
	// if err := v1alpha1.AddToScheme(m.GetScheme()); err != nil {
	// 	return fmt.Errorf("could not add registration API to scheme: %w", err)
	// }

	// if err := apiextensionsv1.AddToScheme(m.GetScheme()); err != nil {
	// 	return fmt.Errorf("could not add apiextensions API to scheme: %w", err)
	// }

	// r := &reconciler{}
	// if err := builder.ControllerManagedBy(m).
	// 	For(&v1alpha1.CRDRegistration{}).
	// 	Complete(r); err != nil {
	// 	return fmt.Errorf("could not build controller for CRD registration: %w", err)
	// }

	return nil
}

// CRD registration reconciler is a simple ControllerManagedBy example implementation.
type Reconciler struct {
	log logr.Logger
	client.Client

	Registry component.ControllerRegistry
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.V(1).Info("Reconciling CRD registration", "request", req)

	cr := &v1alpha1.CRDRegistration{}
	err := r.Get(ctx, req.NamespacedName, cr)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.log.V(5).Info("CRD registration retrieved", "object", cr)

	// lookup the information for the CRD registration.

	// compare data

	// cr.Spec.CRD
	// _, err = r.br.ReconcileCRD(ctx, reg)
	// if err != nil {
	// 	return reconcile.Result{}, err
	// }

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
