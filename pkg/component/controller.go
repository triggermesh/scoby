package component

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	scobyv1alpha1 "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/v1alpha1"
)

type Controller struct {
	// keep track of the CRD and the version inside the CRD
	// that is being used for this controller
	gvk schema.GroupVersionKind
	// crd    *apiextensionsv1.CustomResourceDefinition
	// crdVer *apiextensionsv1.CustomResourceDefinitionVersion
}

// func NewController(crd *apiextensionsv1.CustomResourceDefinition,  mgr manager.Manager) (*Controller, error) {
func NewController(gvk schema.GroupVersionKind, mgr manager.Manager) (*Controller, error) {

	c := &Controller{
		gvk: gvk,
	}
	// c.crd = crd.DeepCopy()

	// // pick top CRD version
	// for _, v := range crd.Spec.Versions {
	// 	if c.crdVer == nil {
	// 		c.crdVer = &v
	// 		continue
	// 	}

	// 	if version.CompareKubeAwareVersionStrings(v.Name, c.crdVer.Name) > 0 {
	// 		c.crdVer = &v
	// 	}
	// }

	logger := mgr.GetLogger()
	// crdForKind := unstructured.Unstructured{}
	// crdForKind.SetGroupVersionKind(c.crd.GroupVersionKind())

	// TODO
	ctx := context.Background()

	h := func(obj client.Object, q workqueue.RateLimitingInterface) {
		rl := &scobyv1alpha1.CRDRegistrationList{}
		if err := mgr.GetCache().List(ctx, rl, &client.ListOptions{}); err != nil {
			if !apierrs.IsNotFound(err) {
				logger.Error(err, "could not retrieve CRDRegistrationList")
			}
			return
		}

		for _, r := range rl.Items {
			if r.Spec.CRD == obj.GetName() {
				logger.Info("Enqueueing CRD for registration", "crd", obj, "reg", &r)
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      r.Name,
					Namespace: r.Namespace,
				}})
			}
		}
	}

	crdHandler := &handler.Funcs{
		CreateFunc:  func(e event.CreateEvent, q workqueue.RateLimitingInterface) { h(e.Object, q) },
		DeleteFunc:  func(e event.DeleteEvent, q workqueue.RateLimitingInterface) { h(e.Object, q) },
		UpdateFunc:  func(e event.UpdateEvent, q workqueue.RateLimitingInterface) { h(e.ObjectNew, q) },
		GenericFunc: func(e event.GenericEvent, q workqueue.RateLimitingInterface) { h(e.Object, q) },
	}

	if err := builder.ControllerManagedBy(mgr).
		For(c.newObject()).
		Watches(
			&source.Kind{Type: &apiextensionsv1.CustomResourceDefinition{}},
			crdHandler,
		).
		Complete(&reconciler{}); err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", gvk, err)
	}

	// TODO use context to cancel the controller.
	// depends on: https://github.com/kubernetes-sigs/controller-runtime/pull/2099

	return c, nil
}

func (c *Controller) Stop() {
	// TODO use context to cancel the controller.
	// depends on: https://github.com/kubernetes-sigs/controller-runtime/pull/2099

}

func (c *Controller) newObject() client.Object {
	obj := &unstructured.Unstructured{}

	//obj.SetGroupVersionKind(c.crd.GroupVersionKind())
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		// Group:   c.crd.Spec.Group,
		// Kind:    c.crd.Spec.Names.Kind,
		// Version: c.crdVer.Name,
		Group:   c.gvk.Group,
		Kind:    c.gvk.Kind,
		Version: c.gvk.Version,
	})
	return obj
}
