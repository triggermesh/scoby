// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	deploymetr "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/deployment"
	knservingr "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/knservice"
)

func NewReconciler(ctx context.Context, gvk schema.GroupVersionKind, reg common.Registration, mgr manager.Manager) (reconcile.Reconciler, error) {
	switch {
	case reg.GetWorkload().FormFactor.KnativeService != nil:
		return knservingr.NewComponentReconciler(ctx, gvk, reg, mgr)
	default:
		// Defaults to deployment
		return deploymetr.NewComponentReconciler(ctx, gvk, reg, mgr)
	}

}
