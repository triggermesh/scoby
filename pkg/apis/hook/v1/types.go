package v1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
)

// HookRequest sent to configured hooks.
type HookRequest struct {
	// Reference to the object that is being reconciled.
	Object *unstructured.Unstructured `json:"object"`

	// Candidates are generated kubernetes objects that are to be created,
	// but are still in an early rendering stage.
	// The hook has the chance of modifying them before creation.
	Candidates map[string]*unstructured.Unstructured `json:"candidates,omitempty"`

	// Reuse the hook capability name as the phase for the
	// hook API.
	Phase commonv1alpha1.HookPhase `json:"phase"`
}

// HookRequestPreReconcile sent to configured hooks
// for the pre-reconcile phase.
type HookRequestPreReconcile struct {
	HookRequest `json:",inline"`
}

// HookRequestFinalize sent to configured hooks
// for the finalize phase.
type HookRequestFinalize struct {
	HookRequest `json:",inline"`
}

// HookResponse is the expected reconcile reply from configured hooks.
type HookResponse struct {
	Error *HookResponseError `json:"error,omitempty"`

	// Candidates are generated kubernetes objects that might have been modified
	// by the hook processing.
	Candidates map[string]*unstructured.Unstructured `json:"candidates,omitempty"`

	// Status whose elements should be merged with those that Scoby creates.
	Status *commonv1alpha1.Status `json:"status,omitempty"`
}

// HookResponseFinalize is the expected finalize reply from configured hooks.
type HookResponseFinalize struct {
	Error *HookResponseError `json:"error,omitempty"`

	// Status whose elements should be merged with those that Scoby creates.
	Status *commonv1alpha1.Status `json:"status,omitempty"`
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
