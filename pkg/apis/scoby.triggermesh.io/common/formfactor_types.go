// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

// FormFactor contains workload form factor settings.
type FormFactor struct {
	// Deployment hosting the user workload.
	Deployment *DeploymentFormFactor `json:"deployment,omitempty"`
	// KnativeService hosting the user workload.
	KnativeService *KnativeServiceFormFactor `json:"knativeService,omitempty"`
}

// DeploymentFormFactor contains parameters for Deployment choice.
type DeploymentFormFactor struct {
	// Replicas for the deployment.
	Replicas int `json:"replicas"`

	// Service to create pointing to the deployment.
	// +optional
	Service *DeploymentService `json:"service"`
}

type DeploymentService struct {
	// Port exposed at the service.
	Port int `json:"port"`
	// Port exposed at the target deployment.
	TargetPort int `json:"targetPort"`
}

// KnativeServiceFormFactor contains parameters for Deployment choice.
type KnativeServiceFormFactor struct {
	// MinScale is the service minimum scaling replicas
	// +optional
	MinScale *int `json:"minScale"`
	// MaxScale is the service maximum scaling replicas
	// +optional
	MaxScale *int `json:"maxScale"`
	// Visibility is the network visibility for the service
	// +optional
	Visibility *string `json:"visibility,omitempty"`
}
