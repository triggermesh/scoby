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

// FormFactorInfo for the configured renderer.
type FormFactorInfo struct {
	Name string `json:"name"`
}

// HookRequest sent to configured hooks.
type HookRequest struct {
	// Information about the Scoby configured renderer.
	FormFactor FormFactorInfo `json:"formFactor"`

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

	// Wrapper for internal error at Scoby, do not use it when
	// implementing hooks since it will not be serialized.
	Err error `json:"-"`
}

func (hre *HookResponseError) Error() string {
	if hre.Err != nil {
		return hre.Err.Error()
	}
	return hre.Message
}

func (hre *HookResponseError) IsContinue() bool {
	if hre.Continue == nil {
		return false
	}
	return *hre.Continue
}

func (hre *HookResponseError) IsPermanent() bool {
	if hre.Permanent == nil {
		return false
	}
	return *hre.Permanent
}

func (hre *HookResponseError) Unwrap() error {
	if hre == nil {
		return nil
	}
	return hre.Err
}

// HookResponse is the expected reconcile reply from configured hooks.
type HookResponse struct {
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
