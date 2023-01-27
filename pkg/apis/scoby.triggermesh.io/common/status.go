// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

// Part of this package is inspired by Knative implementation at knative/dev/pkg.
package common

import (
	"reflect"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return &StatusManager{
		status:             status,
		happyConditionType: happyCond,
		conditionTypes:     conds,

		time: realTime{},
	}
}

func (sm *StatusManager) InitializeforUpdate(generation int64) {
	// make sure all expected conditions are listed at the current status
	// by moving them to the front of the slice keeping the index.
	i := 0
	for _, c := range sm.status.Conditions {
		if _, ok := sm.conditionTypes[c.Type]; !ok {
			sm.status.Conditions[i] = c
			i++
		}
	}

	// if there are conditions that we do not expect at the tail, remove them.
	if i != len(sm.conditionTypes)-1 {
		sm.status.Conditions = sm.status.Conditions[:i]
	}

	// some expected conditions might be missing, or some not expected conditions
	// might e present
	//
	// this condition should be false almost always, nested loops inside should
	// not impact performance.
	if len(sm.conditionTypes) != len(sm.status.Conditions) {
		tt := sm.time.Now()
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
				})
		}
	}

	sort.Slice(sm.status.Conditions, func(i, j int) bool { return sm.status.Conditions[i].Type < sm.status.Conditions[j].Type })

	// update observed generation according to the reconciliation
	sm.status.ObservedGeneration = generation
}

func (sm *StatusManager) SetAnnotation(key, value string) {
	if sm.status.Annotations == nil {
		sm.status.Annotations = make(map[string]string, 1)
	}
	sm.status.Annotations[key] = value
}

func (sm *StatusManager) DeleteAnnotation(key string) {
	if sm.status.Annotations == nil {
		return
	}
	delete(sm.status.Annotations, key)
}

func (sm *StatusManager) SetCondition(c Condition) {
	var cds Conditions

	nonHappyMessages := []string{}
	allTrue := c.Status
	var happyReason string
	for _, sc := range sm.status.Conditions {
		switch sc.Type {
		case sm.happyConditionType:
			// Happy condition does not cast vote on happiness

		case c.Type:
			// If we'd only update the LastTransitionTime, then return.
			sc.LastTransitionTime = c.LastTransitionTime
			if reflect.DeepEqual(sc, c) {
				return
			}

			if c.Status != metav1.ConditionTrue && c.Message != "" {
				nonHappyMessages = append(nonHappyMessages, c.Message)
			}

		default:
			// if a condition is not set to true, global happiness
			// will be false.
			if sc.Status != metav1.ConditionTrue &&
				allTrue != metav1.ConditionFalse {
				allTrue = metav1.ConditionFalse
				happyReason = "NOTALLTRUE"

				if sc.Message != "" {
					// copy any message for this condition to the pool
					nonHappyMessages = append(nonHappyMessages, sc.Message)
				}
			}

			// add other conditions to the array
			cds = append(cds, c)
		}
	}

	// append created/updated condition
	c.LastTransitionTime = sm.time.Now()
	cds = append(cds, c)

	if allTrue == metav1.ConditionTrue {
		happyReason = "ALLTRUE"
	}

	// append happy condition
	cds = append(cds, Condition{
		Type:               sm.happyConditionType,
		Status:             allTrue,
		Message:            strings.Join(nonHappyMessages, "."),
		Reason:             happyReason,
		LastTransitionTime: c.LastTransitionTime,
	})

	sort.Slice(cds, func(i, j int) bool { return cds[i].Type < cds[j].Type })
	sm.status.Conditions = cds
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
