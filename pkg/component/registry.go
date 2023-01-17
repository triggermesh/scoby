package component

import (
	"sync"

	"github.com/go-logr/logr"
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ControllerRegistry keeps track of the controller created
// for each registered component.
type ControllerRegistry interface {
	/*
		TODO add configuration maybe.
	*/
	EnsureComponentController(crd *apiextensionsv1.CustomResourceDefinition, workload *common.Workload) error
	RemoveComponentController(crd *apiextensionsv1.CustomResourceDefinition) error
}

type controllerRegistry struct {
	// controllers keeps a map for GVR to dynamically created controllers.
	controllers map[schema.GroupVersionKind]*Controller

	lock sync.RWMutex
	mgr  manager.Manager

	logger *logr.Logger
}

// NewControllerRegistry creates a controller registry for registered components.
func NewControllerRegistry(mgr manager.Manager, logger *logr.Logger) ControllerRegistry {
	logger.Info("Creating new controller registry")

	return &controllerRegistry{
		controllers: make(map[schema.GroupVersionKind]*Controller),
		mgr:         mgr,
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

func (cr *controllerRegistry) EnsureComponentController(crd *apiextensionsv1.CustomResourceDefinition, workload *common.Workload) error {
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
	// ctrl, err := NewController(crd, ver, cr.mgr)
	ctrl, err := NewController(gvk, cr.mgr)
	if err != nil {
		return err
	}

	cr.controllers[gvk] = ctrl
	return nil
}

func (cr *controllerRegistry) RemoveComponentController(crd *apiextensionsv1.CustomResourceDefinition) error {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	gvk := crd.GroupVersionKind()
	ctl, found := cr.controllers[gvk]
	if !found {
		cr.logger.Info("Component Controller does not exists. Skipping removal", zap.Any("gvk", gvk))
		return nil
	}

	cr.logger.Info("Unloading component controller", zap.Any("gvk", gvk))
	ctl.Stop()
	delete(cr.controllers, gvk)

	return nil
}
