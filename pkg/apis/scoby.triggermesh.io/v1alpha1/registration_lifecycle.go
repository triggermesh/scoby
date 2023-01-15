// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
)

func (r *GenericRegistration) GetCRDNames() *apiextensionsv1.CustomResourceDefinitionNames {
	n := r.Spec.Generate.Names
	crdn := &apiextensionsv1.CustomResourceDefinitionNames{
		Singular: n.Singular,
		Plural:   n.Plural,
	}

	if n.Kind != nil {
		crdn.Kind = *n.Kind
	}

	if crdn.Kind == "" {
		titleCaser := cases.Title(language.English)
		crdn.Kind = titleCaser.String(crdn.Singular)
	}

	crdn.ListKind = crdn.Kind + "List"

	// if n.Singular != nil {
	// 	crdn.Singular = *n.Singular
	// }

	// switch {
	// case crdn.Kind == "" && crdn.Singular == "":
	// 	// If Kind and singular name are not informed use
	// 	// the registration object name.
	// 	name := r.GetName()
	// 	crdn.Singular = name[:strings.IndexByte(name, '.')]
	// 	crdn.Kind = titleCaser.String(crdn.Singular)

	// case crdn.Kind == "":
	// 	crdn.Kind = titleCaser.String(crdn.Singular)

	// case crdn.Singular == "":
	// 	crdn.Singular = strings.ToLower(crdn.Kind)
	// }

	// crdn.ListKind = crdn.Kind + "List"

	return crdn
}

// GetCRDDecoration returns the decoration elements for generated CRDs. Implements the Registration interface.
func (r *GenericRegistration) GetGenerateDecoration() *common.GenerateDecoration {
	return &r.Spec.Generate.GenerateDecoration
}

func (r *GenericRegistration) GetGenerateVersion() *common.GenerateVersion {
	return r.Spec.Generate.Version
}

func (r *GenericRegistration) GetWorkload() *common.Workload {
	return &r.Spec.Workload
}

func (r *GenericRegistration) GetConfiguration() *common.Configuration {
	return &r.Spec.Configuration
}
