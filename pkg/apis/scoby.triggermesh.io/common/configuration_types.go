// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

import corev1 "k8s.io/api/core/v1"

// Configuration contains parameter configuration for registered components.
type Configuration struct {
	// FromImage contains the container image information.
	Parameters []Parameter `json:"parameters,omitempty"`
}

// Parameter defines key/values to be passed to components.
type Parameter struct {
	// Name for the parameter.
	Name string `json:"name"`
	// Type for the parameter, must be a valid CRD type.
	Type *string `json:"type"`
	// Required flag for the parameter, must be a valid CRD type.
	Required *bool `json:"required"`
	// Section will set rendered parameter in a nested section element.
	Section *string `json:"section"`
	// ValueFrom indicates the reference source for the parameter.
	ValueFrom *ParameterSource `json:"valueFrom"`
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
