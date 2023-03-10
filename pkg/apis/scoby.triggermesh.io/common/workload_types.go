// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	corev1 "k8s.io/api/core/v1"
)

// Workload contains workload settings.
type Workload struct {
	// FormFactor indicates the kubernetes object that
	// will run instances of the component's workload.
	// +optional
	FormFactor *FormFactor `json:"formFactor,omitempty"`
	// FromImage contains the container image information.
	FromImage RegistrationFromImage `json:"fromImage"`
	// ParameterConfiguration sets how object elements
	// are transformed into workload parameters.
	// +optional
	ParameterConfiguration *ParameterConfiguration `json:"parameterConfiguration,omitempty"`
}

// RegistrationFromImage contains information to retrieve the container image.
type RegistrationFromImage struct {
	// Repo where the image can be downloaded
	Repo string `json:"repo"`
}

// ParameterConfiguration for the workload.
type ParameterConfiguration struct {
	// Global defines the configuration to be applied to all generated parameters.
	// +optional
	Global *GlobalParameterConfiguration `json:"global,omitempty"`

	// AddEnvs contains configurations for parameters to be added to the workload
	// not derived from the user instance.
	// +optional
	AddEnvs []corev1.EnvVar `json:"addEnvs,omitempty"`

	// Customize contains instructions to modify parameters generation from
	// the instance's spec.
	// +optional
	Customize []CustomizeParameterConfiguration `json:"customize,omitempty"`
}

// GlobalParameterConfiguration defines configuration to be applied to all generated parameters.
type GlobalParameterConfiguration struct {
	// DefaultPrefix to be appeneded to keys by all generated parameters.
	// This configuration does not affect parameter keys explicitly set by users.
	// +optional
	DefaultPrefix *string `json:"defaultPrefix,omitempty"`
}

func (gpc *GlobalParameterConfiguration) GetDefaultPrefix() string {
	if gpc == nil || gpc.DefaultPrefix == nil {
		return ""
	}
	return *gpc.DefaultPrefix
}

// // AddParameters contains instructions to add arbitrary parameters
// // to the workload.
// type AddParameterConfiguration struct {
// 	// List of environment variables to add to the container.
// 	Env []corev1.EnvVar `json:"env,omitempty"`
// }

// CustomizeParameters contains instructions to modify parameters generation from
// the instance's spec.
type CustomizeParameterConfiguration struct {
	// JSON simplified path for the parameter.
	Path string `json:"path"`

	// Render options for the parameter generation.
	// +optional
	Render *ParameterRenderConfiguration `json:"render,omitempty"`
}

// ParameterRenderConfiguration are the customization options for an specific
// parameter generation.
type ParameterRenderConfiguration struct {
	// Key is the name of the parameter to be created.
	// +optional
	Key *string `json:"key,omitempty"`

	// Value is a literal value to be assigned to the parameter.
	// +optional
	Value *string `json:"value,omitempty"`

	// ValueFromConfigMap is a reference to a ConfigMap.
	// +optional
	ValueFromConfigMap *ObjectReference `json:"valueFromConfigMap,omitempty"`

	// ValueFromSecret is a reference to a Secret.
	// +optional
	ValueFromSecret *ObjectReference `json:"valueFromSecret,omitempty"`

	// ValueFromBuiltInFunc configures the field to
	// be rendered acording to the chosen built-in function.
	// +optional
	ValueFromBuiltInFunc *BuiltInfunction `json:"valueFromBuiltInFunc,omitempty"`

	// Skip sets whether the object should skip rendering
	// as a workload parameter.
	// +optional
	Skip *bool `json:"skip,omitempty"`
}

func (prc *ParameterRenderConfiguration) IsValueOverriden() bool {
	if prc == nil || (prc.Value == nil &&
		prc.ValueFromConfigMap == nil &&
		prc.ValueFromSecret == nil &&
		prc.ValueFromBuiltInFunc == nil) {
		return false
	}
	return true
}

// Selects a key from a Secret or ConfigMap.
type ObjectReference struct {
	// Object name
	Name string `json:"name"`
	// The key to select.
	Key string `json:"key"`
}

// References a built-in function
type BuiltInfunction struct {
	// Function name
	Name string `json:"name"`
	// The key to select.
	Args []string `json:"args"`
}

func (prc *ParameterRenderConfiguration) IsSkip() bool {
	if prc == nil || prc.Skip == nil {
		return false
	}
	return *prc.Skip
}

func (prc *ParameterRenderConfiguration) GetKey() string {
	if prc == nil || prc.Key == nil {
		return ""
	}
	return *prc.Key
}
