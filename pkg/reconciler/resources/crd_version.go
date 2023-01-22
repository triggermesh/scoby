// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	// "knative.dev/pkg/kmeta"
)

type CRDVersionOption func(*apiextensionsv1.CustomResourceDefinitionVersion) error

// name must be formatted as <plural-resource>.<group>.
func NewCRDVersion(version string, served, storage bool, spec *apiextensionsv1.JSONSchemaProps, opts ...CRDVersionOption) (*apiextensionsv1.CustomResourceDefinitionVersion, error) {

	preserveFields := true
	properties := map[string]apiextensionsv1.JSONSchemaProps{
		"spec": *spec,
		"status": {
			Type:                   "object",
			XPreserveUnknownFields: &preserveFields,
		},
	}

	v := &apiextensionsv1.CustomResourceDefinitionVersion{
		Name:    version,
		Served:  served,
		Storage: storage,

		Schema: &apiextensionsv1.CustomResourceValidation{
			OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
				Type:       "object",
				Properties: properties,
			},
		},

		Subresources: &apiextensionsv1.CustomResourceSubresources{
			Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
		},

		AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
			{
				Name:     "Ready",
				Type:     "string",
				JSONPath: ".status.conditions[?(@.type=='Ready')].status",
			},
			{
				Name:     "Reason",
				Type:     "string",
				JSONPath: ".status.conditions[?(@.type=='Ready')].reason",
			},
			{
				Name:     "Age",
				Type:     "date",
				JSONPath: ".metadata.creationTimestamp",
			},
		},
	}

	var err error
	for _, opt := range opts {
		if err = opt(v); err != nil {
			return nil, err
		}
	}

	return v, err
}
