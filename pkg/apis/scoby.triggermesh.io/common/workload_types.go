// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

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
	// Global defines  theconfiguration to be applied to all generated parameters.
	Global *GlobalParameterConfiguration `json:"global,omitempty"`

	// Add contains parameters to be added to the workload not derived from the
	// user instance.
	// +optional
	Add []AddParameterConfiguration `json:"add,omitempty"`

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

// AddParameters contains instructions to add arbitrary parameters
// to the workload.
type AddParameterConfiguration struct {
	// Key is the name of the parameter to be added.
	// +optional
	Key *string `json:"key,omitempty"`

	// Value is a literal value to be assigned to the parameter.
	// +optional
	Value *string `json:"value,omitempty"`
}

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

	// Skip sets whether the object should skip rendering
	// as a workload parameter.
	// +optional
	Skip *bool `json:"skip,omitempty"`
}
