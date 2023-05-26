// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
	"github.com/triggermesh/scoby/pkg/utils/resources"
)

var (
	securityContext = resources.NewSecurityContext(
		resources.SecurityContextWithPrivilegeEscalation(false),
		resources.SecurityContextWithReadOnlyRootFilesystem(true),
	)

	defaultContainerOpts = []resources.ContainerOption{
		resources.ContainerWithTerminationMessagePolicy(corev1.TerminationMessageFallbackToLogsOnError),
		resources.ContainerWithSecurityContext(securityContext),
	}
)

type object struct {
	// Kubernetes object.
	*unstructured.Unstructured

	// Status manager customized for the object instance.
	statusManager reconciler.StatusManager

	// Environment variables to be added to the workload,
	// mapped by their JSON path and Name.
	//
	// These values are stored to be able to use them
	// for calculations.
	evsByPath map[string]*corev1.EnvVar
	evsByName map[string]*corev1.EnvVar

	vmByPath map[string]*commonv1alpha1.FromSpecToVolume
	vmByName map[string]*commonv1alpha1.FromSpecToVolume
}

var _ reconciler.Object = (*object)(nil)

func (o object) AsKubeObject() client.Object {
	return o.Unstructured
}

func (o object) GetStatusManager() reconciler.StatusManager {
	return o.statusManager
}

// Object Renderer methods

func (o object) AddEnvVar(fromPath string, ev *corev1.EnvVar) {
	o.evsByPath[fromPath] = ev
	o.evsByName[ev.Name] = ev
}

func (o object) AddVolumeMount(fromPath string, vm *commonv1alpha1.FromSpecToVolume) {
	o.vmByPath[fromPath] = vm
	o.vmByName[vm.Name] = vm
}

func (o object) AsContainerOptions() []resources.ContainerOption {
	envNames := make([]string, 0, len(o.evsByName))
	for k := range o.evsByName {
		envNames = append(envNames, k)
	}
	sort.Strings(envNames)

	// Initialize array of options and add default set.
	copts := make([]resources.ContainerOption, 0, len(o.evsByName)+len(defaultContainerOpts))
	copts = append(copts, defaultContainerOpts...)

	for _, k := range envNames {
		ev := o.evsByName[k]
		copts = append(copts, resources.ContainerAddEnv(ev))
	}

	return copts
}

func (o object) GetEnvVarAtPath(path string) *corev1.EnvVar {
	return o.evsByPath[path]
}
