// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package knservice

import (
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render/podspec"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

const defaultReplicas = 1

type Renderer struct {
	reg common.Registration

	psr *podspec.Renderer
	log logr.Logger
}

func New(reg common.Registration, log logr.Logger) *Renderer {
	return &Renderer{
		reg: reg,
		psr: podspec.New("adapter", reg.GetWorkload().FromImage.Repo),
		log: log,
	}
}

func (r *Renderer) RenderControlledObjects(obj client.Object) ([]client.Object, error) {
	s, err := r.createKnServiceFrom(obj)
	if err != nil {
		return nil, err
	}

	return []client.Object{s}, nil
}

func (r *Renderer) createKnServiceFrom(obj client.Object) (*servingv1.Service, error) {
	// TODO generate names

	// use parameter options to define parameters policy
	// use obj to gather
	ps, _ := r.psr.Render(obj)

	// replicas := defaultReplicas
	// if ffd := r.reg.GetWorkload().FormFactor.KnativeService.; ffd != nil {
	// 	replicas = ffd.Replicas
	// }
	// TODO min, max scale, visibility,

	return resources.NewKnativeService(obj.GetNamespace(), obj.GetName(),
		resources.KnativeServiceWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, r.reg.GetName()),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, render.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, render.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, render.ManagedBy),

			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.KnativeServiceWithRevisionSpecOptions(
			resources.RevisionSpecWithPodSpecOptions(ps...),
		)), nil
}

func (r *Renderer) EnsureRemoved() {

}
