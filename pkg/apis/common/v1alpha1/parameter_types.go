// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import corev1 "k8s.io/api/core/v1"

// ParameterRender options.
type ParameterRender struct {
	// Key literal for the workload parameter.
	// +optional
	Key *string `json:"key,omitempty"`
}

// Parameter defines key/values to be passed to components.
type Parameter struct {
	// JSONPath for the parameter.
	Path string `json:"path"`

	// Skip sets whether the object should skip rendering
	// as a workload parameter.
	// +optional
	Skip *bool `json:"skip,omitempty"`

	// Render options for the parameter.
	// +optional
	Render *ParameterRender `json:"render,omitempty"`
}

type ReferenceType string

const (
	ReferenceTypeSecret    ReferenceType = "secret"
	ReferenceTypeConfigMap ReferenceType = "configmap"
	ReferenceTypeDownward  ReferenceType = "downward"
)

// ParameterSource represents the source for the value of a parameter.
type ParameterSource struct {
	// ReferenceType for the parameter.
	ReferenceType ReferenceType               `json:"referenceType"`
	FieldRef      *corev1.ObjectFieldSelector `json:"fieldRef"`
}
