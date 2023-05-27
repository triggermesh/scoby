package renderer

import (
	"context"
	"strings"
	"testing"

	tlogr "github.com/go-logr/logr/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	basecrd "github.com/triggermesh/scoby/pkg/component/reconciler/base/crd"
	baseobject "github.com/triggermesh/scoby/pkg/component/reconciler/base/object"
	basestatus "github.com/triggermesh/scoby/pkg/component/reconciler/base/status"
	"github.com/triggermesh/scoby/pkg/utils/configmap"
	"github.com/triggermesh/scoby/pkg/utils/resolver"
	"github.com/triggermesh/scoby/pkg/utils/resources"

	. "github.com/triggermesh/scoby/test"
)

const (
	tScobyNamespace = "triggermesh"
)

// The Kuard example contains a CRD with spec elements that
// use most features that Scoby provides.
var (
	gvk = &schema.GroupVersionKind{
		Group:   "extensions.triggermesh.io",
		Version: "v1",
		Kind:    "Kuard",
	}

	kuardCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: kuards.extensions.triggermesh.io
spec:
  group: extensions.triggermesh.io
  scope: Namespaced
  names:
    plural: kuards
    singular: kuard
    kind: Kuard
  versions:
    - name: v1
      served: true
      storage: true
      subresources:
        status: {}
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                # Root variables demo
                variable1:
                  type: string
                variable2:
                  type: string
                # Nested variables demo
                group:
                  type: object
                  properties:
                    variable3:
                      type: boolean
                    variable4:
                      type: integer
                # Simple array demo
                array:
                  type: array
                  items:
                    type: string
                # Secret reference demo
                refToSecret:
                  type: object
                  properties:
                    secretName:
                      type: string
                    secretKey:
                      type: string
                # ConfigMap reference demo
                refToConfigMap:
                  type: object
                  properties:
                    configName:
                      type: string
                    configKey:
                      type: string
                # URI resolving demo
                refToAddress:
                  type: object
                  properties:
                    ref:
                      type: object
                      properties:
                        apiVersion:
                          type: string
                        kind:
                          type: string
                        name:
                          type: string
                        namespace:
                          type: string
                      required:
                      - kind
                      - name
                      - apiVersion
                    uri:
                      type: string
                  oneOf:
                  - required: [ref]
                  - required: [uri]

            status:
              description: CRDRegistrationStatus defines the observed state of CRDRegistration
              properties:
                address:
                  description: URL exposed by this workload.
                  type: object
                  properties:
                    url:
                      type: string
                sinkUri:
                  description: URI this workload is pointing to.
                  type: string
                  format: uri
                annotations:
                  additionalProperties:
                    type: string
                  description: Annotations is additional Status fields for the Resource
                    to save some additional State as well as convey more information
                    to the user. This is roughly akin to Annotations on any k8s resource,
                    just the reconciler conveying richer information outwards.
                  type: object
                conditions:
                  description: Conditions the latest available observations of a resource's
                    current state.
                  items:
                    properties:
                      lastTransitionTime:
                        description: lastTransitionTime is the last time the condition
                          transitioned from one status to another. This should be when
                          the underlying condition changed.  If that is not known, then
                          using the time when the API field changed is acceptable.
                        format: date-time
                        type: string
                      message:
                        description: message is a human readable message indicating
                          details about the transition. This may be an empty string.
                        maxLength: 32768
                        type: string
                      reason:
                        description: reason contains a programmatic identifier indicating
                          the reason for the condition's last transition. Producers
                          of specific condition types may define expected values and
                          meanings for this field, and whether the values are considered
                          a guaranteed API. The value should be a CamelCase string.
                          This field may not be empty.
                        maxLength: 1024
                        minLength: 1
                        pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
                        type: string
                      status:
                        description: status of the condition, one of True, False, Unknown.
                        enum:
                        - "True"
                        - "False"
                        - Unknown
                        type: string
                      type:
                        description: type of condition in CamelCase or in foo.example.com/CamelCase.
                          --- Many .condition.type values are consistent across resources
                          like Available, but because arbitrary conditions can be useful
                          (see .node.status.conditions), the ability to deconflict is
                          important. The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
                        maxLength: 316
                        pattern: ^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$
                        type: string
                    required:
                    - status
                    - type
                    type: object
                  type: array
                observedGeneration:
                  description: ObservedGeneration is the 'Generation' of the Object
                    that was last processed by the controller.
                  format: int64
                  type: integer
              type: object

      additionalPrinterColumns:
      - jsonPath: .status.address.url
        name: URL
        type: string
      - jsonPath: .status.conditions[?(@.type=="Ready")].status
        name: Ready
        type: string
`

	kuardInstance = `
apiVersion: extensions.triggermesh.io/v1
kind: Kuard
metadata:
  name: my-kuard-extension
spec:
  variable1: value 1
  variable2: value 2
  group:
    variable3: false
    variable4: 42
  array:
  - alpha
  - beta
  - gamma
`
)

// func TestRenderedContainerCopy(t *testing.T) {

// 	// Use the Kuard CRD for all cases
// 	crdv := basecrd.CRDPrioritizedVersion(ReadCRD(kuardCRD))

// 	testCases := map[string]struct {
// 		// Resolver related rendering might need existing objects. The
// 		// kuard instance used for reconciliation does not need to be
// 		// here, only any referenced object.
// 		existingObjects []client.Object

// 		// Kuard instance fir rendering.
// 		kuardInstance string

// 		// Registration sub-element for parameter configuration.
// 		parameterConfig string

// 		// Managed conditions
// 		happyCond    string
// 		conditionSet []string

// 		//
// 		// Expected data fiels
// 		//

// 		// Only if rendering should return an error.
// 		expectedError *string

// 		// Environment variables for the rendered container.
// 		expectedEnvs []corev1.EnvVar
// 	}{
// 		"no parameter policies": {
// 			kuardInstance: kuardInstance,
// 			expectedEnvs: []corev1.EnvVar{
// 				{Name: "ARRAY", Value: "alpha,beta,gamma"},
// 				{Name: "GROUP_VARIABLE3", Value: "false"},
// 				{Name: "GROUP_VARIABLE4", Value: "42"},
// 				{Name: "VARIABLE1", Value: "value 1"},
// 				{Name: "VARIABLE2", Value: "value 2"},
// 			},
// 		},
// 		"skip variable from rendering": {
// 			kuardInstance: kuardInstance,
// 			parameterConfig: `
// fromSpec:
// - path: spec.variable2
//   skip: true
// `,
// 			expectedEnvs: []corev1.EnvVar{
// 				{Name: "ARRAY", Value: "alpha,beta,gamma"},
// 				{Name: "GROUP_VARIABLE3", Value: "false"},
// 				{Name: "GROUP_VARIABLE4", Value: "42"},
// 				{Name: "VARIABLE1", Value: "value 1"},
// 				/* {Name: "VARIABLE2", Value: "value 2"}, */
// 			},
// 		},
// 	}

// 	logr := tlogr.NewTestLogger(t)

// 	for name, tc := range testCases {
// 		t.Run(name, func(t *testing.T) {
// 			// for this test we can hardcode to deployment, we are only testing container output.
// 			wkl := &commonv1alpha1.Workload{
// 				FormFactor: &commonv1alpha1.FormFactor{
// 					Deployment: &commonv1alpha1.DeploymentFormFactor{
// 						Replicas: 1,
// 					},
// 				},
// 				ParameterConfiguration: &commonv1alpha1.ParameterConfiguration{},
// 			}

// 			// Read parameter configuration into structure
// 			err := yaml.Unmarshal([]byte(tc.parameterConfig), wkl.ParameterConfiguration)
// 			require.NoError(t, err)

// 			ctx := context.Background()

// 			cb := fake.NewClientBuilder()
// 			rsv := resolver.New(cb.WithObjects(tc.existingObjects...).Build())

// 			r := NewRenderer(wkl, rsv)

// 			smf := basestatus.NewStatusManagerFactory(crdv, tc.happyCond, tc.conditionSet, logr)
// 			mgr := baseobject.NewManager(gvk, r, smf)

// 			// Replace with the test object
// 			obj := mgr.NewObject()
// 			u := obj.AsKubeObject().(*unstructured.Unstructured)
// 			err = yaml.Unmarshal([]byte(tc.kuardInstance), u)
// 			require.NoError(t, err)

// 			err = r.Render(ctx, obj)
// 			if tc.expectedError != nil {
// 				require.Contains(t, err.Error(), *tc.expectedError)

// 			} else {
// 				require.NoError(t, err)
// 			}

// 			c := resources.NewContainer(
// 				"test-name",
// 				"test-image",
// 				obj.AsContainerOptions()...,
// 			)

// 			assert.Equal(t, tc.expectedEnvs, c.Env)
// 		})
// 	}
// }

func TestRenderedContainer(t *testing.T) {

	// Use the Kuard CRD for all cases
	crdv := basecrd.CRDPrioritizedVersion(ReadCRD(kuardCRD))

	testCases := map[string]struct {
		// Resolver related rendering might need existing objects. The
		// kuard instance used for reconciliation does not need to be
		// here, only any referenced object.
		existingObjects []client.Object

		// Kuard instance fir rendering.
		kuardInstance string

		// Registration sub-element for parameter configuration.
		parameterConfig string

		// Managed conditions
		happyCond    string
		conditionSet []string

		//
		// Expected data fiels
		//

		// Only if rendering should return an error.
		expectedError *string

		// Environment variables for the rendered container.
		expectedEnvs []corev1.EnvVar
	}{
		"no parameter policies": {
			kuardInstance: kuardInstance,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "VARIABLE1", Value: "value 1"},
				{Name: "VARIABLE2", Value: "value 2"},
			},
		},
		"skip variable from rendering": {
			kuardInstance: kuardInstance,
			parameterConfig: `
fromSpec:
  skip:
  - path: spec.variable2
`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "VARIABLE1", Value: "value 1"},
				/* {Name: "VARIABLE2", Value: "value 2"}, */
			},
		},
		"rename variable": {
			kuardInstance: kuardInstance,
			parameterConfig: `
fromSpec:
  toEnv:
  - path: spec.variable2
    name: KUARD_VARIABLE_TWO
`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "KUARD_VARIABLE_TWO", Value: "value 2"},
				{Name: "VARIABLE1", Value: "value 1"},
			},
		},
		"default value - when present": {
			kuardInstance: kuardInstance,
			parameterConfig: `
fromSpec:
  toEnv:
  - path: spec.variable2
    default:
      value: new variable2 value
`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "VARIABLE1", Value: "value 1"},
				{Name: "VARIABLE2", Value: "value 2"},
			},
		},
		"default value - when not present": {
			// remove variable2 entry from kuard instance.
			kuardInstance: strings.ReplaceAll(kuardInstance, "variable2: value 2", ""),
			parameterConfig: `
fromSpec:
  toEnv:
  - path: spec.variable2
    default:
      value: new variable2 value
`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "VARIABLE1", Value: "value 1"},
				{Name: "VARIABLE2", Value: "new variable2 value"},
			},
		},
	}

	_ = map[string]struct {
		// Resolver related rendering might need existing objects. The
		// kuard instance used for reconciliation does not need to be
		// here, only any referenced object.
		existingObjects []client.Object

		// Kuard instance fir rendering.
		kuardInstance string

		// Registration sub-element for parameter configuration.
		parameterConfig string

		// Managed conditions
		happyCond    string
		conditionSet []string

		//
		// Expected data fiels
		//

		// Only if rendering should return an error.
		expectedError *string

		// Environment variables for the rendered container.
		expectedEnvs []corev1.EnvVar
	}{
		"default value - when present": {
			kuardInstance: kuardInstance,
			parameterConfig: `
fromSpec:
- path: spec.variable2
  toEnv:
    defaultValue: new variable2 value
`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "VARIABLE1", Value: "value 1"},
				{Name: "VARIABLE2", Value: "value 2"},
			},
		},
		"default value - when not present": {
			// remove variable2 entry from kuard instance.
			kuardInstance: strings.ReplaceAll(kuardInstance, "variable2: value 2", ""),
			parameterConfig: `
fromSpec:
- path: spec.variable2
  toEnv:
    defaultValue: new variable2 value

`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "VARIABLE1", Value: "value 1"},
				{Name: "VARIABLE2", Value: "new variable2 value"},
			},
		},
		"secret reference": {
			// add secret reference to kuard base instance.
			kuardInstance: kuardInstance + `
  refToSecret:
    secretName: kuard-secret
    secretKey: kuard-key
`,
			parameterConfig: `
fromSpec:
- path: spec.refToSecret
  toEnv:
    name: FOO_CREDENTIALS
    valueFromSecret:
      name: spec.refToSecret.secretName
      key: spec.refToSecret.secretKey
`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "FOO_CREDENTIALS", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kuard-secret",
						},
						Key: "kuard-key",
					},
				}},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "VARIABLE1", Value: "value 1"},
				{Name: "VARIABLE2", Value: "value 2"},
			},
		},
		"configmap reference": {
			// add secret reference to kuard base instance.
			kuardInstance: kuardInstance + `
  refToConfigMap:
    configMapName: kuard-cm
    configMapKey: kuard-cm-key
`,
			parameterConfig: `
fromSpec:
- path: spec.refToConfigMap
  toEnv:
    name: FOO_CONFIG
    valueFromConfigMap:
      name: spec.refToConfigMap.configMapName
      key: spec.refToConfigMap.configMapKey
`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "FOO_CONFIG", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "kuard-cm",
						},
						Key: "kuard-cm-key",
					},
				}},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "VARIABLE1", Value: "value 1"},
				{Name: "VARIABLE2", Value: "value 2"},
			},
		},
		"add parameters": {
			kuardInstance: kuardInstance,
			parameterConfig: `
addEnvs:
- name: NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
- name: K_METRICS_CONFIG
  value: "{}"
- name: K_LOGGING_CONFIG
  value: "{}"
`,
			expectedEnvs: []corev1.EnvVar{
				{Name: "ARRAY", Value: "alpha,beta,gamma"},
				{Name: "GROUP_VARIABLE3", Value: "false"},
				{Name: "GROUP_VARIABLE4", Value: "42"},
				{Name: "K_LOGGING_CONFIG", Value: "{}"},
				{Name: "K_METRICS_CONFIG", Value: "{}"},
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				}},
				{Name: "VARIABLE1", Value: "value 1"},
				{Name: "VARIABLE2", Value: "value 2"},
			},
		},
	}

	logr := tlogr.NewTestLogger(t)

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// for this test we can hardcode to deployment, we are only testing container output.
			wkl := &commonv1alpha1.Workload{
				FormFactor: &commonv1alpha1.FormFactor{
					Deployment: &commonv1alpha1.DeploymentFormFactor{
						Replicas: 1,
					},
				},
				ParameterConfiguration: &commonv1alpha1.ParameterConfiguration{},
			}

			// Read parameter configuration into structure
			err := yaml.Unmarshal([]byte(tc.parameterConfig), wkl.ParameterConfiguration)
			require.NoError(t, err)

			ctx := context.Background()

			cb := fake.NewClientBuilder()
			client := cb.WithObjects(tc.existingObjects...).Build()
			rsv := resolver.New(client)
			cmr := configmap.NewNamespacedReader(tScobyNamespace, client)

			r, err := NewRenderer(wkl, rsv, cmr)
			assert.NoError(t, err, "error creating renderer")

			smf := basestatus.NewStatusManagerFactory(crdv, tc.happyCond, tc.conditionSet, logr)
			mgr := baseobject.NewManager(gvk, r, smf)

			// Replace with the test object
			obj := mgr.NewObject()
			u := obj.AsKubeObject().(*unstructured.Unstructured)
			err = yaml.Unmarshal([]byte(tc.kuardInstance), u)
			require.NoError(t, err)

			err = r.Render(ctx, obj)
			if tc.expectedError != nil {
				require.Contains(t, err.Error(), *tc.expectedError)

			} else {
				require.NoError(t, err)
			}

			c := resources.NewContainer(
				"test-name",
				"test-image",
				obj.AsContainerOptions()...,
			)

			assert.Equal(t, tc.expectedEnvs, c.Env)
		})
	}
}
