// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"go.uber.org/zap"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
)

// ComponentRegistry keeps track of the controllers created
// for each registered component.
type ComponentRegistry interface {
	EnsureComponentController(crd *apiextensionsv1.CustomResourceDefinition, reg common.Registration) error
	RemoveComponentController(crd *apiextensionsv1.CustomResourceDefinition) error
}

type entry struct {
	reconciler reconciler.ComponentReconciler
	cancel     context.CancelFunc
}

type componentRegisty struct {
	// controllers keeps a map for GVR to dynamically created controllers.
	controllers map[schema.GroupVersionKind]*entry

	lock    sync.RWMutex
	mgr     manager.Manager
	context context.Context
	logger  *logr.Logger
}

// New creates a controller registry for registered components.
func New(ctx context.Context, mgr manager.Manager, logger *logr.Logger) ComponentRegistry {
	logger.Info("Creating new controller registry")

	return &componentRegisty{
		controllers: make(map[schema.GroupVersionKind]*entry),
		mgr:         mgr,
		context:     ctx,
		logger:      logger,
	}
}

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

func (cr *componentRegisty) EnsureComponentController(crd *apiextensionsv1.CustomResourceDefinition, reg common.Registration) error {
	cr.logger.V(1).Info("EnsureComponentController", "crd", crd.Name)
	ver := CRDPriotizedVersion(crd)
	cr.lock.Lock()
	defer cr.lock.Unlock()

	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: ver.Name,
		Kind:    crd.Spec.Names.Kind,
	}

	_, found := cr.controllers[gvk]
	if found {
		return nil
	}

	cr.logger.Info("Creating component controller for CRD", "name", crd.Name)

	ctx, cancel := context.WithCancel(cr.context)
	r, err := reconciler.NewComponentReconciler(ctx, gvk, reg, cr.mgr)
	if err != nil {
		cancel()
		return err
	}

	cr.controllers[gvk] = &entry{
		reconciler: r,
		cancel:     cancel,
	}
	return nil
}

func (cr *componentRegisty) RemoveComponentController(crd *apiextensionsv1.CustomResourceDefinition) error {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	gvk := crd.GroupVersionKind()
	entry, found := cr.controllers[gvk]
	if !found {
		cr.logger.Info("Component Controller does not exists. Skipping removal", zap.Any("gvk", gvk))
		return nil
	}

	cr.logger.Info("Unloading component controller", zap.Any("gvk", gvk))

	// TODO use context to cancel the controller.
	// depends on: https://github.com/kubernetes-sigs/controller-runtime/pull/2099

	entry.cancel()
	delete(cr.controllers, gvk)

	return nil
}
