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

	// metav1.ObjectMeta{
	// 	Name:            name,
	// 	OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(cr)},
	// 	Labels:          decoration.Labels,
	// 	Annotations:     decoration.Annotations,
	// },

	for _, opt := range opts {
		opt(crd)
	}

	return crd, nil
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

// // BuildCRDFromRegistration creates a component CRD from
// // registration data.
// func BuildCRDFromRegistration(cr v1alpha1.Registration) *apiextensionsv1.CustomResourceDefinition {
// 	name, group, singular, plural, kind := MakeComponentNames(cr)
// 	decoration := cr.GetCRDDecoration()

// 	return &apiextensionsv1.CustomResourceDefinition{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: name,
// 			// OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(cr)},
// 			Labels:      decoration.Labels,
// 			Annotations: decoration.Annotations,
// 		},
// 		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
// 			Names: apiextensionsv1.CustomResourceDefinitionNames{
// 				Kind:       kind,
// 				Categories: defaultCategories,
// 				Singular:   singular,
// 				Plural:     plural,
// 			},
// 			Group: group,
// 			Scope: apiextensionsv1.NamespaceScoped,
// 			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
// 				*makeVersion(cr),
// 			},
// 		},
// 	}
// }

// // MakeComponentNames creates the component CRD names based on registration data.
// func MakeComponentNames(cr v1alpha1.Registration) (name, group, singular, plural, kind string) {
// 	names := cr.GetCRDNaming()
// 	if names.Kind != nil {
// 		kind = *names.Kind
// 	}
// 	if names.Singular != nil {
// 		singular = *names.Singular
// 	}

// 	if kind == "" && singular == "" {
// 		singular = strings.ToLower(cr.GetName())
// 		kind = strings.Title(cr.GetName())
// 	}

// 	if singular == "" {
// 		singular = strings.ToLower(kind)
// 	}

// 	if kind == "" {
// 		kind = strings.Title(cr.GetName())
// 	}

// 	plural = names.Plural

// 	return plural + "." + defaultComponentGroup, defaultComponentGroup, singular, plural, kind
// }

// // makeVersion creates the main CRD spec version.
// func makeVersion(cr v1alpha1.Registration) *apiextensionsv1.CustomResourceDefinitionVersion {
// 	properties := map[string]apiextensionsv1.JSONSchemaProps{
// 		"spec":   makeSpecSchemaProps(cr),
// 		"status": makeStatusSchemaProps(),
// 	}

// 	return &apiextensionsv1.CustomResourceDefinitionVersion{
// 		Name:    defaultVersion,
// 		Served:  true,
// 		Storage: true,
// 		Subresources: &apiextensionsv1.CustomResourceSubresources{
// 			Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
// 		},
// 		Schema: &apiextensionsv1.CustomResourceValidation{
// 			OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
// 				Type:       "object",
// 				Properties: properties,
// 			},
// 		},
// 		AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
// 			{
// 				Name:     "Ready",
// 				Type:     "string",
// 				JSONPath: ".status.conditions[?(@.type=='Ready')].status",
// 			},
// 			{
// 				Name:     "Reason",
// 				Type:     "string",
// 				JSONPath: ".status.conditions[?(@.type=='Ready')].reason",
// 			},
// 			{
// 				Name:     "Age",
// 				Type:     "date",
// 				JSONPath: ".metadata.creationTimestamp",
// 			},
// 		},
// 	}
// }

// func makeSpecSchemaProps(cr v1alpha1.Registration) apiextensionsv1.JSONSchemaProps {
// 	spec := apiextensionsv1.JSONSchemaProps{
// 		Type: "object",
// 	}

// 	if opt := cr.GetWorkload().ParameterOptions; opt != nil && opt.ArbitraryParameters != nil && *opt.ArbitraryParameters {
// 		preserveFields := true
// 		spec.XPreserveUnknownFields = &preserveFields
// 	}

// 	props, req := makeConfigurationProps(cr)

// 	if cr.IsKnativeSource() {
// 		props[sinkAddressableAttribute] = makeSinkSchemaProps()
// 		spec.Required = []string{sinkAddressableAttribute}
// 	}

// 	spec.Properties = props
// 	spec.Required = append(spec.Required, req...)

// 	return spec
// }

// func makeConfigurationProps(cr v1alpha1.Registration) (map[string]apiextensionsv1.JSONSchemaProps, []string) {
// 	params := map[string]apiextensionsv1.JSONSchemaProps{}
// 	required := make([]string, 0)

