// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

// FormFactor contains workload form factor settings.
type FormFactor struct {
	// Deployment hosting the user workload
	Deployment *DeploymentFormFactor `json:"deployment,omitempty"`
	// KnativeService hosting the user workload
	KnativeService *KnativeServiceFormFactor `json:"knativeService,omitempty"`
}

// DeploymentFormFactor contains parameters for Deployment choice.
type DeploymentFormFactor struct {
	// Replicas for the deployment
	Replicas int `json:"replicas"`
}

// KnativeServiceFormFactor contains parameters for Deployment choice.
type KnativeServiceFormFactor struct {
	// MinScale is the service minimum scaling replicas
	MinScale int `json:"minScale"`
	// MaxScale is the service maximum scaling replicas
	MaxScale int `json:"maxScale"`
	// Visibility is the network visibility for the service
	Visibility string `json:"visibility,omitempty"`
}
