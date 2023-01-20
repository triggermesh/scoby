package component

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render"
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

	objs, err := r.renderer.RenderControlledObjects(obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, desired := range objs {
		r.log.V(1).Info("rendered desired object", "object", desired)

		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(desired.GetObjectKind().GroupVersionKind())
		err := r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)

		switch {
		case err == nil:
			// Compare
			// If same, that is ok
			// If not same, versioning is not supported, fail.

		case apierrs.IsNotFound(err):
			r.log.Info("Creating CRD", "object", desired)
			if err = r.client.Create(ctx, desired); err != nil {
				// TODO Propagate error to status
				return reconcile.Result{}, fmt.Errorf("could not create controlled object: %w", err)
			}

		default:
			return reconcile.Result{}, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
		}

		// update status

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
