package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type envVars []struct {
	key       string
	value     string
	prevValue string
}

func (ev envVars) set() {
	for i := range ev {
		ev[i].prevValue = os.Getenv(ev[i].key)
		os.Setenv(ev[i].key, ev[i].value)
	}
}

func (ev envVars) unset() {
	for i := range ev {
		if ev[i].prevValue != "" {
			os.Setenv(ev[i].key, ev[i].prevValue)
			continue
		}
		os.Unsetenv(ev[i].key)
	}
}

func TestScobyConfig(t *testing.T) {

	testCases := map[string]struct {
		envs envVars

		expectedPanic          string
		expectedScobyNamespace string
	}{
		"scoby namespace not informed": {
			expectedPanic: "required key SCOBY_NAMESPACE missing value",
		},
		"all envs informed": {
			envs: envVars{
				{
					key:   "SCOBY_NAMESPACE",
					value: "triggermesh",
				},
			},
			expectedScobyNamespace: "triggermesh",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tc.expectedPanic == "" {
					assert.Nil(t, r)
				} else {
					assert.ErrorContains(t, r.(error), tc.expectedPanic)
				}
			}()

			tc.envs.set()
			defer tc.envs.unset()

			ParseFromEnvironment()

			assert.Equal(t, tc.expectedScobyNamespace, Get().ScobyNamespace())
		})
	}
}
