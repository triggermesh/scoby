// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	corev1 "k8s.io/api/core/v1"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/utils/configmap"
)

type specRenderer struct {
	// AllPaths keep track of all paths that contain instructions.
	allByPath map[string]struct{}

	// Skip element structured information.
	skipsByPath map[string]struct{}

	// Environment variables structured information.
	evDefaultValuesByPath   map[string]commonv1alpha1.SpecToEnvDefaultValue
	evNameByPath            map[string]string
	evConfigMapByPath       map[string]corev1.ConfigMapKeySelector
	evSecretByPath          map[string]corev1.SecretKeySelector
	evBuiltInFunctionByPath map[string]commonv1alpha1.BuiltInfunction

	// Volume mount structured information.
	volumeByPath map[string]commonv1alpha1.FromSpecToVolume
}

func newSpecRenderer(speccfg *commonv1alpha1.FromSpecConfiguration, cmr configmap.Reader) (*specRenderer, error) {
	sr := &specRenderer{
		skipsByPath: make(map[string]struct{}),

		evDefaultValuesByPath:   make(map[string]commonv1alpha1.SpecToEnvDefaultValue),
		evNameByPath:            make(map[string]string),
		evConfigMapByPath:       make(map[string]corev1.ConfigMapKeySelector),
		evSecretByPath:          make(map[string]corev1.SecretKeySelector),
		evBuiltInFunctionByPath: make(map[string]commonv1alpha1.BuiltInfunction),

		volumeByPath: make(map[string]commonv1alpha1.FromSpecToVolume),
	}

	if speccfg == nil {
		return sr, nil
	}

	for i := range speccfg.Skip {
		sr.skipsByPath[normalizePath(speccfg.Skip[i].Path)] = struct{}{}
	}

	for i := range speccfg.ToEnv {
		ev := speccfg.ToEnv[i]

		path := normalizePath(ev.Path)

		if ev.Default != nil {
			sr.evDefaultValuesByPath[path] = *ev.Default
		}

		if ev.Name != nil {
			sr.evNameByPath[path] = *ev.Name
		}

		if ev.ValueFrom == nil {
			continue
		}

		vf := ev.ValueFrom

		switch {
		case vf.ConfigMap != nil:
			sr.evConfigMapByPath[path] = *vf.ConfigMap
		case vf.Secret != nil:
			sr.evSecretByPath[path] = *vf.Secret
		case vf.BuiltInFunc != nil:
			sr.evBuiltInFunctionByPath[path] = *vf.BuiltInFunc
		}
	}

	for i := range speccfg.ToVolume {
		sr.volumeByPath[normalizePath(speccfg.ToVolume[i].Path)] = speccfg.ToVolume[i]
	}

	return sr, nil
}
