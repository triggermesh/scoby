package crd

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/version"
)

func CRDPriotizedVersion(crd *apiextensionsv1.CustomResourceDefinition) *apiextensionsv1.CustomResourceDefinitionVersion {
	var crdv *apiextensionsv1.CustomResourceDefinitionVersion
	for _, v := range crd.Spec.Versions {
		if crdv == nil {
			crdv = &v
			continue
		}

		if version.CompareKubeAwareVersionStrings(v.Name, crdv.Name) > 0 {
			crdv = &v
		}
	}
	return crdv
}

type StatusFlag uint16

const (
	StatusFlagObservedGeneration = 1 << iota
	StatusFlagAnnotations
	StatusFlagConditionLastTranstitionTime
	StatusFlagConditionMessage
	StatusFlagConditionReason
	StatusFlagConditionStatus
	StatusFlagConditionType
)

func (sf StatusFlag) AllowConditions() bool {
	return sf&StatusFlagConditionLastTranstitionTime != 0 &&
		sf&StatusFlagConditionMessage != 0 &&
		sf&StatusFlagConditionReason != 0 &&
		sf&StatusFlagConditionStatus != 0 &&
		sf&StatusFlagConditionType != 0
}

func (sf StatusFlag) AllowAnnotations() bool {
	return sf&StatusFlagAnnotations != 0
}

func (sf StatusFlag) AllowObservedGeneration() bool {
	return sf&StatusFlagObservedGeneration != 0
}

func CRDStatusFlag(crdv *apiextensionsv1.CustomResourceDefinitionVersion) StatusFlag {
	var sf StatusFlag = 0
	status, ok := crdv.Schema.OpenAPIV3Schema.Properties["status"]
	if !ok {
		return sf
	}

	observedGeneration, ok := status.Properties["observedGeneration"]
	if ok && observedGeneration.Type == "integer" {
		sf |= StatusFlagObservedGeneration
	}

	annotations, ok := status.Properties["annotations"]
	if ok && annotations.Type == "object" &&
		annotations.AdditionalProperties != nil &&
		annotations.AdditionalProperties.Schema.Type == "string" {
		sf |= StatusFlagAnnotations
	}

	conditions, ok := status.Properties["conditions"]
	if !ok || conditions.Type != "array" {
		return sf
	}

	cprops := conditions.Items.Schema
	ltt, ok := cprops.Properties["lastTransitionTime"]
	if ok && ltt.Type == "string" {
		sf |= StatusFlagConditionLastTranstitionTime
	}

	message, ok := cprops.Properties["message"]
	if ok && message.Type == "string" {
		sf |= StatusFlagConditionMessage
	}

	reason, ok := cprops.Properties["reason"]
	if ok && reason.Type == "string" {
		sf |= StatusFlagConditionReason
	}

	cs, ok := cprops.Properties["status"]
	if ok && cs.Type == "string" {
		sf |= StatusFlagConditionStatus
	}

	t, ok := cprops.Properties["type"]
	if ok && t.Type == "string" {
		sf |= StatusFlagConditionType
	}

	return sf
}
