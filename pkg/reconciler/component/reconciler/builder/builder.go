// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
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

const (
	defaultContainerName = "adapter"
)

func NewReconciler(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, reg common.Registration, mgr manager.Manager) (chan error, error) {
	log := mgr.GetLogger()

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

	renderer := baserenderer.NewRenderer(defaultContainerName, wkl, baseresolver.New(mgr.GetClient()))

	// The status factory is created using only the ConditionTypeReady condition, it is up
	// to the base reconciler user to update with their set of conditions before using it.
	smf := basestatus.NewStatusManagerFactory(crdv,
		reconciler.ConditionTypeReady,
		[]string{reconciler.ConditionTypeReady}, log)

	om := baseobject.NewManager(gvk, renderer, smf)

	var ff reconciler.FormFactor
	switch {
	case reg.GetWorkload().FormFactor.KnativeService != nil:
		ff = knservice.New()

	default:
		// Defaults to deployment
		ff = deployment.New()
	}

	c, err := base.NewController(om, reg, ff, mgr, log)
	if err != nil {
		return nil, err
	}

	stCh := make(chan error)
	go func() {
		stCh <- c.Start(ctx)
	}()

	return stCh, nil
}
