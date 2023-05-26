// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/utils/configmap"
)

const addEnvsPrefix = "$added."

type addRenderer struct {
	// pre-parsed environment variables that should be
	// added to workloads without further processing.
	envVarsByPath map[string]*corev1.EnvVar

	// add instructions for environment variables that need
	// to be parsed when rendering.
	processEnvsByPath map[string]commonv1alpha1.AddToEnvConfiguration
	cmr               configmap.Reader
}

func newAddRenderer(addcfg *commonv1alpha1.AddConfiguration, cmr configmap.Reader) (*addRenderer, error) {
	ar := &addRenderer{
		envVarsByPath:     make(map[string]*corev1.EnvVar),
		processEnvsByPath: make(map[string]commonv1alpha1.AddToEnvConfiguration),
		cmr:               cmr,
	}

	if len(addcfg.ToVolume) != 0 {
		return nil, fmt.Errorf("registration workload contains instructions to add volumes, which are not implemented")
	}

	// pre-parse environment variables instructions
	for i := range addcfg.ToEnv {
		ev := addcfg.ToEnv[i]

		// There is no path for added envrionment variables, but
		// we want to keep consistency, so we also add them here
		// using a prefix plus the variable name.
		pseudoPath := addEnvsPrefix + ev.Name

		switch {
		case ev.Value != nil:
			ar.envVarsByPath[pseudoPath] = &corev1.EnvVar{
				Name:  ev.Name,
				Value: *ev.Value,
			}

		case ev.ValueFrom != nil:
			ar.envVarsByPath[pseudoPath] = &corev1.EnvVar{
				Name:      ev.Name,
				ValueFrom: ev.ValueFrom.ToEnvVarSource(),
			}

		case ev.ValueFromControllerConfigMap != nil:
			ar.processEnvsByPath[pseudoPath] = ev
		}
	}

	return ar, nil
}

// renderEnvVars returns the list of environment variables to be added to the workload indexed
// by pseudo-json path.
//
// Pseudo-json path is provided to potentially allow other fields to perform calculations based
// on either the resulting environment variable name, or the registration information (which is the
// basis for the pseudo-json path).
func (aer *addRenderer) renderEnvVars(ctx context.Context) (map[string]*corev1.EnvVar, error) {
	r := make(map[string]*corev1.EnvVar, len(aer.envVarsByPath)+len(aer.processEnvsByPath))

	// Add all environment variables that do not need processing.
	for k := range aer.envVarsByPath {
		r[k] = aer.envVarsByPath[k]
	}

	// Add all enviroment variables that need processing.
	// We only support for this case reading a ConfigMap at the Scoby controller namespace
	// and writting the contents to the environment variable.
	for k, aec := range aer.processEnvsByPath {
		v, err := aer.cmr.Read(ctx,
			aec.ValueFromControllerConfigMap.Name,
			aec.ValueFromControllerConfigMap.Key)
		if err != nil {
			return nil, fmt.Errorf("could not read configmap to fill environment variable %q: %w", aec.Name, err)
		}

		// Add ConfigMap contents as the environment variable value.
		r[k] = &corev1.EnvVar{
			Name:  aec.Name,
			Value: *v,
		}
	}

	return r, nil
}
