package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	tHappyCondition       = "Ready"
	tGeneration     int64 = 3
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
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
			},
		},
		"previously initialized": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
			},
		},
		"partialy initialized": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
			},
		},
		"happiness from true to unknown": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
			},
		},
		"some true entries": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
			},
		},
		"some false entries": {
			inConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
			},
			expectedConditions: []Condition{
				{
					Type:               "Condition1",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition2",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Condition3",
					Status:             metav1.ConditionUnknown,
					LastTransitionTime: tLastTransition,
				},
				{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: tLastTransition,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			status := &Status{Conditions: tc.inConditions}
			sm := NewStatusManager(status, tHappyCondition, tConditions)

			// Deterministic time for tests
			sm.time = mockTime{}

			sm.InitializeforUpdate(tGeneration)

			t.Logf("expected conditions: %+v", tc.expectedConditions)
			t.Logf("real conditions %+v", status.Conditions)

			assert.ElementsMatch(t, tc.expectedConditions, status.Conditions)
			assert.Equal(t, tGeneration, status.ObservedGeneration)
		})
	}

}
