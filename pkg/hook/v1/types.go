package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
)

// HookRequest sent to configured hooks.
type HookRequest struct {
	// Reference to the object that is being reconciled.
	Object commonv1alpha1.Reference `json:"object"`
	// Reuse the hook capability name as the phase for the
	// hook API.
	Phase commonv1alpha1.HookCapability `json:"phase"`
}

// HookRequestPreReconcile sent to configured hooks
// for the pre-reconcile phase.
type HookRequestPreReconcile struct {
	HookRequest `json:",inline"`
}

// HookRequestPostReconcile sent to configured hooks
// for the post-reconcile phase.
type HookRequestPostReconcile struct {
	HookRequest `json:",inline"`
	// Objects rendered so far at the reconciliation.
	Rendered []unstructured.Unstructured `json:"rendered,omitempty"`
}

// HookRequestFinalize sent to configured hooks
// for the finalize phase.
type HookRequestFinalize struct {
	HookRequest `json:",inline"`
}

// HookResponse is the expected reconcile reply from configured hooks.
type HookResponse struct {
	Error *HookResponseError `json:"error,omitempty"`

	// Workload whose elements should be merged with those that Scoby creates.
	Workload *HookResponseWorkload `json:"workload,omitempty"`

	// Status whose elements should be merged with those that Scoby creates.
	Status *commonv1alpha1.Status `json:"status,omitempty"`
}

// HookResponseFinalize is the expected finalize reply from configured hooks.
type HookResponseFinalize struct {
	Error *HookResponseError `json:"error,omitempty"`
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

// HookResponseWorkload contains workload elements that the hook
// sets on the generated elements.
type HookResponseWorkload struct {
	PodSpec        *corev1.PodSpec        `json:"podSpec,omitempty"`
	ServiceAccount *corev1.ServiceAccount `json:"serviceAccount,omitempty"`
}
