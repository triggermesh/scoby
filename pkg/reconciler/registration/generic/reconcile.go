// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package generic

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/v1alpha1"
	"github.com/triggermesh/scoby/pkg/reconciler/registration/base"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func SetupReconciler(m manager.Manager, br *base.Reconciler) error {

	// if err := v1alpha1.AddToScheme(m.GetScheme()); err != nil {
	// 	return fmt.Errorf("could not add registration API to scheme: %w", err)
	// }

	// if err := apiextensionsv1.AddToScheme(m.GetScheme()); err != nil {
	// 	return fmt.Errorf("could not add apiextensions API to scheme: %w", err)
	// }

	r := &reconciler{
		br: br,
	}
	if err := builder.ControllerManagedBy(m).
		For(&v1alpha1.GenericRegistration{}).
		Owns(&apiextensionsv1.CustomResourceDefinition{}).
		Complete(r); err != nil {
		return fmt.Errorf("could not build controller for generic registration: %w", err)
	}

	return nil
}

// GenericRegistrationReconciler   is a simple ControllerManagedBy example implementation.
type reconciler struct {
	br *base.Reconciler

	log logr.Logger
	client.Client
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.V(1).Info("Reconciling generic registration", "request", req)

	reg := &v1alpha1.GenericRegistration{}
	err := r.Get(ctx, req.NamespacedName, reg)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.log.V(5).Info("Generic registration retrieved", "object", reg)

	_, err = r.br.ReconcileCRD(ctx, reg)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}

func (r *reconciler) InjectLogger(l logr.Logger) error {
	r.log = l.WithName("genericregistration")
	l.V(5).Info("logger injected into GenericRegistrationReconciler")
	return nil
}
