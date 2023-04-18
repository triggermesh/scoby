// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

// Part of this package is inspired by Knative implementation at knative/dev/pkg.
package v1alpha1

import (
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionReasonUnknown    = "UNKNOWN"
	ConditionReasonAllTrue    = "CONDITIONSOK"
	ConditionReasonNotAllTrue = "CONDITIONSNOTOK"
)

type Condition struct {
	// type of condition in CamelCase or in foo.example.com/CamelCase.
	// ---
	// Many .condition.type values are consistent across resources like Available, but because arbitrary conditions can be
	// useful (see .node.status.conditions), the ability to deconflict is important.
	// The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
	// +kubebuilder:validation:MaxLength=316
	Type string `json:"type"`
	// status of the condition, one of True, False, Unknown.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=True;False;Unknown
	Status metav1.ConditionStatus `json:"status"`
	// lastTransitionTime is the last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// reason contains a programmatic identifier indicating the reason for the condition's last transition.
	// Producers of specific condition types may define expected values and meanings for this field,
	// and whether the values are considered a guaranteed API.
	// The value should be a CamelCase string.
	// This field may not be empty.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`
	Reason string `json:"reason"`
	// message is a human readable message indicating details about the transition.
	// This may be an empty string.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32768
	Message string `json:"message"`
}

// Conditions is the schema for the conditions portion of the payload
type Conditions []Condition

// Given a type returns a pointer to the condition that holds it,
// nil if it does not exist.
func (cs Conditions) GetByType(t string) *Condition {
	for i := range cs {
		if cs[i].Type == t {
			return &cs[i]
		}
	}

	return nil
}

type Status struct {
	// ObservedGeneration is the 'Generation' of the Object that
	// was last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions the latest available observations of a resource's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Annotations is additional Status fields for the Resource to save some
	// additional State as well as convey more information to the user. This is
	// roughly akin to Annotations on any k8s resource, just the reconciler conveying
	// richer information outwards.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Time is a helper wrap around time.Now that
// enables us to write tests.
// +kubebuilder:object:generate=false
type Time interface {
	Now() metav1.Time
}

type realTime struct{}

func (realTime) Now() metav1.Time { return metav1.NewTime(time.Now()) }

// +kubebuilder:object:generate=false
type StatusManager struct {
	status *Status

	// Type for the condition that summarizes
	// happiness for the object.
	happyConditionType string

	// Condition types defined for the object,
	// including the happy condition type
	conditionTypes map[string]struct{}

	time Time
}

func NewStatusManager(status *Status, happyCond string, conds map[string]struct{}) *StatusManager {
	return newStatusManagerWithTime(status, happyCond, conds, realTime{})
}

// Allows overriding time for tests
func newStatusManagerWithTime(status *Status, happyCond string, conds map[string]struct{}, time Time) *StatusManager {
	sm := &StatusManager{
		status:             status,
		happyConditionType: happyCond,
		conditionTypes:     conds,

		time: time,
	}

	sm.sanitizeConditions()

	return sm
}

func (sm *StatusManager) sanitizeConditions() {
	// make sure all expected conditions are listed at the current status
	// by moving them to the front of the slice keeping the index.
	i := 0
	happyStatus := metav1.ConditionTrue
	for _, c := range sm.status.Conditions {
		if _, ok := sm.conditionTypes[c.Type]; ok {
			sm.status.Conditions[i] = c

			// advance index of valid conditions
			i++

			// track readiness, but don't let readiness vote
			// on itself, only dependent conditions
			if c.Type == sm.happyConditionType {
				continue
			}

			switch c.Status {
			case metav1.ConditionFalse:
				if happyStatus != metav1.ConditionFalse {
					happyStatus = metav1.ConditionFalse
				}
			case metav1.ConditionUnknown:
				if happyStatus == metav1.ConditionTrue {
					happyStatus = metav1.ConditionUnknown
				}
			}

		}
	}

	// if there are conditions that we do not expect at the tail, remove them.
	if i != len(sm.conditionTypes)-1 {
		sm.status.Conditions = sm.status.Conditions[:i]
	}

	// Use the same last transition time for all conditions
	tt := sm.time.Now()

	// some expected conditions might be missing, or some not expected conditions
	// might be present
	//
	// this loop should is not expected often, almost never. Nested loops inside should
	// not impact performance.
	if len(sm.conditionTypes) != len(sm.status.Conditions) {
		// new elements are going to be added, set global readiness to unknown
		if happyStatus == metav1.ConditionTrue {
			happyStatus = metav1.ConditionUnknown
		}

		for k := range sm.conditionTypes {
			found := false
			for i = range sm.status.Conditions {
				if sm.status.Conditions[i].Type == k {
					found = true
					break
				}
			}

			if found {
				continue
			}

			sm.status.Conditions = append(sm.status.Conditions,
				Condition{
					Type:               k,
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tt,
					Reason:             ConditionReasonUnknown,
				})
		}
	}

	// adjust happiness
	for i := range sm.status.Conditions {
		if sm.status.Conditions[i].Type == sm.happyConditionType {
			hp := &sm.status.Conditions[i]
			if happyStatus == metav1.ConditionTrue {
				if hp.Reason != ConditionReasonAllTrue || hp.Status != metav1.ConditionTrue {
					hp.Reason = ConditionReasonAllTrue
					hp.Status = metav1.ConditionTrue
					hp.LastTransitionTime = tt
					break
				}
			} else if hp.Reason != ConditionReasonNotAllTrue || hp.Status != metav1.ConditionFalse {
				hp.Reason = ConditionReasonNotAllTrue
				hp.Status = metav1.ConditionFalse
				hp.LastTransitionTime = tt

			}
			break
		}
	}

	sort.Slice(sm.status.Conditions, func(i, j int) bool { return sm.status.Conditions[i].Type < sm.status.Conditions[j].Type })
}

func (sm *StatusManager) SetObservedGeneration(generation int64) {
	sm.status.ObservedGeneration = generation
}

func (sm *StatusManager) SetCondition(c Condition) {
	for i := range sm.status.Conditions {
		sc := &sm.status.Conditions[i]
		if sc.Type == c.Type {
			// copy values to avoid creating a new object
			sc.Message = c.Message
			sc.LastTransitionTime = sm.time.Now()
			sc.Status = c.Status
			sc.Reason = c.Reason
			break
		}
	}

	sm.sanitizeConditions()
}

func (sm *StatusManager) SetAnnotation(key, value string) {
	if sm.status.Annotations == nil {
		sm.status.Annotations = make(map[string]string, 1)
	}
	sm.status.Annotations[key] = value
}

func (sm *StatusManager) GetAnnotation(key string) (bool, *string) {
	if sm.status.Annotations != nil {
		if v, ok := sm.status.Annotations[key]; ok {
			return true, &v
		}
	}

	return false, nil
}

func (sm *StatusManager) DeleteAnnotation(key string) {
	if sm.status.Annotations == nil {
		return
	}
	delete(sm.status.Annotations, key)
}

func (sm *StatusManager) MarkConditionTrue(condtype, reason string) {
	sm.SetCondition(Condition{
		Type:               condtype,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		LastTransitionTime: sm.time.Now(),
	})
}

func (sm *StatusManager) MarkConditionFalse(condtype, reason, message string) {
	sm.SetCondition(Condition{
		Type:               condtype,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: sm.time.Now(),
		Reason:             reason,
		Message:            message,
	})
}
