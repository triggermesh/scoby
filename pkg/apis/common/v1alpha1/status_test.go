// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tHappyCondition       = "Ready"
	tGeneration     int64 = 3
	tReason               = "TEST"
)

var (
	tConditions = map[string]struct{}{
		"Condition1": {},
		"Condition2": {},
		"Condition3": {},
		"Ready":      {},
	}
	tLastTransition = metav1.NewTime(time.Now())
)

type mockTime struct{}

func (mockTime) Now() metav1.Time { return tLastTransition }

func TestStatusManagerInitialize(t *testing.T) {
	testCases := map[string]struct {
		inConditions       []Condition
		expectedConditions []Condition
	}{
		"empty status": {
			inConditions: []Condition{},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
		"previously initialized": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
		"partialy initialized": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
		"happiness from true to false": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonAllTrue,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
		"true condition": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
		"false condition": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
		"all true condition but happiness": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonAllTrue,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			status := &Status{Conditions: tc.inConditions}
			sm := newStatusManagerWithTime(status, tHappyCondition, tConditions, mockTime{})
			sm.SetObservedGeneration(tGeneration)

			// t.Logf("expected conditions: %+v", tc.expectedConditions)
			// t.Logf("real conditions %+v", status.Conditions)

			assert.ElementsMatch(t, tc.expectedConditions, status.Conditions)
			assert.Equal(t, tGeneration, status.ObservedGeneration)
		})
	}
}

func TestStatusManagerSetCondition(t *testing.T) {
	testCases := map[string]struct {
		inConditions       []Condition
		setCondition       Condition
		expectedConditions []Condition
	}{
		"added condition to empty status": {
			inConditions: []Condition{},
			setCondition: Condition{
				Type:   "Condition1",
				Status: metav1.ConditionUnknown,
				Reason: tReason,
			},

			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
		"set existing condition same value": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             tReason,
				},
			},
			setCondition: Condition{
				Type:   "Condition1",
				Status: metav1.ConditionTrue,
				Reason: tReason,
			},

			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
		"set existing condition from true to false": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             tReason,
				},
			},
			setCondition: Condition{
				Type:   "Condition1",
				Status: metav1.ConditionFalse,
				Reason: tReason,
			},

			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             tReason,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonUnknown,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
					Reason:             ConditionReasonNotAllTrue,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			status := &Status{Conditions: tc.inConditions}
			sm := newStatusManagerWithTime(status, tHappyCondition, tConditions, mockTime{})
			sm.SetCondition(tc.setCondition)

			// t.Logf("expected conditions: %+v", tc.expectedConditions)
			// t.Logf("real conditions %+v", status.Conditions)

			assert.ElementsMatch(t, tc.expectedConditions, status.Conditions)
		})
	}
}

func TestConditionByType(t *testing.T) {
	cs := Conditions{
		{Type: "condition1"},
		{Type: "condition2"},
		{Type: "condition3"},
	}

	testCases := map[string]struct {
		conditions    Conditions
		searchType    string
		expectedFound bool
	}{
		"found": {
			conditions:    cs,
			searchType:    "condition1",
			expectedFound: true,
		},
		"not found": {
			conditions:    cs,
			searchType:    "condition4",
			expectedFound: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			res := tc.conditions.GetByType(tc.searchType)
			assert.True(t, tc.expectedFound == (res != nil), "Condition set and search type do not properly match")
		})
	}

}
