// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package crd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestCRDPriotizedVersion(t *testing.T) {
	testCases := map[string]struct {
		in     *apiextensionsv1.CustomResourceDefinition
		schema map[string]apiextensionsv1.JSONSchemaProps
	}{
		"one version": {
			in: readCRD(`
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
      schema:
        openAPIV3Schema:
          type: object
          properties:
            testA:
              type: string`),
			schema: map[string]apiextensionsv1.JSONSchemaProps{
				"testA": {
					Type: "string",
				},
			},
		},

		"two versions": {
			in: readCRD(`
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
    - name: v1beta1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            testB:
              type: string
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            testA:
              type: string`),
			schema: map[string]apiextensionsv1.JSONSchemaProps{
				"testA": {
					Type: "string",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := CRDPrioritizedVersion(tc.in)
			assert.Equal(t, tc.schema, got.Schema.OpenAPIV3Schema.Properties)
		})
	}
}

func TestCRDStatus(t *testing.T) {
	testCases := map[string]struct {
		in                      *apiextensionsv1.CustomResourceDefinition
		allowConditions         bool
		allowAnnotations        bool
		allowObservedGeneration bool
	}{
		"full status": {
			in: readCRD(`
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
            status:
              description: CRDRegistrationStatus defines the observed state of CRDRegistration
              properties:
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
                    - lastTransitionTime
                    - message
                    - reason
                    - status
                    - type
                    type: object
                  type: array
                observedGeneration:
                  description: ObservedGeneration is the 'Generation' of the Object
                    that was last processed by the controller.
                  format: int64
                  type: integer
              type: object`),
			allowConditions:         true,
			allowObservedGeneration: true,
			allowAnnotations:        true,
		},
		"full status but no subresource": {
			in: readCRD(`
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
      schema:
        openAPIV3Schema:
          type: object
          properties:
            status:
              description: CRDRegistrationStatus defines the observed state of CRDRegistration
              properties:
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
                    - lastTransitionTime
                    - message
                    - reason
                    - status
                    - type
                    type: object
                  type: array
                observedGeneration:
                  description: ObservedGeneration is the 'Generation' of the Object
                    that was last processed by the controller.
                  format: int64
                  type: integer
              type: object`),
			allowConditions:         false,
			allowObservedGeneration: false,
			allowAnnotations:        false,
		},
		"conditions only": {
			in: readCRD(`
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
            status:
              description: CRDRegistrationStatus defines the observed state of CRDRegistration
              properties:
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
                    - lastTransitionTime
                    - message
                    - reason
                    - status
                    - type
                    type: object
                  type: array
              type: object`),
			allowConditions:         true,
			allowObservedGeneration: false,
			allowAnnotations:        false,
		},
		"no status": {
			in: readCRD(`
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
          type: object`),
			allowConditions:         false,
			allowObservedGeneration: false,
			allowAnnotations:        false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sf := CRDStatusFlag(CRDPrioritizedVersion(tc.in))

			t.Logf("value is %+v", sf)
			assert.Equal(t, tc.allowConditions, sf.AllowConditions(), "unexpected status conditions support")
			assert.Equal(t, tc.allowAnnotations, sf.AllowAnnotations(), "unexpected status annotations support")
			assert.Equal(t, tc.allowObservedGeneration, sf.AllowObservedGeneration(), "unexpected status observedgeneration support")
		})
	}
}

func readCRD(crd string) *apiextensionsv1.CustomResourceDefinition {
	sch := runtime.NewScheme()

	err := scheme.AddToScheme(sch)
	if err != nil {
		panic(err)
	}

	err = apiextensionsv1.AddToScheme(sch)
	if err != nil {
		panic(err)
	}

	obj, _, err := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode([]byte(crd), nil, nil)
	if err != nil {
		panic(err)
	}

	return obj.(*apiextensionsv1.CustomResourceDefinition)
}
