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

	// AddEnvs contains instructions to create environment variables at the workload
	// not derived from the user instance.
	// +optional
	AddEnvs []corev1.EnvVar `json:"addEnvs,omitempty"`

	// FromSpec contains instructions to generate workload items from
	// the instance's spec.
	// +optional
	FromSpec []FromSpecConfiguration `json:"fromSpec,omitempty"`

	// // SpecToVolumes contains instructions to generate volumes and mounts from
	// // the instance's spec.
	// // +optional
	// SpecToVolumes []SpecToVolumeParameterConfiguration `json:"specToVolumes,omitempty"`
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

// FromSpecConfiguration contains instructions to generate rendering from
// the controlled instance spec.
type FromSpecConfiguration struct {
	// JSON simplified path for the parameter.
	Path string `json:"path"`

	// Skip sets whether the object should skip rendering
	// as a workload item.
	// +optional
	Skip *bool `json:"skip,omitempty"`

	// Render options for the parameter generation.
	// +optional
	ToEnv *SpecToEnvConfiguration `json:"toEnv,omitempty"`

	// Render options for the parameter generation.
	// +optional
	ToVolume *SpecToVolumeConfiguration `json:"toVolume,omitempty"`
}

func (fsc *FromSpecConfiguration) IsRenderer() bool {
	return fsc != nil && (fsc.ToEnv != nil || fsc.ToVolume == nil)
}

func (fsc *FromSpecConfiguration) IsValueOverriden() bool {
	if fsc == nil || (!fsc.ToEnv.IsValueOverriden() && !fsc.ToVolume.IsValueOverriden()) {
		return false
	}
	return true
}

// IsSkip returns if the parameter rendering should be skipped.
func (fsc *FromSpecConfiguration) IsSkip() bool {
	if fsc == nil || fsc.Skip == nil {
		return false
	}
	return *fsc.Skip
}

// // SpecToVolumeParameterConfiguration contains instructions to generate volumes
// // and volume mounts from the controlled instance spec.
// type SpecToVolumeParameterConfiguration struct {
// 	// JSON simplified path for the parameter.
// 	Path string `json:"path"`

// 	Render *SpecToVolumeRenderConfiguration `json:"render,omitempty"`
// }

// SpecToVolumeRenderConfiguration are the customization options for an specific
// parameter generation.
type SpecToVolumeConfiguration struct {
	// Name for the volume.
	Name string `json:"name,omitempty"`

	// Path where the file will be mounted.
	MountPath string `json:"mountPath,omitempty"`

	// ValueFromConfigMap is a reference to a ConfigMap.
	// +optional
	ValueFromConfigMap *ObjectReference `json:"valueFromConfigMap,omitempty"`

	// ValueFromSecret is a reference to a Secret.
	// +optional
	ValueFromSecret *ObjectReference `json:"valueFromSecret,omitempty"`
}

func (svc *SpecToVolumeConfiguration) IsValueOverriden() bool {
	if svc == nil || (svc.ValueFromConfigMap == nil &&
		svc.ValueFromSecret == nil) {
		return false
	}
	return true
}

// SpecToEnvRenderConfiguration are the customization options for an specific
// parameter generation.
type SpecToEnvConfiguration struct {
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
}

// GetName returns the key defined at the parameter rendering
// configuration.
// Returns an empty string if not defined.
func (sec *SpecToEnvConfiguration) GetName() string {
	if sec == nil || sec.Name == nil {
		return ""
	}
	return *sec.Name
}

func (sec *SpecToEnvConfiguration) IsValueOverriden() bool {
	if sec == nil || (sec.ValueFromConfigMap == nil &&
		sec.ValueFromSecret == nil &&
		sec.ValueFromBuiltInFunc == nil) {
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
