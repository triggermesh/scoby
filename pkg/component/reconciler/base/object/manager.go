// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
)

func NewManager(gvk *schema.GroupVersionKind, renderer reconciler.ObjectRenderer, smf reconciler.StatusManagerFactory) reconciler.ObjectManager {
	return &manager{
		gvk:                  gvk,
		renderer:             renderer,
		statusManagerFactory: smf,
	}
}

type manager struct {
	gvk                  *schema.GroupVersionKind
	renderer             reconciler.ObjectRenderer
	statusManagerFactory reconciler.StatusManagerFactory
}

func (m manager) NewObject() reconciler.Object {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(*m.gvk)

	return &object{
		Unstructured:  u,
		statusManager: m.statusManagerFactory.ForObject(u),

		evsByPath: make(map[string]*corev1.EnvVar),
		evsByName: make(map[string]*corev1.EnvVar),

		vmByPath: make(map[string]*commonv1alpha1.FromSpecToVolume),
		vmByName: make(map[string]*commonv1alpha1.FromSpecToVolume),
	}
}

func (m manager) GetRenderer() reconciler.ObjectRenderer {
	return m.renderer
}
