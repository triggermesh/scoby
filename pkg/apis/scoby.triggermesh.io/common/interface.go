// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:object:generate=false
type Registration interface {
	runtime.Object
	metav1.Object

	GetWorkload() *Workload
	GetConfiguration() *Configuration
}
