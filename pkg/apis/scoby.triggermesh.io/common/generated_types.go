// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

// GenerateNames contains naming parameters for CRD generation.
type GenerateNames struct {
	// Kind for the CRD resource that will be generated from this component registration.
	// When not informed the capitalized singular Sield is used.
	// +optional
	Kind *string `json:"kind,omitempty"`

	// Singular is the name of the CRD resource.
	Singular string `json:"singular,omitempty"`

	// Plural name for the the CRD resource.
	Plural string `json:"plural,omitempty"`
}

type GenerateVersion struct {
	// Version for the generated CRD resource.
	Version string `json:"version,omitempty"`

	// Whether the resource is enabled at the API or not.
	// +kubebuilder:default:=true
	Served bool `json:"served,omitempty"`

	// Marks if this version should be used for internal storage.
	// +kubebuilder:default:=true
	Storage bool `json:"storage,omitempty"`
}

// GenerateDecoration contains elements that decorate generated CRDs.
type GenerateDecoration struct {
	// Labels to be added to the generated CRD object
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations to be added to the generated CRD object
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}
