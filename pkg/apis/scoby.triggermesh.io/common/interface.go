// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// +kubebuilder:object:generate=false
type Registration interface {
	runtime.Object
	metav1.Object

	GetCRDNames() *apiextensionsv1.CustomResourceDefinitionNames
	GetGenerateVersion() *GenerateVersion
	GetGenerateDecoration() *GenerateDecoration
	GetWorkload() *Workload
	GetConfiguration() *Configuration
}
