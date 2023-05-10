// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

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
	// StatusConfiguration contains rules to populate
	// a controlled instance status.
	// +optional
	StatusConfiguration *StatusConfiguration `json:"statusConfiguration,omitempty"`
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

// CustomizeParameters contains instructions to modify parameters generation for
// the controlled instance spec.
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
	// Name is the name of the parameter to be created.
	// +optional
	Name *string `json:"name,omitempty"`

	// Value is a literal value to be assigned to the parameter when
	// a value is not provided by users.
	// +optional
	DefaultValue *string `json:"defaultValue,omitempty"`

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
	if prc == nil || (prc.ValueFromConfigMap == nil &&
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

// References a built-in function.
type BuiltInfunction struct {
	// Function name
	Name string `json:"name"`
	// The key to select.
	// +optional
	Args []string `json:"args,omitempty"`
}

// IsSkip returns if the parameter rendering should be skipped.
func (prc *ParameterRenderConfiguration) IsSkip() bool {
	if prc == nil || prc.Skip == nil {
		return false
	}
	return *prc.Skip
}

// GetName returns the key defined at the parameter rendering
// configuration.
// Returns an empty string if not defined.
func (prc *ParameterRenderConfiguration) GetName() string {
	if prc == nil || prc.Name == nil {
		return ""
	}
	return *prc.Name
}

// StatusConfiguration contains instructions to modify status generation for
// the controlled instance.
type StatusConfiguration struct {
	// AddElements contains configurations for status elements to be added.
	// +optional
	AddElements []StatusAddElement `json:"addElements,omitempty"`

	// ConditionsFromHook contains conditions expected to be informed from the Hook.
	// +optional
	ConditionsFromHook []ConditionsFromHook `json:"conditionsFromHook,omitempty"`
}

// StatusAddElement is a customization option that adds or fills an element
// at an object instance status structure.
type StatusAddElement struct {
	// JSON simplified path for the status element.
	Path string `json:"path"`

	// Render options for the status.
	// +optional
	Render *StatusRenderConfiguration `json:"render,omitempty"`
}

// StatusRenderConfiguration is a customization status option for the
// status generation.
type StatusRenderConfiguration struct {
	// Reference an object element and use its parameter to
	// fill the status.
	ValueFromParameter *StatusValueFromParameter `json:"valueFromParameter,omitempty"`
}

// StatusValueFromParameter contains a reference to an object element
// that is used at the status.
type StatusValueFromParameter struct {
	// JSON simplified path for the referenced element.
	Path string `json:"path"`
}

// ConditionsFromHook are extended conditions that must be informed from
// the configured Hook.
type ConditionsFromHook struct {
	// Type of the condition to be informed.
	Type string `json:"type"`
}
