// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
)

type KnativeServiceOption func(*servingv1.Service)

func NewKnativeService(namespace, name string, opts ...KnativeServiceOption) *servingv1.Service {
	meta := NewMeta(namespace, name)

	s := &servingv1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: servingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: *meta,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func KnativeServiceWithMetaOptions(opts ...MetaOption) KnativeServiceOption {
	return func(s *servingv1.Service) {
		for _, opt := range opts {
			opt(&s.ObjectMeta)
		}
	}
}

type RevisionTemplateOption func(*servingv1.RevisionTemplateSpec)

func KnativeServiceWithRevisionOptions(opts ...RevisionTemplateOption) KnativeServiceOption {
	return func(s *servingv1.Service) {
		for _, opt := range opts {
			opt(&s.Spec.Template)
		}
	}
}

func RevisionWithMetaOptions(opts ...MetaOption) RevisionTemplateOption {
	return func(rts *servingv1.RevisionTemplateSpec) {
		for _, opt := range opts {
			opt(&rts.ObjectMeta)
		}
	}
}

func RevisionSpecWithPodSpecOptions(opts ...PodSpecOption) RevisionTemplateOption {
	return func(rts *servingv1.RevisionTemplateSpec) {
		for _, opt := range opts {
			opt(&rts.Spec.PodSpec)
		}
	}
}
