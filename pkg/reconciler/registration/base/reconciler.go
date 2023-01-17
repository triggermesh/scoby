// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package base

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

const (
	// SecretSelectorElement to be included at the CRD and instances.
	SecretSelectorElement = "secretKeyRef"
	// ConfigMapSelectorElement to be included at the CRD and instances.
	ConfigMapSelectorElement = "configMapKeyRef"
)

var (
	defaultVersion = common.GenerateVersion{
		Version: "v1alpha1",
		Served:  true,
		Storage: true,
	}
)

// type Reconciler interface {
// 	ReconcileCRD(ctx context.Context, registration common.Registration) (*apiextensionsv1.CustomResourceDefinition, error)
// }

type ReconcilerOpt func(*Reconciler)

func New(client client.Client, logger *logr.Logger, opts ...ReconcilerOpt) *Reconciler {
	r := &Reconciler{
		apiGroup: "",

		client: client,
		log:    logger,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// reconciler implements controller.Reconciler for the registration.
type Reconciler struct {
	// crdReconciler         *syncs.CRDReconciler
	// clusterRoleReconciler *syncs.ClusterRoleReconciler

	// registry component.ControllerRegistry
	apiGroup string

	log    *logr.Logger
	client client.Client
}

func ReconcilerWithAPIGroup(apiGroup string) ReconcilerOpt {
	return func(r *Reconciler) {
		r.apiGroup = apiGroup
	}
}

// ReconcileCRD for the registration.
func (r *Reconciler) ReconcileCRD(ctx context.Context, registration common.Registration) (*apiextensionsv1.CustomResourceDefinition, error) {
	r.log.V(1).Info("Reconciling CRD for registration", "registration", registration.GetObjectKind().GroupVersionKind())

	// Build CRD object
	desired, err := createCRDFromRegistration(registration)
	if err != nil {
		return nil, fmt.Errorf("could not create desired CRD: %w", err)
	}

	r.log.V(5).Info("Desired CRD for registration", "object", desired.String())

	existing := &apiextensionsv1.CustomResourceDefinition{}

	err = r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		// Compare
		// If same, that is ok
		// If not same, versioning is not supported, fail.

	case apierrs.IsNotFound(err):
		r.log.Info("Creating CRD", "object", desired)
		if err = r.client.Create(ctx, desired); err != nil {
			// TODO Propagate error to status
			return nil, fmt.Errorf("could not create CRD: %w", err)
		}

	default:
		return nil, fmt.Errorf("could not retrieving CRD %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	// crd := resources.BuildCRDFromRegistration(registration)
	// status := registration.GetStatusManager()

	// curcrd, event := r.crdReconciler.Reconcile(ctx, crd)
	// if curcrd == nil && event != nil {
	// 	status.MarkNoGeneratedCRDError(event)
	// 	// send error instead of event to log the error message.
	// 	return nil, fmt.Errorf(event.Error())
	// }

	// if ok := status.PropagateCRDAvailability(curcrd); !ok {
	// 	return nil, event
	// }

	// return curcrd, nil

	return desired, err
}

func createCRDFromRegistration(registration common.Registration) (*apiextensionsv1.CustomResourceDefinition, error) {
	v, err := createCRDVersionFromRegistration(registration)
	if err != nil {
		return nil, err
	}

	return resources.NewCRD(registration.GetName(),
		resources.CRDWithNames(registration.GetCRDNames()),
		resources.CRDAddVersion(v),
	)
}

func createCRDVersionFromRegistration(registration common.Registration) (*apiextensionsv1.CustomResourceDefinitionVersion, error) {
	spec, err := createCRDVersionSpecFromRegistration(registration)
	if err != nil {
		return nil, err
	}

	v := registration.GetGenerateVersion()
	if v == nil {
		v = &defaultVersion
	}

	return resources.NewCRDVersion(v.Version, v.Served, v.Storage, spec)
}

func createCRDVersionSpecFromRegistration(registration common.Registration) (*apiextensionsv1.JSONSchemaProps, error) {
	spec := &apiextensionsv1.JSONSchemaProps{
		Type:       "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{},
		Required:   make([]string, 0),
	}

	if opt := registration.GetWorkload().ParameterOptions; opt != nil && opt.ArbitraryParameters != nil && *opt.ArbitraryParameters {
		preserveFields := true
		spec.XPreserveUnknownFields = &preserveFields
	}

	// TODO for sources workloads add required K_SINK

	cfg := registration.GetConfiguration()
	if cfg == nil || len(cfg.Parameters) == 0 {
		return spec, nil
	}

	// TODO parse parameters when we get an alterantive to CRDs we are happy with.

	// for _, p := range cfg.Parameters {
	// 	// parse each parameter and add to spec
	// 	p, r := parseParameter(p)

	// }

	// spec.Properties =

	return spec, nil
}

// func parseParameter(p *common.Parameter) (*apiextensionsv1.JSONSchemaProps, *string /* required */) {
// 	props := &apiextensionsv1.JSONSchemaProps{}
// 	required := ""

// 	if p.Type != nil && *p.Type != "" {
// 		props.Type = *p.Type
// 	}

// 	// referenced value
// 	if p.ValueFrom != nil {
// 		switch p.ValueFrom.ReferenceType {
// 		case common.ReferenceTypeSecret:
// 			props.Type = "object"
// 			props.Properties = map[string]apiextensionsv1.JSONSchemaProps{
// 				SecretSelectorElement: {
// 					Type: "object",
// 					Properties: map[string]apiextensionsv1.JSONSchemaProps{
// 						"key": {
// 							Type: "string",
// 						},
// 						"name": {
// 							Type: "string",
// 						},
// 					},
// 				},
// 			}

// 		case common.ReferenceTypeConfigMap:
// 			props.Type = "object"
// 			props.Properties = map[string]apiextensionsv1.JSONSchemaProps{
// 				ConfigMapSelectorElement: {
// 					Type: "object",
// 					Properties: map[string]apiextensionsv1.JSONSchemaProps{
// 						"key": {
// 							Type: "string",
// 						},
// 						"name": {
// 							Type: "string",
// 						},
// 					},
// 				},
// 			}

// 		case common.ReferenceTypeDownward:
// 			// Downward is not reflected at the CRD but when
// 			// rendering the workload
// 			return nil, nil
// 		}
// 	}

// 	// configuration item is under .spec and not
// 	// nested in a section
// 	if p.Section == nil || *p.Section == "" {
// 		if p.Required != nil && *p.Required {
// 			required = p.Name
// 		}
// 		return props, &required
// 	}

// 	// // configuration item under .spec required
// 	// if p.Section == nil || *p.Section == "" {
// 	// 	if p.Required != nil && *p.Required {
// 	// 		required = append(required, p.Name)
// 	// 	}
// 	// 	params[p.Name] = param
// 	// 	continue
// 	// }

// 	// // get or create section under .spec
// 	// _, ok := params[*p.Section]
// 	// if !ok {
// 	// 	params[*p.Section] = apiextensionsv1.JSONSchemaProps{
// 	// 		Type:       "object",
// 	// 		Properties: map[string]apiextensionsv1.JSONSchemaProps{},
// 	// 	}
// 	// }

// 	// // add parameter under .spec.section
// 	// params[*p.Section].Properties[p.Name] = param
// 	// if p.Required != nil && *p.Required {
// 	// 	s := params[*p.Section]
// 	// 	s.Required = append(s.Required, p.Name)
// 	// 	params[*p.Section] = s

// 	// 	// if an element at the section is required, set the section to required.
// 	// 	// TODO: this can be improved letting users choose if a section is
// 	// 	// required indenpendently of the fields in that section.
// 	// 	sreq := false
// 	// 	for _, r := range required {
// 	// 		if r == *p.Section {
// 	// 			sreq = true
// 	// 			break
// 	// 		}
// 	// 	}
// 	// 	if !sreq {
// 	// 		required = append(required, *p.Section)
// 	// 	}
// 	// }

// 	return props
// }
