// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/triggermesh/scoby/pkg/apis/scoby/common"
)

// CRDRegistrationSpec defines the desired state of a CRD Registration
type CRDRegistrationSpec struct {
	// Name of the CRD to be used.
	CRD string `json:"crd"`

	// Workload is information on how to create the user workload.
	Workload common.Workload `json:"workload"`
}

// CRDRegistrationStatus defines the observed state of CRDRegistration
type CRDRegistrationStatus struct {
	common.Status `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName={"crdreg"}

// CRDRegistration uses existing CRDs to provide generic controllers for them.
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".spec.crd"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
type CRDRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CRDRegistrationSpec `json:"spec"`

	// +optional
	Status CRDRegistrationStatus `json:"status,omitempty"`
}

var _ common.Registration = (*CRDRegistration)(nil)

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
