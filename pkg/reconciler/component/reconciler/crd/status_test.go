package crd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestStatusManagerSanitize(t *testing.T) {
	testCases := map[string]struct {
		statusFlag     StatusFlag
		inObject       *unstructured.Unstructured
		expectedObject *unstructured.Unstructured
	}{
		// "no status support": {
		// 	statusFlag:     0,
		// 	inObject:       unstructuredObjectStatus(),
		// 	expectedObject: unstructuredObjectStatus(),
		// },
		// "observed generation": {
		// 	statusFlag: StatusFlagObservedGeneration,
		// 	inObject:   unstructuredObjectStatus(),
		// 	expectedObject: unstructuredObjectStatus(
		// 		withObservedGeneration(0),
		// 	),
		// },
		"conditions": {
			statusFlag: StatusFlagConditionMessage |
				StatusFlagConditionStatus |
				StatusFlagConditionType |
				StatusFlagConditionReason |
				StatusFlagConditionLastTranstitionTime,
			inObject:       unstructuredObjectStatus(),
			expectedObject: unstructuredObjectStatus(),
			// withCondition("Condition1", "Unknown", tLastTransition.UTC().Format(time.RFC3339), ConditionReasonUnknown)),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			smf := &statusManagerFactory{
				flag:      tc.statusFlag,
				happyCond: tHappyCondition,
				conds:     tConditions,

				// override time provider at factory
				time: mockTime{},
			}

			_ = smf.ForObject(tc.inObject)

			assert.Equal(t, tc.expectedObject.Object, tc.inObject.Object)

			if tc.statusFlag.AllowConditions() {
				conditions, ok, err := unstructured.NestedSlice(tc.inObject.Object, "status", "conditions")
				if !ok {
					assert.Fail(t, "status.conditions not found", err)
				}

				for _, c := range conditions {
					t.Log(c)
				}
			}

		})
	}
}

type statusOption func(*unstructured.Unstructured)

func unstructuredObjectStatus(opts ...statusOption) *unstructured.Unstructured {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	for _, opt := range opts {
		opt(u)
	}

	return u
}

func withObservedGeneration(generation int64) statusOption {
	return func(u *unstructured.Unstructured) {
		unstructured.SetNestedField(u.Object, generation, "status", "observedGeneration")
	}
}
