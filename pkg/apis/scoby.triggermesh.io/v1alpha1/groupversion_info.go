// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 contains API Schema definitions for the scoby API group
// +kubebuilder:object:generate=true
// +groupName=scoby.triggermesh.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "scoby.triggermesh.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
