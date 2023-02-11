// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base"
	deployment "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/deployment"
	knservice "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/knservice"
)

func NewReconciler(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, reg common.Registration, mgr manager.Manager) (reconcile.Reconciler, error) {
	psr := base.NewPodSpecRenderer("adapter", reg.GetWorkload().FromImage.Repo)
	b := base.NewReconciler(crd, reg, psr, mgr.GetLogger())

	switch {
	case reg.GetWorkload().FormFactor.KnativeService != nil:
		return knservice.NewComponentReconciler(ctx, b, mgr)

	default:
		// Defaults to deployment
		return deployment.NewComponentReconciler(ctx, b, mgr)
	}
}
