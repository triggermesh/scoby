// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package podspec

import (
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

type Renderer struct {
	name  string
	image string
}

func New(name, image string) *Renderer {
	return &Renderer{
		name:  name,
		image: image,
	}
}

func (r *Renderer) Render(obj client.Object) ([]resources.PodSpecOption, error) {
	return []resources.PodSpecOption{resources.PodSpecAddContainer(
		resources.NewContainer(r.name, r.image,
			resources.ContainerWithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError),
		),
	)}, nil
}
