package object

import (
	"testing"

	"gopkg.in/yaml.v3"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

const (
	tName  = "testName"
	tImage = "testImage:v.test"

	object1 = `
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
  arraySubstruct:
  - a: 1
  - b: 2
  arrayComplexSubstruct:
  - a:
    - x: 1
    - y: 2
  - b:
    - x: 3
    - y: 4
  valueFromReference:
    key: reference-key
    name: reference-name
  uriSinkByRef:
    ref:
      apiVersion: v1
      kind: Service
      namespace: test-namespace
      name: test-name
  uriSinkByURI:
    uri: http://dummy
`
)

var (
	tTrue     = true
	tEnvName  = "ALIAS"
	tEnvValue = "no name"
)

func TestObjectRender(t *testing.T) {
	testCases := map[string]struct {
		configuration apicommon.ParameterConfiguration
		object        string

		expectedPodSpec *corev1.PodSpec
	}{
		"default rendering": {
			configuration: apicommon.ParameterConfiguration{
				Customize: []apicommon.CustomizeParameterConfiguration{},
			},
			object: object1,
			expectedPodSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("ARRAYCOMPLEXSUBSTRUCT", `[{"a":[{"x":1},{"y":2}]},{"b":[{"x":3},{"y":4}]}]`),
				resources.ContainerAddEnvFromValue("ARRAYSUBSTRUCT", `[{"a":1},{"b":2}]`),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_APIVERSION", "v1"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_KIND", "Service"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAME", "test-name"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAMESPACE", "test-namespace"),
				resources.ContainerAddEnvFromValue("URISINKBYURI_URI", "http://dummy"),
				resources.ContainerAddEnvFromValue("VALUEFROMREFERENCE_KEY", "reference-key"),
				resources.ContainerAddEnvFromValue("VALUEFROMREFERENCE_NAME", "reference-name"),
			),
		},
		"skip element rendering": {
			configuration: apicommon.ParameterConfiguration{
				Customize: []apicommon.CustomizeParameterConfiguration{
					{
						Path: "spec.age",
						Render: &apicommon.ParameterRenderConfiguration{
							Skip: &tTrue,
						},
					},
					{
						Path: "spec.arraySubstruct",
						Render: &apicommon.ParameterRenderConfiguration{
							Skip: &tTrue,
						},
					},
				},
			},
			object: object1,
			expectedPodSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("ARRAYCOMPLEXSUBSTRUCT", `[{"a":[{"x":1},{"y":2}]},{"b":[{"x":3},{"y":4}]}]`),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_APIVERSION", "v1"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_KIND", "Service"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAME", "test-name"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAMESPACE", "test-namespace"),
				resources.ContainerAddEnvFromValue("URISINKBYURI_URI", "http://dummy"),
				resources.ContainerAddEnvFromValue("VALUEFROMREFERENCE_KEY", "reference-key"),
				resources.ContainerAddEnvFromValue("VALUEFROMREFERENCE_NAME", "reference-name"),
			),
		},
		"rename key": {
			configuration: apicommon.ParameterConfiguration{
				Customize: []apicommon.CustomizeParameterConfiguration{
					{
						Path: "spec.name",
						Render: &apicommon.ParameterRenderConfiguration{
							Key: &tEnvName,
						},
					},
				},
			},
			object: object1,
			expectedPodSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue(tEnvName, "danny"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("ARRAYCOMPLEXSUBSTRUCT", `[{"a":[{"x":1},{"y":2}]},{"b":[{"x":3},{"y":4}]}]`),
				resources.ContainerAddEnvFromValue("ARRAYSUBSTRUCT", `[{"a":1},{"b":2}]`),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_APIVERSION", "v1"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_KIND", "Service"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAME", "test-name"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAMESPACE", "test-namespace"),
				resources.ContainerAddEnvFromValue("URISINKBYURI_URI", "http://dummy"),
				resources.ContainerAddEnvFromValue("VALUEFROMREFERENCE_KEY", "reference-key"),
				resources.ContainerAddEnvFromValue("VALUEFROMREFERENCE_NAME", "reference-name"),
			),
		},
		"replace value": {
			configuration: apicommon.ParameterConfiguration{
				Customize: []apicommon.CustomizeParameterConfiguration{
					{
						Path: "spec.arrayComplexSubstruct",
						Render: &apicommon.ParameterRenderConfiguration{
							Value: &tEnvValue,
						},
					},
				},
			},
			object: object1,
			expectedPodSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("ARRAYCOMPLEXSUBSTRUCT", tEnvValue),
				resources.ContainerAddEnvFromValue("ARRAYSUBSTRUCT", `[{"a":1},{"b":2}]`),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_APIVERSION", "v1"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_KIND", "Service"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAME", "test-name"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAMESPACE", "test-namespace"),
				resources.ContainerAddEnvFromValue("URISINKBYURI_URI", "http://dummy"),
				resources.ContainerAddEnvFromValue("VALUEFROMREFERENCE_KEY", "reference-key"),
				resources.ContainerAddEnvFromValue("VALUEFROMREFERENCE_NAME", "reference-name"),
			),
		},
		"render configmap": {
			configuration: apicommon.ParameterConfiguration{
				Customize: []apicommon.CustomizeParameterConfiguration{
					{
						Path: "spec.valueFromReference",
						Render: &apicommon.ParameterRenderConfiguration{
							ValueFromConfigMap: &apicommon.ObjectReference{
								Name: "spec.valueFromReference.name",
								Key:  "spec.valueFromReference.key",
							},
						},
					},
				},
			},
			object: object1,
			expectedPodSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("ARRAYCOMPLEXSUBSTRUCT", `[{"a":[{"x":1},{"y":2}]},{"b":[{"x":3},{"y":4}]}]`),
				resources.ContainerAddEnvFromValue("ARRAYSUBSTRUCT", `[{"a":1},{"b":2}]`),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_APIVERSION", "v1"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_KIND", "Service"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAME", "test-name"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAMESPACE", "test-namespace"),
				resources.ContainerAddEnvFromValue("URISINKBYURI_URI", "http://dummy"),
				resources.ContainerAddEnvVarFromConfigMap("VALUEFROMREFERENCE", "reference-name", "reference-key"),
			),
		},
		"render secret": {
			configuration: apicommon.ParameterConfiguration{
				Customize: []apicommon.CustomizeParameterConfiguration{
					{
						Path: "spec.valueFromReference",
						Render: &apicommon.ParameterRenderConfiguration{
							ValueFromSecret: &apicommon.ObjectReference{
								Name: "spec.valueFromReference.name",
								Key:  "spec.valueFromReference.key",
							},
						},
					},
				},
			},
			object: object1,
			expectedPodSpec: newPodSpec(
				resources.ContainerAddEnvFromValue("AGE", "90"),
				resources.ContainerAddEnvFromValue("ANIMALS", "dragonfly"),
				resources.ContainerAddEnvFromValue("ARRAYCOMPLEXSUBSTRUCT", `[{"a":[{"x":1},{"y":2}]},{"b":[{"x":3},{"y":4}]}]`),
				resources.ContainerAddEnvFromValue("ARRAYSUBSTRUCT", `[{"a":1},{"b":2}]`),
				resources.ContainerAddEnvFromValue("COLORS", "Green,white"),
				resources.ContainerAddEnvFromValue("MIXED", "true,13,barnacle"),
				resources.ContainerAddEnvFromValue("NAME", "danny"),
				resources.ContainerAddEnvFromValue("NUMBERS", "1,1.1,0"),
				resources.ContainerAddEnvFromValue("SKILLS_COOKING", "false"),
				resources.ContainerAddEnvFromValue("SKILLS_DANCING", "true"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_APIVERSION", "v1"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_KIND", "Service"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAME", "test-name"),
				resources.ContainerAddEnvFromValue("URISINKBYREF_REF_NAMESPACE", "test-namespace"),
				resources.ContainerAddEnvFromValue("URISINKBYURI_URI", "http://dummy"),
				resources.ContainerAddEnvVarFromSecret("VALUEFROMREFERENCE", "reference-name", "reference-key"),
			),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			r := NewRenderer(tName, tImage, tc.configuration)

			rendered, err := r.Render(newReconciledObjectFromYaml(tc.object))
			require.NoError(t, err)

			// Create PodSpec with returned options to compare results
			ps := resources.NewPodSpec(rendered.GetPodSpecOptions()...)
			assert.Equal(t, tc.expectedPodSpec, ps)
		})
	}
}

func newReconciledObjectFromYaml(in string) Reconciling {
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
