package component

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	gvk      schema.GroupVersionKind
	workload *common.Workload
	renderer render.Renderer

	client client.Client
	log    logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.Info("Reconciling request", "request", req)

	obj := r.newObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		return reconcile.Result{}, err
	}

	r.log.Info("Object read", "obj", obj)

	if err := r.renderer.EnsureCreated(obj); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectLogger(l logr.Logger) error {
	r.log = l.WithName("dynrecl")
	l.V(5).Info("logger injected into dynamic component reconciler")
	return nil
}

func (r *reconciler) newObject() client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   r.gvk.Group,
		Kind:    r.gvk.Kind,
		Version: r.gvk.Version,
	})
	return obj
}
