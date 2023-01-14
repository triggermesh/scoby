// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
)

// GenericRegistrationSpec defines the desired state of GenericRegistration
type GenericRegistrationSpec struct {
	// Generate contains global attributes for the CRD.
	Generate GenericRegistrationGenerate `json:"generate"`

	// // Workload is information on how to create the user workload.
	Workload common.Workload `json:"workload"`

	// // Configuration contains parameter definitions.
	// Configuration Configuration `json:"configuration,omitempty"`
}

// GenericRegistrationGenerate contains parameters for CRD generation.
type GenericRegistrationGenerate struct {
	// Names is the group parameters that help naming the CRD.
	Names common.GenerateNames `json:"names"`

	Version *common.GenerateVersion `json:"version,omitempty"`

	common.GenerateDecoration `json:",inline,omitempty"`
}

// GenericRegistrationStatus defines the observed state of GenericRegistration
type GenericRegistrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GenericRegistration let users create new custom resources that are
// reconciled using generic controllers.
type GenericRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GenericRegistrationSpec   `json:"spec"`
	Status GenericRegistrationStatus `json:"status,omitempty"`
}

var _ common.Registration = (*GenericRegistration)(nil)

//+kubebuilder:object:root=true

// GenericRegistrationList contains a list of GenericRegistration
type GenericRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GenericRegistration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GenericRegistration{}, &GenericRegistrationList{})
}
