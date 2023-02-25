// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/status"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

type Reconciling interface {
	client.Object

	AsKubeObject() client.Object

	StatusGetObservedGeneration() int64
	StatusSetObservedGeneration(generation int64)
	StatusGetCondition(conditionType string) *apicommon.Condition
	StatusSetCondition(condition *apicommon.Condition)
	StatusIsEqual(client.Object) bool
}

type ReconcilingObjectFactory interface {
	NewReconcilingObject() Reconciling
}

func NewReconcilingObjectFactory(gvk schema.GroupVersionKind, smf status.StatusManagerFactory, renderer Renderer) ReconcilingObjectFactory {
	return &reconciledObjectFactory{
		gvk:      gvk,
		smf:      smf,
		renderer: renderer,
	}
}

type reconciledObjectFactory struct {
	gvk      schema.GroupVersionKind
	smf      status.StatusManagerFactory
	renderer Renderer
}

func (rof *reconciledObjectFactory) NewReconcilingObject() Reconciling {
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
	sm       status.StatusManager
	renderer Renderer
}

func (ro *reconciledObject) AsKubeObject() client.Object {
	return ro.Unstructured
}

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
