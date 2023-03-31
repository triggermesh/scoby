// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base"
	basecrd "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/crd"
	baseobject "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/object"
	baserenderer "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/renderer"
	baseresolver "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/resolver"
	basestatus "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/status"
	deployment "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/deployment"
	knservice "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/knservice"
)

func NewReconciler(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, reg commonv1alpha1.Registration, mgr manager.Manager) (chan error, error) {
	log := mgr.GetLogger()
	client := mgr.GetClient()

	crdv := basecrd.CRDPrioritizedVersion(crd)
	if crdv == nil {
		return nil, fmt.Errorf("no available CRD version for %s at %s", crd.GetName(), reg.GetName())
	}

	gvk := &schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crdv.Name,
		Kind:    crd.Spec.Names.Kind,
	}

	wkl := reg.GetWorkload()

	renderer := baserenderer.NewRenderer(wkl, baseresolver.New(client))

	var ffr reconciler.FormFactorReconciler
	switch {
	case wkl.FormFactor.KnativeService != nil:
		ffr = knservice.New(reg.GetName(), wkl, client, log)

	default:
		// Defaults to deployment
		ffr = deployment.New(reg.GetName(), wkl, client, log)
	}

	// The status factory is created using the form factor's conditions
	happy, all := ffr.GetStatusConditions()

	// Add conditions informed from a hook
	if wkl.StatusConfiguration != nil {
		for _, c := range wkl.StatusConfiguration.ConditionsFromHook {
			all = append(all, c.Type)
		}
	}

	smf := basestatus.NewStatusManagerFactory(crdv, happy, all, log)

	om := baseobject.NewManager(gvk, renderer, smf)

	c, err := base.NewController(om, reg, ffr, mgr, log)
	if err != nil {
		return nil, err
	}

	stCh := make(chan error)
	go func() {
		stCh <- c.Start(ctx)
	}()

	return stCh, nil
}
