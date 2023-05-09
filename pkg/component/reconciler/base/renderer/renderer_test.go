package renderer

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	tlogr "github.com/go-logr/logr/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	basecrd "github.com/triggermesh/scoby/pkg/component/reconciler/base/crd"
	baseobject "github.com/triggermesh/scoby/pkg/component/reconciler/base/object"
	basestatus "github.com/triggermesh/scoby/pkg/component/reconciler/base/status"
	"github.com/triggermesh/scoby/pkg/utils/resolver"
	"github.com/triggermesh/scoby/pkg/utils/resources"

	. "github.com/triggermesh/scoby/test"
)

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

func TestRenderer(t *testing.T) {

	testCases := map[string]struct {
		existingObjects []client.Object
		wkl             *commonv1alpha1.Workload
		crd             *apiextensionsv1.CustomResourceDefinition
		happyCond       string
		conditionSet    []string
		log             logr.Logger

		// Conditions
		recObject *unstructured.Unstructured

		expectedError *string
	}{
		"nothing to render": {
			wkl: &commonv1alpha1.Workload{
				FormFactor: &commonv1alpha1.FormFactor{
					Deployment: &commonv1alpha1.DeploymentFormFactor{
						Replicas: 1,
					},
				},
			},
			crd: ReadCRD(kuardCRD),
		},
	}

	logr := tlogr.NewTestLogger(t)

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			cb := fake.NewClientBuilder()
			rsv := resolver.New(cb.WithObjects(tc.existingObjects...).Build())
			r := NewRenderer(tc.wkl, rsv)

			crdv := basecrd.CRDPrioritizedVersion(tc.crd)
			smf := basestatus.NewStatusManagerFactory(crdv, tc.happyCond, tc.conditionSet, logr)
			mgr := baseobject.NewManager(gvk, r, smf)

			// replace with the test object
			obj := mgr.NewObject()
			u := obj.AsKubeObject().(*unstructured.Unstructured)
			err := yaml.Unmarshal([]byte(kuardInstance), u)
			require.NoError(t, err)

			// *u = *tc.recObject

			err = r.Render(ctx, obj)
			if tc.expectedError != nil {
				assert.Contains(t, err.Error(), *tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			c := resources.NewContainer(
				"test-name",
				"test-image",
				obj.AsContainerOptions()...,
			)

			t.Logf("container: %+v", c)
		})
	}

}
