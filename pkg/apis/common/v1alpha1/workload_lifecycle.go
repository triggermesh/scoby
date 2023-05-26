// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

// Part of this package is inspired by Knative implementation at knative/dev/pkg.
package v1alpha1

import corev1 "k8s.io/api/core/v1"

func (dv *SpecToEnvDefaultValue) ToEnv(name string) *corev1.EnvVar {
	ev := &corev1.EnvVar{
		Name: name,
	}

	switch {
	case dv.Value != nil:
		ev.Value = *dv.Value

	case dv.ConfigMap != nil:
		ev.ValueFrom = &corev1.EnvVarSource{
			ConfigMapKeyRef: dv.ConfigMap,
		}

	case dv.Secret != nil:
		ev.ValueFrom = &corev1.EnvVarSource{
			SecretKeyRef: dv.Secret,
		}
	}

	return ev
}
