// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package base

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"
	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	basecrd "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/crd"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/object"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/status"
)

// The base reconciler is created using data from the registration
// and exposes methods common to all reconciliations, no matter the
// form factor.
type Reconciler interface {
	// Registered element information
	RegisteredGetName() string
	RegisteredGetWorkload() *apicommon.Workload

	// Create new object using the GVK
	NewReconcilingObject() ReconcilingObject

	// Render a Reconciling into data that can be
	// used at reconciliation.
	RenderReconciling(ReconcilingObject) (RenderedObject, error)

	// Status management
	StatusConfigureManagerConditions(happy string, conditions ...string)
}

func NewReconciler(crd *apiextensionsv1.CustomResourceDefinition, reg apicommon.Registration, renderer object.Renderer, log logr.Logger) Reconciler {
	// Choose CRD version
	crdv := basecrd.CRDPrioritizedVersion(crd)

	// The status factory is created using only the ConditionTypeReady condition, it is up
	// to the base reconciler user to update with their set of conditions before using it.
	smf := status.NewStatusManagerFactory(crdv, ConditionTypeReady, []string{ConditionTypeReady}, log)

	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: crdv.Name,
		Kind:    crd.Spec.Names.Kind,
	}

	rof := object.NewReconcilingObjectFactory(gvk, smf, renderer)

	return &reconciler{
		gvk:      gvk,
		log:      &log,
		reg:      reg,
		renderer: renderer,
		smf:      smf,
		rof:      rof,
	}
}

type reconciler struct {
	// GVK for the registered CRD
	gvk schema.GroupVersionKind

	reg apicommon.Registration

	renderer object.Renderer

	// Status manager factory to create status managers per
	// reconciling object.
	smf status.StatusManagerFactory

	rof object.ReconcilingObjectFactory

	log *logr.Logger
}

// NewReconcilingObject creates a new empty reference of a reconciling
// object that can be used by a kubernetes client to be filled with an
// existing instance of the object at the cluster.
func (r *reconciler) NewReconcilingObject() ReconcilingObject {
	return r.rof.NewReconcilingObject()
}

// RenderReconciling uses the incoming reconciling object to process its data
// and turn it into structures that can be used to render dependent objects.
func (r *reconciler) RenderReconciling(obj ReconcilingObject) (RenderedObject, error) {
	return r.renderer.Render(obj)
}

// Return the registration name.
func (r *reconciler) RegisteredGetName() string {
	return r.reg.GetName()
}

// Return the registration workload structure
func (r *reconciler) RegisteredGetWorkload() *apicommon.Workload {
	return r.reg.GetWorkload()
}

// ConfigureStatusManager with conditions
func (r *reconciler) StatusConfigureManagerConditions(happy string, conditions ...string) {
	r.smf.UpdateConditionSet(happy, conditions...)
}
