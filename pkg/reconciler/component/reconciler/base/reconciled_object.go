// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/client"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

type ReconciledObject interface {
	client.Object

	// Render() (RenderedObject, error)
	// RenderPodSpecOptions() ([]resources.PodSpecOption, error)
	AsKubeObject() client.Object

	StatusGetObservedGeneration() int64
	StatusSetObservedGeneration(generation int64)
	StatusGetCondition(conditionType string) *apicommon.Condition
	StatusSetCondition(condition *apicommon.Condition)
	StatusIsEqual(client.Object) bool
}

type ReconciledObjectFactory interface {
	NewReconciledObject() ReconciledObject
}

func NewReconciledObjectFactory(gvk schema.GroupVersionKind, smf StatusManagerFactory, renderer Renderer) ReconciledObjectFactory {
	return &reconciledObjectFactory{
		gvk:      gvk,
		smf:      smf,
		renderer: renderer,
	}
}

type reconciledObjectFactory struct {
	gvk      schema.GroupVersionKind
	smf      StatusManagerFactory
	renderer Renderer
}

func (rof *reconciledObjectFactory) NewReconciledObject() ReconciledObject {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(rof.gvk)
	ro := &reconciledObject{
		Unstructured: u,

		sm:       rof.smf.ForObject(u),
		renderer: rof.renderer,
	}

	return ro
}

type reconciledObject struct {
	*unstructured.Unstructured
	sm       StatusManager
	renderer Renderer
}

func (ro *reconciledObject) AsKubeObject() client.Object {
	return ro.Unstructured
}

// func (ro *reconciledObject) RenderPodSpecOptions() ([]resources.PodSpecOption, error) {
// 	return ro.renderer.Render(ro)
// }

// func (ro *reconciledObject) Render() (RenderedObject, error) {
// 	return ro.renderer.Render(ro)
// }

func (ro *reconciledObject) StatusGetObservedGeneration() int64 {
	return ro.sm.GetObservedGeneration()
}

func (ro *reconciledObject) StatusSetObservedGeneration(generation int64) {
	ro.sm.SetObservedGeneration(generation)
}

func (ro *reconciledObject) StatusSetCondition(condition *apicommon.Condition) {
	ro.sm.SetCondition(condition)
}

func (ro *reconciledObject) StatusGetCondition(conditionType string) *apicommon.Condition {
	return ro.sm.GetCondition(conditionType)
}
func (ro *reconciledObject) StatusIsEqual(in client.Object) bool {
	u, ok := in.(*unstructured.Unstructured)
	if !ok {
		return false
	}

	if !semantic.Semantic.DeepEqual(u.Object["status"], ro.Unstructured.Object["status"]) {
		return false
	}

	return true
}
