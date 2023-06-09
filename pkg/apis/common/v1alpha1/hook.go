// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "knative.dev/pkg/apis"

// HookPhase identifies the phases where hooks can
// intercept the reconciliation process.
type HookPhase string
type HookCapabilities []HookPhase

type HookAPIVersion string

const (
	HookCapabilityPreReconcile HookPhase = "pre-reconcile"
	HookCapabilityFinalize     HookPhase = "finalize"

	HookAPIVersionV1 HookAPIVersion = "v1"

	CRDRegistrationAnnotationHookURL = "hookURL"
)

type Hook struct {
	Version HookAPIVersion `json:"version"`

	Address Address `json:"address"`

	// Timeout for hook calls.
	// +optional
	Timeout *string `json:"timeout"`

	// Capabilities that a hook implements.
	Capabilities HookCapabilities `json:"capabilities,omitempty"`
}

type Address struct {
	// Ref points to an addressable object.
	// +optional
	Ref *Reference `json:"ref,omitempty"`

	// URI can be an absolute URL(non-empty scheme and non-empty host) pointing to the target or a relative URI. Relative URIs will be resolved using the base URI retrieved from Ref.
	// +optional
	URI *apis.URL `json:"uri,omitempty"`
}

// Reference contains enough information to refer to another object.
// It's a trimmed down version of corev1.ObjectReference.
type Reference struct {
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind"`

	// Namespace of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/
	// This is optional field, it gets defaulted to the object holding it if left out.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the hook address.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`

	// API version of the hook address.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

func (hc HookCapabilities) IsFinalizer() bool {
	for i := range hc {
		if hc[i] == HookCapabilityFinalize {
			return true
		}
	}

	return false
}

func (hc HookCapabilities) IsPreReconciler() bool {
	for i := range hc {
		if hc[i] == HookCapabilityPreReconcile {
			return true
		}
	}

	return false
}
