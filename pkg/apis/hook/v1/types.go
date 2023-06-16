// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Phase identifies the phases where hooks can
// intercept the reconciliation process. Supported phases are `pre-reconcile` and `finalize`
type Phase string

const (
	PhasePreReconcile Phase = "pre-reconcile"
	PhaseFinalize     Phase = "finalize"
)

// HookRequest sent to configured hooks.
type HookRequest struct {
	// Reference to the object that is being reconciled.
	Object unstructured.Unstructured `json:"object"`

	// Reuse the hook capability name as the phase for the
	// hook API.
	Phase Phase `json:"phase"`

	// Children are generated kubernetes children objects that are to
	// be controlled from the Scoby controller.
	Children map[string]*unstructured.Unstructured `json:"children,omitempty"`
}

// HookResponseError contains the information that Scoby needs to
// handle an error that ocurred at a hook.
type HookResponseError struct {
	Message string `json:"message"`
	// When true, informs Scoby that the reconciliation cycle should
	// not be requeued after this error.
	Permanent *bool `json:"permanent,omitempty"`
	// When true, informs Scoby that the reconciliation process
	// should not stop after this error.
	Continue *bool `json:"continue,omitempty"`
}

// HookResponse is the expected reconcile reply from configured hooks.
type HookResponse struct {
	Error *HookResponseError `json:"error,omitempty"`

	// Object that triggered the reconciliation and whose status might
	// have been modified from the hook.
	//
	// An empty object at the hook response means no changes to the
	// reconciling object.
	Object *unstructured.Unstructured `json:"object,omitempty"`

	// Children are generated kubernetes children objects that are to
	// be controlled from the Scoby controller and that might have been
	// modified from the hook.
	Children map[string]*unstructured.Unstructured `json:"children,omitempty"`
}
