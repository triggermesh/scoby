// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package render

import "sigs.k8s.io/controller-runtime/pkg/client"

type Renderer interface {
	RenderControlledObjects(obj client.Object) ([]client.Object, error)
	// TODO add status management. Given a status element for the object and the set of
	// controlled elements, it fills the status accordingly
}
