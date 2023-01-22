// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/render"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

const defaultReplicas = 1

type ComponentReconciler interface {
	reconcile.Reconciler
	NewObject() client.Object
}

func NewComponentReconciler(ctx context.Context, gvk schema.GroupVersionKind, reg common.Registration, mgr manager.Manager) (ComponentReconciler, error) {
	log := mgr.GetLogger().WithName(gvk.GroupKind().String())

	r := &reconciler{
		log:          log,
		gvk:          gvk,
		registration: reg,
		psr:          render.NewPodSpecRenderer("adapter", reg.GetWorkload().FromImage.Repo),
	}

	if err := builder.ControllerManagedBy(mgr).
		For(r.NewObject()).
		Owns(resources.NewDeployment("", "")).
		Owns(resources.NewService("", "")).
		Complete(r); err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", gvk, err)
	}

	return r, nil
}

type reconciler struct {
	gvk          schema.GroupVersionKind
	registration common.Registration
	psr          render.PodSpecRenderer

	client client.Client
	log    logr.Logger
}

var _ ComponentReconciler = (*reconciler)(nil)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.Info("Reconciling request", "request", req)

	obj := r.NewObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		return reconcile.Result{}, err
	}

	r.log.Info("Object read", "obj", obj)

	// render deployment
	desired, err := r.createDeploymentFrom(obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	// sync deployment

	r.log.V(1).Info("rendered desired object", "object", desired)

	existing := &appsv1.Deployment{}
	err = r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		if !semantic.Semantic.DeepEqual(desired, existing) {
			r.log.Info("rendered deployment does not match the expected object", "object", desired)
			// delete, the next reconciliation will create the deployment

			r.log.V(5).Info("mismatched deployment", "desired", *desired, "existing", *existing)

			// resourceVersion must be returned to the API server unmodified for
			// optimistic concurrency, as per Kubernetes API conventions
			desired.SetResourceVersion(existing.GetResourceVersion())

			// adapter, err := cg(currentAdapter.GetNamespace()).Update(ctx, desiredAdapter, metav1.UpdateOptions{})
			// if err != nil {
			// 	var t T
			// 	return t, reconciler.NewEvent(corev1.EventTypeWarning, ReasonFailedAdapterUpdate,
			// 		"Failed to update adapter %s %q: %s", gvk.Kind, currentAdapter.GetName(), err)
			// }
			// event.Normal(ctx, ReasonAdapterUpdate, "Updated adapter %s %q", gvk.Kind, adapter.GetName())

			if err = r.client.Update(ctx, desired); err != nil {
				return reconcile.Result{
					Requeue: false,
				}, fmt.Errorf("could not update deployment at reconciliation: %+w", err)
			}
			return reconcile.Result{}, nil
		}

	case apierrs.IsNotFound(err):
		r.log.Info("Creating deployment", "object", desired)
		if err = r.client.Create(ctx, desired); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not create controlled object: %w", err)
		}

	default:
		return reconcile.Result{}, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	// update status

	return reconcile.Result{}, nil
}

func (r *reconciler) createDeploymentFrom(obj client.Object) (*appsv1.Deployment, error) {
	// TODO generate names

	// use parameter options to define parameters policy
	// use obj to gather
	ps, err := r.psr.Render(obj)
	if err != nil {
		return nil, err
	}

	replicas := defaultReplicas
	if ffd := r.registration.GetWorkload().FormFactor.Deployment; ffd != nil {
		replicas = ffd.Replicas
	}

	return resources.NewDeployment(obj.GetNamespace(), obj.GetName(),
		resources.DeploymentWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, r.registration.GetName()),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, render.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, render.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, render.ManagedBy),

			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.DeploymentSetReplicas(int32(replicas)),
		resources.DeploymentAddSelectorForTemplate(resources.AppNameLabel, r.registration.GetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppInstanceLabel, obj.GetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppComponentLabel, render.ComponentWorkload),

		resources.DeploymentWithTemplateSpecOptions(
			resources.PodTemplateSpecWithPodSpecOptions(ps...),
		)), nil
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

func (r *reconciler) NewObject() client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   r.gvk.Group,
		Kind:    r.gvk.Kind,
		Version: r.gvk.Version,
	})
	return obj
}
