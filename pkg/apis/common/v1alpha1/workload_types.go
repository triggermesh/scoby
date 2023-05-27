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

	// Add contains instructions to render elements at the generated workload
	// not derived from the user instance.
	// +optional
	Add *AddConfiguration `json:"add,omitempty"`

	// FromSpec contains instructions to generate workload items from
	// the instance's spec.
	// +optional
	FromSpec *FromSpecConfiguration `json:"fromSpec,omitempty"`
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

// AddConfiguration contains instructions to add rendering elements
// not related to the user spec input.
type AddConfiguration struct {
	// Render options for adding environment variables unrelated to
	// the user's object input.
	// +optional
	ToEnv []AddToEnvConfiguration `json:"toEnv,omitempty"`

	// Render options for mounting volumes unrelated to
	// the user's object input.
	// Volume source must exists at the user's namespace.
	// +optional
	ToVolume []AddToVolumeConfiguration `json:"toVolume,omitempty"`
}

func (ac *AddConfiguration) IsEmpty() bool {
	return ac == nil || (len(ac.ToEnv) == 0 && len(ac.ToVolume) == 0)
}

// AddToEnvConfiguration are the customization options for an environment variable
// added from scratch.
type AddToEnvConfiguration struct {
	// Name is the name of the environment variable to be created.
	Name string `json:"name,omitempty"`

	// Value is a literal value to be assigned to the parameter.
	// +optional
	Value *string `json:"value,omitempty"`

	ValueFrom *AddToEnvValueFrom `json:"valueFrom,omitempty"`

	// ValueFromControllerConfigMap adds an environment variable whose value is
	// read from a ConfigMap at the controller's namespace.
	// +optional
	ValueFromControllerConfigMap *corev1.ConfigMapKeySelector `json:"valueFromControllerConfigMap,omitempty"`
}

// Instructions to extract envrionment variables values from the object's registration.
type AddToEnvValueFrom struct {
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`

	// Selects a key of a secret in the pod's namespace
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// Selects a field of the pod: supports metadata.name, metadata.namespace, `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`,
	// spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.
	// +optional
	FieldRef *corev1.ObjectFieldSelector `json:"field,omitempty" protobuf:"bytes,1,opt,name=fieldRef"`
}

func (aevf *AddToEnvValueFrom) ToEnvVarSource() *corev1.EnvVarSource {
	switch {
	case aevf.ConfigMap != nil:
		return &corev1.EnvVarSource{
			ConfigMapKeyRef: aevf.ConfigMap,
		}

	case aevf.Secret != nil:
		return &corev1.EnvVarSource{
			SecretKeyRef: aevf.Secret,
		}

	case aevf.FieldRef != nil:
		return &corev1.EnvVarSource{
			FieldRef: aevf.FieldRef,
		}
	}

	return nil
}

// Instructions to extract volume mount information from the object's registration.
type AddToVolumeConfiguration struct {
	// Name for the volume.
	Name string `json:"name,omitempty"`

	// Path where the file will be mounted.
	MountPath string `json:"mountPath,omitempty"`

	// ValueFrom references an object to mount.
	MountFrom MountFrom `json:"mountFrom,omitempty"`
}

// Instructions to look for the volume mount source.
type MountFrom struct {
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`

	// Selects a key of a secret in the pod's namespace
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`
}

// func (mf *MountFrom) IsValueOverriden() bool {
// 	if mf != nil && (mf.ConfigMap != nil ||
// 		mf.Secret != nil) {
// 		return true
// 	}
// 	return false
// }

// FromSpecConfiguration contains instructions to generate rendering from
// the controlled instance spec.
type FromSpecConfiguration struct {

	// Skip sets whether the object should skip rendering
	// as a workload item.
	// +optional
	Skip []FromSpecSkip `json:"skip,omitempty"`

	// Render options for generating environment variables derived from
	// the user's object input.
	// +optional
	ToEnv []FromSpecToEnv `json:"toEnv,omitempty"`

	// Render options for mounting volumes derived from
	// the user's object input.
	// +optional
	ToVolume []FromSpecToVolume `json:"toVolume,omitempty"`
}

func (sc *FromSpecConfiguration) IsEmpty() bool {
	return sc == nil || (len(sc.Skip) == 0 && len(sc.ToEnv) == 0 && len(sc.ToVolume) == 0)
}

// FromSpecToEnv is the customization option to avoid an spec
// path from generating any rendering output.
type FromSpecSkip struct {
	// JSON simplified path for the parameter.
	Path string `json:"path"`
}

// FromSpecToEnv are the customization options for an environment variable
// generated from an object spec.
type FromSpecToEnv struct {
	// JSON simplified path for the parameter.
	Path string `json:"path"`

	// Name is the name of the envr to be created.
	// +optional
	Name *string `json:"name,omitempty"`

	// Default to be assigned to the parameter when
	// a value is not provided by users.
	// +optional
	Default *SpecToEnvDefaultValue `json:"default,omitempty"`

	// ValueFrom uses a .
	// +optional
	ValueFrom *SpecToEnvValueFrom `json:"valueFrom,omitempty"`
}

// FromSpecToVolume are the customization options for a volume
// being mounted from configuration.
type FromSpecToVolume struct {
	// JSON simplified path for the parameter.
	Path string `json:"path"`

	// Name for the volume.
	Name string `json:"name,omitempty"`

	// Path where the file will be mounted.
	MountPath string `json:"mountPath,omitempty"`

	// ValueFrom references an object to mount.
	MountFrom MountFrom `json:"mountFrom,omitempty"`
}

func (fsc *FromSpecConfiguration) IsRenderer() bool {
	return fsc != nil && (fsc.ToEnv != nil || fsc.ToVolume == nil)
}

// Instructions to extract envrionment variables values from elements or functions
type SpecToEnvDefaultValue struct {
	// Value is a literal value to be assigned to the parameter.
	// +optional
	Value *string `json:"value,omitempty"`

	// Selects a key of a ConfigMap.
	// +optional
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`

	// Selects a key of a secret in the pod's namespace
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`
}

// Instructions to extract envrionment variables values from elements or functions
type SpecToEnvValueFrom struct {
	// Selects a key of a ConfigMap.
	// +optional
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`

	// Selects a key of a secret in the pod's namespace
	// +optional
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`

	// BuiltInFunc configures the field to
	// be rendered acording to the chosen built-in function.
	// +optional
	BuiltInFunc *BuiltInfunction `json:"builtInFunc,omitempty"`
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
