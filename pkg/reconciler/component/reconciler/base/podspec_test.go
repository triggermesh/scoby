package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	// "sigs.k8s.io/yaml"
	"gopkg.in/yaml.v3"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

const (
	tName  = "testName"
	tImage = "testImage:v.test"

	objectText = `
spec:
  age: 90
  name: danny
  skills:
    cooking: false
    dancing: true
  colors:
  - Green
  - white
  animals:
  - dragonfly
  numbers:
  - 1
  - 1.1
  - 0
  mixed:
  - true
  - 13
  - barnacle
  substruct:
  - a: 1
  - b: 2

`
)

var (
	tTrue    = true
	tLiteral = "My_New_Name"
)

func TestParseObjectIntoContainer(t *testing.T) {

	testCases := map[string]struct {
		configuration *apicommon.ParameterConfiguration
		object        string

		podSpec *corev1.PodSpec
	}{
		"default rendering": {
			configuration: &apicommon.ParameterConfiguration{
				Parameters: []apicommon.Parameter{},
			},
			object: objectText,
			podSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
			),
		},
		"skip a value field": {
			configuration: &apicommon.ParameterConfiguration{
				Parameters: []apicommon.Parameter{
					{
						Path: "$.spec.age",
						Skip: &tTrue,
					},
				},
			},
			object: objectText,
			podSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
			),
		},
		"skip an array field": {
			configuration: &apicommon.ParameterConfiguration{
				Parameters: []apicommon.Parameter{
					{
						Path: "$.spec.numbers",
						Skip: &tTrue,
					},
				},
			},
			object: objectText,
			podSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
			),
		},
		"change a liternal": {
			configuration: &apicommon.ParameterConfiguration{
				Parameters: []apicommon.Parameter{
					{
						Path: ".spec.age",
						Render: &apicommon.ParameterRender{
							Literal: &tLiteral,
						},
					},
				},
			},
			object: objectText,
			podSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
			),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			psr := NewPodSpecRenderer(tName, tImage, tc.configuration)

			pso, err := psr.Render(newReconciledObjectFromYaml(tc.object))
			require.NoError(t, err)

			// Create PodSpec with returned options to compare results
			ps := resources.NewPodSpec(pso...)

			// assert.Equal(t, corev1.TerminationMessageFallbackToLogsOnError, c.TerminationMessagePolicy)
			if t == nil {
				assert.Equal(t, tc.podSpec, ps)
			}

		})
	}

}

// // Delete this
// func TestParseWithConfiguration(t *testing.T) {

// 	testCases := map[string]struct {
// 		configuration *apicommon.ParameterConfiguration
// 		object        ReconciledObject
// 	}{
// 		"first try": {
// 			configuration: &apicommon.ParameterConfiguration{
// 				Parameters: []apicommon.Parameter{
// 					{
// 						Path: "$.spec.integer",
// 						// Path: ".spec.integer",
// 						//Path: "lala.land",
// 						// Path: "2.spec.integer",
// 					},
// 				},
// 			},
// 			object: newReconciledObject(),
// 		},
// 	}

// 	for name, tc := range testCases {
// 		t.Run(name, func(t *testing.T) {
// 			psr := podSpecRenderer{
// 				configuration: tc.configuration,
// 			}
// 			_, err := psr.parseWithConfiguration(tc.object)
// 			assert.NoError(t, err)
// 		})
// 	}

// }

// func newReconciledObject() ReconciledObject {
// 	ro := &reconciledObject{
// 		Unstructured: &unstructured.Unstructured{
// 			Object: make(map[string]interface{}),
// 		},
// 	}

// 	unstructured.SetNestedField(ro.Object, "20", "spec", "integer")
// 	unstructured.SetNestedField(ro.Object, "hello", "spec", "string")

// 	return ro
// }

func newReconciledObjectFromYaml(in string) ReconciledObject {
	ro := &reconciledObject{
		Unstructured: &unstructured.Unstructured{
			Object: make(map[string]interface{}),
		},
	}

	err := yaml.Unmarshal([]byte(in), &ro.Unstructured.Object)
	if err != nil {
		panic(err)
	}

	return ro
}

func newPodSpec(co ...resources.ContainerOption) *corev1.PodSpec {
	co = append(co, resources.ContainerWithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError))
	c := resources.NewContainer(tName, tImage, co...)

	ps := &corev1.PodSpec{
		Containers: []corev1.Container{*c},
	}

	return ps
}
