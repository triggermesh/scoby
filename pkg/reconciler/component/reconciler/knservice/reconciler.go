// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package knativeservice

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"

	"knative.dev/networking/pkg/apis/networking"
	"knative.dev/serving/pkg/apis/autoscaling"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	rcrd "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/crd"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/render"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

type ComponentReconciler interface {
	reconcile.Reconciler
	NewObject() client.Object
}

func NewComponentReconciler(ctx context.Context, crd *rcrd.Registered, reg common.Registration, mgr manager.Manager) (ComponentReconciler, error) {
	gvk := crd.GetGVK()
	log := mgr.GetLogger().WithName(gvk.String())

	r := &reconciler{
		log:          log,
		crd:          crd,
		registration: reg,
		psr:          render.NewPodSpecRenderer("adapter", reg.GetWorkload().FromImage.Repo),
	}

	if err := builder.ControllerManagedBy(mgr).
		For(r.NewObject()).
		Owns(resources.NewKnativeService("", "")).
		Complete(r); err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", gvk.String(), err)
	}

	return r, nil
}

type reconciler struct {
	crd          *rcrd.Registered
	registration common.Registration
	psr          render.PodSpecRenderer

	client client.Client
	log    logr.Logger
}

var _ ComponentReconciler = (*reconciler)(nil)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.V(1).Info("reconciling request", "request", req)

	obj := r.NewObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		// Return and let the ownership clean
		// owned resources.
		return reconcile.Result{}, nil
	}

	// render service
	desired, err := r.createKnServiceFrom(obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	r.log.V(5).Info("desired knative service object", "object", *desired)

	existing := &servingv1.Service{}
	err = r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		if !semantic.Semantic.DeepEqual(desired, existing) {
			r.log.Info("rendered knative service does not match the expected object", "object", desired)
			r.log.V(5).Info("mismatched knative service", "desired", *desired, "existing", *existing)

			// resourceVersion must be returned to the API server unmodified for
			// optimistic concurrency, as per Kubernetes API conventions
			desired.SetResourceVersion(existing.GetResourceVersion())

			if err = r.client.Update(ctx, desired); err != nil {
				return reconcile.Result{
					Requeue: false,
				}, fmt.Errorf("could not update knative service at reconciliation: %+w", err)
			}
			return reconcile.Result{}, nil
		}

	case apierrs.IsNotFound(err):
		r.log.Info("creating knative service", "object", desired)
		r.log.V(5).Info("desired knative service", "object", *desired)
		if err = r.client.Create(ctx, desired); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not create knative service object: %w", err)
		}

	default:
		return reconcile.Result{}, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	// update status

	return reconcile.Result{}, nil
}

func (r *reconciler) createKnServiceFrom(obj client.Object) (*servingv1.Service, error) {
	// TODO generate names

	ps, err := r.psr.Render(obj)
	if err != nil {
		return nil, err
	}

	ff := r.registration.GetWorkload().FormFactor.KnativeService
	metaopts := []resources.MetaOption{
		resources.MetaAddLabel(resources.AppNameLabel, r.registration.GetName()),
		resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
		resources.MetaAddLabel(resources.AppComponentLabel, render.ComponentWorkload),
		resources.MetaAddLabel(resources.AppPartOfLabel, render.PartOf),
		resources.MetaAddLabel(resources.AppManagedByLabel, render.ManagedBy),

		resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
	}

	revspecopts := []resources.RevisionTemplateOption{
		resources.RevisionSpecWithPodSpecOptions(ps...),
	}

	if ff.Visibility != nil {
		metaopts = append(metaopts, resources.MetaAddLabel(networking.VisibilityLabelKey, *ff.Visibility))
	}

	if ff.MinScale != nil {
		revspecopts = append(revspecopts, resources.RevisionWithMetaOptions(
			resources.MetaAddAnnotation(autoscaling.MinScaleAnnotationKey, strconv.Itoa(*ff.MinScale))))
	}

	if ff.MaxScale != nil {
		revspecopts = append(revspecopts, resources.RevisionWithMetaOptions(
			resources.MetaAddAnnotation(autoscaling.MaxScaleAnnotationKey, strconv.Itoa(*ff.MaxScale))))
	}

	return resources.NewKnativeService(obj.GetNamespace(), obj.GetName(),
		resources.KnativeServiceWithMetaOptions(metaopts...),
		resources.KnativeServiceWithRevisionOptions(revspecopts...)), nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectLogger(l logr.Logger) error {
	r.log = l.WithName("dynrecl")
	l.V(2).Info("logger injected into dynamic component reconciler")
	return nil
}

func (r *reconciler) NewObject() client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(r.crd.GetGVK())
	return obj
}
