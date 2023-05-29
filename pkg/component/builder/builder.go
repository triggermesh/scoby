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
	"github.com/triggermesh/scoby/pkg/component/reconciler"
	"github.com/triggermesh/scoby/pkg/component/reconciler/base"
	basecrd "github.com/triggermesh/scoby/pkg/component/reconciler/base/crd"
	baseobject "github.com/triggermesh/scoby/pkg/component/reconciler/base/object"
	baserenderer "github.com/triggermesh/scoby/pkg/component/reconciler/base/renderer"
	basestatus "github.com/triggermesh/scoby/pkg/component/reconciler/base/status"
	"github.com/triggermesh/scoby/pkg/component/reconciler/formfactor/deployment"
	"github.com/triggermesh/scoby/pkg/component/reconciler/formfactor/knservice"
	"github.com/triggermesh/scoby/pkg/component/reconciler/hook"
	"github.com/triggermesh/scoby/pkg/utils/configmap"
	"github.com/triggermesh/scoby/pkg/utils/resolver"
)

type Builder interface {
	StartNewReconciler(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, reg commonv1alpha1.Registration) (chan error, error)
}

type builder struct {
	mgr   manager.Manager
	reslv resolver.Resolver
	cmr   configmap.Reader
}

func (b *builder) StartNewReconciler(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, reg commonv1alpha1.Registration) (chan error, error) {
	log := b.mgr.GetLogger()

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

	var ffr reconciler.FormFactorReconciler
	switch {
	case wkl.FormFactor.KnativeService != nil:
		ffr = knservice.New(reg.GetName(), wkl, b.mgr)

	default:
		// Defaults to deployment
		ffr = deployment.New(reg.GetName(), wkl, b.mgr)
	}

	// The status factory is created using the form factor's conditions
	happy, all := ffr.GetStatusConditions()

	var hr reconciler.HookReconciler
	if h := reg.GetHook(); h != nil {
		url := reg.GetStatusAnnotation(commonv1alpha1.CRDRegistrationAnnotationHookURL)
		if url == nil {
			return nil, fmt.Errorf("%s registration does not contain the %q status annotation",
				reg.GetName(), commonv1alpha1.CRDRegistrationAnnotationHookURL)
		}

		// Add conditions informed from a hook
		var cfh []commonv1alpha1.ConditionsFromHook
		if wkl.StatusConfiguration != nil {
			for _, c := range wkl.StatusConfiguration.ConditionsFromHook {
				all = append(all, c.Type)
			}
			cfh = wkl.StatusConfiguration.ConditionsFromHook
		}

		log.Info("Configuring hook", "url", *url)
		hr = hook.New(h, *url, cfh, log)
	}

	renderer, err := baserenderer.NewRenderer(wkl, b.reslv, b.cmr)
	if err != nil {
		return nil, fmt.Errorf("could not create renderer for %s at %s: %w", crd.GetName(), reg.GetName(), err)
	}

	smf := basestatus.NewStatusManagerFactory(crdv, happy, all, log)

	om := baseobject.NewManager(gvk, renderer, smf)

	c, err := base.NewController(om, reg, ffr, hr, b.mgr, log)
	if err != nil {
		return nil, fmt.Errorf("could not create controller for %s at %s: %w", crd.GetName(), reg.GetName(), err)
	}

	stCh := make(chan error)
	go func() {
		stCh <- c.Start(ctx)
	}()

	return stCh, nil
}

func NewBuilder(mgr manager.Manager, reslv resolver.Resolver, cmr configmap.Reader) Builder {
	return &builder{
		mgr:   mgr,
		reslv: reslv,
		cmr:   cmr,
	}
}
