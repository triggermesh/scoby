// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"sync"

	"github.com/go-logr/logr"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	creconciler "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
)

// ComponentRegistry keeps track of the controllers created
// for each registered component.
type ComponentRegistry interface {
	EnsureComponentController(reg common.Registration, crd *apiextensionsv1.CustomResourceDefinition) error
	RemoveComponentController(reg common.Registration)
}

type entry struct {
	//reconciler reconciler.ComponentReconciler
	reconciler reconcile.Reconciler
	cancel     context.CancelFunc
}

type componentRegisty struct {
	// controllers keeps a map of dynamically created controllers
	// for registrations.
	controllers map[string]*entry

	lock    sync.RWMutex
	mgr     manager.Manager
	context context.Context
	logger  *logr.Logger
}

// New creates a controller registry for registered components.
func New(ctx context.Context, mgr manager.Manager, logger *logr.Logger) ComponentRegistry {
	logger.Info("Creating new controller registry")

	return &componentRegisty{
		controllers: make(map[string]*entry),
		mgr:         mgr,
		context:     ctx,
		logger:      logger,
	}
}

func (cr *componentRegisty) EnsureComponentController(reg common.Registration, crd *apiextensionsv1.CustomResourceDefinition) error {
	cr.logger.V(1).Info("EnsureComponentController", "crd", crd.Name)

	cr.lock.Lock()
	defer cr.lock.Unlock()

	_, found := cr.controllers[reg.GetName()]
	if found {
		return nil
	}

	cr.logger.Info("Creating component controller for CRD", "name", crd.Name)

	ctx, cancel := context.WithCancel(cr.context)
	r, err := creconciler.NewReconciler(ctx, crd, reg, cr.mgr)
	if err != nil {
		cancel()
		return err
	}

	cr.controllers[reg.GetName()] = &entry{
		reconciler: r,
		cancel:     cancel,
	}
	return nil
}

func (cr *componentRegisty) RemoveComponentController(reg common.Registration) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	rn := reg.GetName()
	if entry, found := cr.controllers[rn]; found {
		cr.logger.Info("Unloading component controller", "registration", rn)
		// TODO use context to cancel the controller.
		// depends on: https://github.com/kubernetes-sigs/controller-runtime/pull/2159

		entry.cancel()
		delete(cr.controllers, rn)
	} else {
		cr.logger.Info("Component Controller does not exists. Skipping removal", "registration", rn)
		return
	}
}

type ReconcilerBuilder func(ctx context.Context, gvk schema.GroupVersionKind, reg common.Registration, mgr manager.Manager) reconcile.Reconciler