// 	cfg := cr.GetConfiguration()
// 	if cfg == nil || len(cfg.Parameters) == 0 {
// 		return params, required
// 	}

// 	for _, p := range cfg.Parameters {

// 		// direct value
// 		param := apiextensionsv1.JSONSchemaProps{}
// 		if p.Type != nil && *p.Type != "" {
// 			param.Type = *p.Type
// 		}

// 		// referenced value
// 		if p.ValueFrom != nil {
// 			switch p.ValueFrom.ReferenceType {
// 			case v1alpha1.ReferenceTypeSecret:
// 				param.Type = "object"
// 				param.Properties = map[string]apiextensionsv1.JSONSchemaProps{
// 					v1alpha1.SecretSelectorElement: {
// 						Type: "object",
// 						Properties: map[string]apiextensionsv1.JSONSchemaProps{
// 							"key": {
// 								Type: "string",
// 							},
// 							"name": {
// 								Type: "string",
// 							},
// 						},
// 					},
// 				}

// 			case v1alpha1.ReferenceTypeConfigMap:
// 				param.Type = "object"
// 				param.Properties = map[string]apiextensionsv1.JSONSchemaProps{
// 					v1alpha1.ConfigMapSelectorElement: {
// 						Type: "object",
// 						Properties: map[string]apiextensionsv1.JSONSchemaProps{
// 							"key": {
// 								Type: "string",
// 							},
// 							"name": {
// 								Type: "string",
// 							},
// 						},
// 					},
// 				}

// 			case v1alpha1.ReferenceTypeDownward:
// 				// Downward is not reflected at the CRD but when
// 				// rendering the workload
// 				continue
// 			}
// 		}

// 		// configuration item under .spec required
// 		if p.Section == nil || *p.Section == "" {
// 			if p.Required != nil && *p.Required {
// 				required = append(required, p.Name)
// 			}
// 			params[p.Name] = param
// 			continue
// 		}

// 		// get or create section under .spec
// 		_, ok := params[*p.Section]
// 		if !ok {
// 			params[*p.Section] = apiextensionsv1.JSONSchemaProps{
// 				Type:       "object",
// 				Properties: map[string]apiextensionsv1.JSONSchemaProps{},
// 			}
// 		}

// 		// add parameter under .spec.section
// 		params[*p.Section].Properties[p.Name] = param
// 		if p.Required != nil && *p.Required {
// 			s := params[*p.Section]
// 			s.Required = append(s.Required, p.Name)
// 			params[*p.Section] = s

// 			// if an element at the section is required, set the section to required.
// 			// TODO: this can be improved letting users choose if a section is
// 			// required indenpendently of the fields in that section.
// 			sreq := false
// 			for _, r := range required {
// 				if r == *p.Section {
// 					sreq = true
// 					break
// 				}
// 			}
// 			if !sreq {
// 				required = append(required, *p.Section)
// 			}
// 		}
// 	}

// 	return params, required
// }

// func makeStatusSchemaProps() apiextensionsv1.JSONSchemaProps {
// 	preserveFields := true
// 	return apiextensionsv1.JSONSchemaProps{
// 		Type:                   "object",
// 		XPreserveUnknownFields: &preserveFields,
// 	}
// }

// func makeSinkSchemaProps() apiextensionsv1.JSONSchemaProps {
// 	return apiextensionsv1.JSONSchemaProps{
// 		Type: "object",
// 		Properties: map[string]apiextensionsv1.JSONSchemaProps{
// 			"ref": {
// 				Description: "Reference of an Addressable object acting as event sink.",
// 				Type:        "object",
// 				Properties: map[string]apiextensionsv1.JSONSchemaProps{
// 					"apiVersion": {
// 						Description: "API version of the referent.",
// 						Type:        "string",
// 					},
// 					"kind": {
// 						Description: "Kind of the referent.",
// 						Type:        "string",
// 					},
// 					"namespace": {
// 						Description: "Namespace of the referent.",
// 						Type:        "string",
// 					},
// 					"name": {
// 						Description: "Name of the referent",
// 						Type:        "string",
// 					},
// 				},
// 				Required: []string{"apiVersion", "kind", "name"},
// 			},
// 			"uri": {
// 				Description: "URI of the event sink.",
// 				Type:        "string",
// 				Format:      "uri",
// 			},
// 		},
// 		OneOf: []apiextensionsv1.JSONSchemaProps{
// 			{Required: []string{"ref"}},
// 			{Required: []string{"uri"}},
// 		},
// 	}
// }
