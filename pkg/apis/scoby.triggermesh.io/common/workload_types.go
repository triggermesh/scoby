// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

// Workload contains workload settings.
type Workload struct {
	// FormFactor indicates the kubernetes object that
	// will run instances of the component's workload.
	FormFactor FormFactor `json:"formFactor,omitempty"`
	// FromImage contains the container image information.
	FromImage RegistrationFromImage `json:"fromImage"`
	// ParameterOptions sets how parameters from configuration
	// and credentials are passed to workloads.
	// +optional
	ParameterOptions *ParameterOptions `json:"parameterOptions"`
}

// RegistrationFromImage contains information to retrieve the container image.
type RegistrationFromImage struct {
	// Repo where the image can be downloaded
	Repo string `json:"repo"`
}

type KeyCasing string
type KeyStyle string

const (
	KeyCasingUpper KeyCasing = "upper"
	KeyCasingLower KeyCasing = "lower"

	KeyStyleSnake KeyStyle = "snake"
	KeyStyleCamel KeyStyle = "camel"
)

// ParameterOptions for passing object property values to workloads.
type ParameterOptions struct {
	// ArbitraryParameters allows users to add any parameter to
	// the component spec.
	ArbitraryParameters *bool `json:"arbitraryParameters"`
	// KeyCasing turns a parameter key casing when used at the workload.
	KeyCasing *KeyCasing `json:"keyCasing,omitempty"`
	// KeyStyle turns a parameter key style when used at the workload.
	KeyStyle *KeyStyle `json:"keyStyle,omitempty"`
	// KeyPrefix adds a prefix to a parameter key when used at the workload.
	KeyPrefix *string `json:"keyPrefix,omitempty"`
}
