// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	hookv1 "github.com/triggermesh/scoby/pkg/apis/hook/v1"
)

type HookCapabilities []hookv1.Phase

type HookAPIVersion string

const (
	// Status annotation name for the resolved Hook URL
	CRDRegistrationAnnotationHookURL = "hookURL"
)

type Hook struct {
	Version HookAPIVersion `json:"version"`

	Address Destination `json:"address"`

	// Timeout for hook calls.
	// +optional
	Timeout *string `json:"timeout"`

	// Capabilities that a hook implements.
	Capabilities HookCapabilities `json:"capabilities,omitempty"`
}

func (hc HookCapabilities) IsFinalizer() bool {
	for i := range hc {
		if hc[i] == hookv1.PhaseFinalize {
			return true
		}
	}

	return false
}

func (hc HookCapabilities) IsPreReconciler() bool {
	for i := range hc {
		if hc[i] == hookv1.PhasePreReconcile {
			return true
		}
	}

	return false
}
