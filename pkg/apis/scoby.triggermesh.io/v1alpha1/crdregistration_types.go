// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
)

// CRDRegistrationSpec defines the desired state of a CRD Registration
type CRDRegistrationSpec struct {
	// Name of the CRD to be used.
	CRD string `json:"crd"`

	// // Workload is information on how to create the user workload.
	Workload common.Workload `json:"workload"`
}

// CRDRegistrationStatus defines the observed state of CRDRegistration
type CRDRegistrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CRDRegistration uses existing CRDs to provide generic controllers for them.
type CRDRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CRDRegistrationSpec   `json:"spec"`
	Status CRDRegistrationStatus `json:"status,omitempty"`
}

// var _ common.Registration = (*CRDRegistration)(nil)

//+kubebuilder:object:root=true

// CRDRegistrationList contains a list of CRDRegistration
type CRDRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CRDRegistration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CRDRegistration{}, &CRDRegistrationList{})
}
