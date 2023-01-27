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

func TestStatusManager(t *testing.T) {
	testCases := map[string]struct {
		status             *Status
		expectedConditions []Condition
	}{
		"empty status": {
			status: &Status{},
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sm := NewStatusManager(tc.status, tHappyCondition, tConditions)

			// Deterministic time for tests
			sm.time = mockTime{}

			sm.InitializeforUpdate(tGeneration)

			t.Log(tc.status)

			assert.ElementsMatch(t, tc.expectedConditions, tc.status.Conditions)
			assert.Equal(t, tGeneration, tc.status.ObservedGeneration)
		})
	}

}
