package v1

import (
	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Operation string

const (
	OperationReconcile Operation = "reconcile"
	OperationFinalize  Operation = "finalize"
)

// HookRequest sent to configured hooks
type HookRequest struct {
	Object    commonv1alpha1.Reference `json:"object"`
	Operation Operation                `json:"operation"`
}

// HookResponse is the expected reply from configured hooks.
type HookResponse struct {
	Status HookStatus `json:"status"`

	// EnvVars contains parameters to be added to the workload.
	EnvVars []corev1.EnvVar `json:"addEnvs,omitempty"`
}

// HookStatus is the status information provided by the hook.
type HookStatus struct {
	// Conditions the latest available observations of a resource's current state.
	Conditions commonv1alpha1.Conditions `json:"conditions,omitempty"`

	// Annotations is additional Status fields for the Resource to save some
	// additional State as well as convey more information to the user. This is
	// roughly akin to Annotations on any k8s resource, just the reconciler conveying
	// richer information outwards.
	Annotations map[string]string `json:"annotations,omitempty"`
}
