// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/builder"
)

const (
	registryGracefulTimeout = 5 * time.Second
)

// ComponentRegistry keeps track of the controllers created
// for each registered component.
type ComponentRegistry interface {
	EnsureComponentController(reg commonv1alpha1.Registration, crd *apiextensionsv1.CustomResourceDefinition) error
	RemoveComponentController(reg commonv1alpha1.Registration)
	WaitStopChannel() <-chan error
}

type entry struct {
	reconcilerCh chan error
	cancel       context.CancelFunc
}

type componentRegistry struct {
	// controllers keeps a map of dynamically created controllers
	// for registrations.
	controllers map[string]*entry

	lock    sync.RWMutex
	mgr     manager.Manager
	context context.Context
	logger  *logr.Logger

	closing bool
	stoCh   chan error
}

// New creates a controller registry for registered components.
func New(ctx context.Context, mgr manager.Manager, logger *logr.Logger) ComponentRegistry {
	logger.Info("Creating new controller registry")

	cr := &componentRegistry{
		controllers: make(map[string]*entry),
		mgr:         mgr,
		context:     ctx,
		logger:      logger,

		stoCh:   make(chan error),
		closing: false,
	}

	// Setup graceful shutdown routine.
	go func() {
		<-ctx.Done()

		cr.lock.Lock()
		defer cr.lock.Unlock()
		cr.closing = true

		errs := []string{}
		wg := sync.WaitGroup{}
		for k := range cr.controllers {
			c := cr.controllers[k]
			name := k
			wg.Add(1)

			go func() {
				defer wg.Done()
				c.cancel()

				select {
				case err := <-c.reconcilerCh:
					if err != nil {
						errs = append(errs, fmt.Sprintf("%s: %v", name, err))
					}
				case <-time.After(registryGracefulTimeout):
					errs = append(errs, fmt.Sprintf("%s: stop timed out", name))
				}
			}()
		}

		wg.Wait()
		if len(errs) != 0 {
			msg := strings.Join(errs, ". ")
			cr.stoCh <- fmt.Errorf(msg[:len(msg)-2])
		}
		close(cr.stoCh)
	}()

	return cr
}

func (cr *componentRegistry) EnsureComponentController(reg commonv1alpha1.Registration, crd *apiextensionsv1.CustomResourceDefinition) error {
	cr.logger.V(1).Info("EnsureComponentController", "crd", crd.Name)

	cr.lock.Lock()
	defer cr.lock.Unlock()

	if cr.closing {
		return fmt.Errorf("component registry is closing")
	}

	_, found := cr.controllers[reg.GetName()]
	if found {
		return nil
	}

	cr.logger.Info("Creating component controller for CRD", "name", crd.Name)

	ctx, cancel := context.WithCancel(cr.context)
	rch, err := builder.NewReconciler(ctx, crd, reg, cr.mgr)
	if err != nil {
		cancel()
		return err
	}

	cr.controllers[reg.GetName()] = &entry{
		reconcilerCh: rch,
		cancel:       cancel,
	}
	return nil
}

func (cr *componentRegistry) RemoveComponentController(reg commonv1alpha1.Registration) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	if cr.closing {
		// the closing procedure will remove all controllers,
		// no need to do it here.
		return
	}

	rn := reg.GetName()
	if entry, found := cr.controllers[rn]; found {
		cr.logger.Info("Unloading component controller", "registration", rn)
		// TODO remove also the underlying informers.
		// depends on: https://github.com/kubernetes-sigs/controller-runtime/pull/2159

		var err error
		entry.cancel()
		select {
		case err = <-entry.reconcilerCh:
			if err != nil {
				cr.logger.Error(err, "controller stop returned an error", "controller", rn)
			}
		case <-time.After(registryGracefulTimeout):
			cr.logger.Error(errors.New("controller stop timed out"), "controller", rn)
		}

		delete(cr.controllers, rn)
	} else {
		cr.logger.Info("Component Controller does not exists. Skipping removal", "registration", rn)
		return
	}
}

func (cr *componentRegistry) WaitStopChannel() <-chan error {
	return cr.stoCh
}
