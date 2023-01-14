// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

var (
	defaultVersion = common.GenerateVersion{
		Version: "v1alpha1",
		Served:  true,
		Storage: true,
	}
)

// type Reconciler interface {
// 	ReconcileCRD(ctx context.Context, registration common.Registration) (*apiextensionsv1.CustomResourceDefinition, error)
// }

type ReconcilerOpt func(*Reconciler)

func New(client client.Client, logger logr.Logger, opts ...ReconcilerOpt) *Reconciler {
	r := &Reconciler{
		apiGroup: "",

		client: client,
		log:    logger,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// reconciler implements controller.Reconciler for the registration.
type Reconciler struct {
	// crdReconciler         *syncs.CRDReconciler
	// clusterRoleReconciler *syncs.ClusterRoleReconciler

	// registry component.ControllerRegistry
	apiGroup string

	log    logr.Logger
	client client.Client
}

func ReconcilerWithAPIGroup(apiGroup string) ReconcilerOpt {
	return func(r *Reconciler) {
		r.apiGroup = apiGroup
	}
}

// ReconcileCRD for the registration.
func (r *Reconciler) ReconcileCRD(ctx context.Context, registration common.Registration) (*apiextensionsv1.CustomResourceDefinition, error) {
	r.log.V(1).Info("Reconciling CRD for registration", "registration", registration.GetObjectKind().GroupVersionKind())

	// Build CRD object
	desired, err := createCRDFromRegistration(registration)
	if err != nil {
		return nil, fmt.Errorf("could not create desired CRD: %w", err)
	}

	r.log.V(5).Info("Desired CRD for registration", "object", desired.String())

	existing := &apiextensionsv1.CustomResourceDefinition{}

	err = r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		// Compare
		// If same, that is ok
		// If not same, versioning is not supported, fail.

	case apierrs.IsNotFound(err):
		r.log.Info("Creating CRD", "object", desired)
		if err = r.client.Create(ctx, desired); err != nil {
			// TODO Propagate error to status
			return nil, fmt.Errorf("could not create CRD: %w", err)
		}

	default:
		return nil, fmt.Errorf("could not retrieving CRD %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	// crd := resources.BuildCRDFromRegistration(registration)
	// status := registration.GetStatusManager()

	// curcrd, event := r.crdReconciler.Reconcile(ctx, crd)
	// if curcrd == nil && event != nil {
	// 	status.MarkNoGeneratedCRDError(event)
	// 	// send error instead of event to log the error message.
	// 	return nil, fmt.Errorf(event.Error())
	// }

	// if ok := status.PropagateCRDAvailability(curcrd); !ok {
	// 	return nil, event
	// }

	// return curcrd, nil

	return desired, err
}

func createCRDFromRegistration(registration common.Registration) (*apiextensionsv1.CustomResourceDefinition, error) {
	spec, err := createCRDVersionSpecFromRegistration(registration)
	if err != nil {
		return nil, err
	}

	return resources.NewCRD(registration.GetName(),
		resources.CRDWithNames(registration.GetCRDNames()),
		resources.CRDAddVersion(spec),
	)
}

func createCRDVersionSpecFromRegistration(registration common.Registration) (*apiextensionsv1.CustomResourceDefinitionVersion, error) {
	v := registration.GetGenerateVersion()
	if v == nil {
		v = &defaultVersion
	}

	return resources.NewCRDVersion(v.Version, v.Served, v.Storage /* TODO build props */)
}
