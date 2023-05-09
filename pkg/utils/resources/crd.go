// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "knative.dev/pkg/kmeta"
)

// default categories
var defaultCategories = []string{"all"}

type CRDOption func(*apiextensionsv1.CustomResourceDefinition) error

// name must be formatted as <plural-resource>.<group>.
func NewCRD(name string, opts ...CRDOption) (*apiextensionsv1.CustomResourceDefinition, error) {
	meta := NewMeta("", name)

	i := strings.IndexByte(name, byte('.'))
	if i <= 0 {
		return nil, fmt.Errorf("CRD name %q must be formatted <resource-name-plural>.<group>", name)
	}

	plural := name[:i]
	group := name[i+1:]

	crd := &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: *meta,
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:     plural,
				Categories: defaultCategories,
			},
			Group: group,
			Scope: apiextensionsv1.NamespaceScoped,
		},
	}

	var err error
	for _, opt := range opts {
		if err = opt(crd); err != nil {
			return nil, err
		}
	}

	return crd, err
}

func CRDWithMetaOptions(opts ...MetaOption) CRDOption {
	return func(crd *apiextensionsv1.CustomResourceDefinition) error {
		for _, opt := range opts {
			opt(&crd.ObjectMeta)
		}
		return nil
	}
}

// Plural name should not be informed. In case it is it should be set according to
// the name and group of the CRD.
func CRDWithNames(crdn *apiextensionsv1.CustomResourceDefinitionNames) CRDOption {
	return func(crd *apiextensionsv1.CustomResourceDefinition) error {
		// CRD name must match <plural>.<group>. We made sure that
		// the name was properly set when the CRD was created. If the
		// plural value is not provided we keep the existing one.
		prevPlural := crd.Spec.Names.Plural
		crd.Spec.Names = *crdn

		if crdn.Plural == "" {
			crd.Spec.Names.Plural = prevPlural
			return nil
		}

		return validateCRDnaming(crd)
	}
}

func validateCRDnaming(crd *apiextensionsv1.CustomResourceDefinition) error {
	i := strings.IndexByte(crd.Name, byte('.'))
	if i <= 0 {
		return fmt.Errorf("CRD name %q must be formatted <resource-name-plural>.<group>", crd.Name)
	}

	plural := crd.Name[:i]

	if crd.Spec.Names.Plural != plural {
		return fmt.Errorf("CRD name %q must be begin with the .spec.names.plural value: %s", crd.Name, crd.Spec.Names.Plural)
	}

	group := crd.Name[i+1:]
	if crd.Spec.Group != group {
		return fmt.Errorf("CRD name %q must be end with the .spec.names.plural value: %s", crd.Name, crd.Spec.Group)
	}

	return nil
}

func CRDAddVersion(v *apiextensionsv1.CustomResourceDefinitionVersion) CRDOption {
	return func(crd *apiextensionsv1.CustomResourceDefinition) error {
		// TODO make sure there is no conflicting with other versions and that only
		// one version has storage set.
		crd.Spec.Versions = append(crd.Spec.Versions, *v)
		return nil
	}
}
