// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	deploymetr "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/deployment"
	knservingr "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/knservice"
)

func NewReconciler(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, reg common.Registration, mgr manager.Manager) (reconcile.Reconciler, error) {
	switch {
	case reg.GetWorkload().FormFactor.KnativeService != nil:
		return knservingr.NewComponentReconciler(ctx, crd, reg, mgr)
	default:
		// Defaults to deployment
		return deploymetr.NewComponentReconciler(ctx, crd, reg, mgr)
	}

}
