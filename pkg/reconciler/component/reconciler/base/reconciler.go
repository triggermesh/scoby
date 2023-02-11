// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package base

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"
	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
)

type Reconciler interface {
	// Registered element information
	RegisteredGetName() string
	RegisteredGetWorkload() *apicommon.Workload

	// Create new object using the GVK
	NewReconciledObject() ReconciledObject

	// Status management
	StatusConfigureManagerConditions(happy string, conditions ...string)
	// StatusGetSupportFlag() StatusFlag
}

func NewReconciler(crd *apiextensionsv1.CustomResourceDefinition, reg apicommon.Registration, psr PodSpecRenderer, log logr.Logger) Reconciler {
	// Choose CRD version
	crdv := CRDPrioritizedVersion(crd)

	// The status factory is created using only the ConditionTypeReady condition, it is up
	// to the base reconciler user to update with their set of conditions before using it.
	smf := NewStatusManagerFactory(crdv, ConditionTypeReady, []string{ConditionTypeReady}, log)

	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crdv.Name,
		Kind:    crd.Spec.Names.Kind,
	}

	rof := NewReconciledObjectFactory(gvk, smf, psr)

	return &reconciler{
		gvk: gvk,
		log: &log,
		reg: reg,
		psr: psr,
		smf: smf,
		rof: rof,
	}
}

type reconciler struct {
	// GVK for the registered CRD
	gvk schema.GroupVersionKind

	reg apicommon.Registration

	psr PodSpecRenderer

	// Status manager factory to create status managers per
	// reconciling object.
	smf StatusManagerFactory

	rof ReconciledObjectFactory

	log *logr.Logger
}

func (r *reconciler) NewReconciledObject() ReconciledObject {
	return r.rof.NewReconciledObject()
}

func (r *reconciler) RegisteredGetName() string {
	return r.reg.GetName()
}

func (r *reconciler) RegisteredGetWorkload() *apicommon.Workload {
	return r.reg.GetWorkload()
}

// ConfigureStatusManager with conditions
func (r *reconciler) StatusConfigureManagerConditions(happy string, conditions ...string) {
	r.smf.UpdateConditionSet(happy, conditions...)
}
