// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package knativeservice

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"knative.dev/networking/pkg/apis/networking"
	"knative.dev/serving/pkg/apis/autoscaling"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	recbase "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

const (
	ConditionTypeKnativeServiceReady = "KnativeServiceReady"

	ConditionReasonKnativeServiceReady   = "KNSERVICEOK"
	ConditionReasonKnativeServiceUnknown = "KNSERVICEUNKOWN"
)

func NewComponentReconciler(ctx context.Context, base recbase.Reconciler, mgr manager.Manager) (chan error, error) {
	log := mgr.GetLogger().WithName(base.RegisteredGetName())
	log.Info("Creating knative serving styled reconciler", "registration", base.RegisteredGetName())

	r := &reconciler{
		log:  log,
		base: base,
	}

	base.StatusConfigureManagerConditions(recbase.ConditionTypeReady, ConditionTypeKnativeServiceReady)

	log.V(1).Info("Reconciler configured, adding to controller manager", "registration", base.RegisteredGetName())

	ctrl, err := controller.NewUnmanaged("my", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", base.RegisteredGetName(), err)
	}

	obj := base.NewReconcilingObject().AsKubeObject()
	if err := ctrl.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, fmt.Errorf("could not set watcher on registered object %q: %w", base.RegisteredGetName(), err)
	}

	if err := ctrl.Watch(&source.Kind{Type: resources.NewKnativeService("", "")}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    obj}); err != nil {
		return nil, fmt.Errorf("could not set watcher on knative services owned by registered object %q: %w", base.RegisteredGetName(), err)
	}

	stCh := make(chan error)
	go func() {
		stCh <- ctrl.Start(ctx)
	}()

	return stCh, nil
}

type reconciler struct {
	base recbase.Reconciler

	client client.Client
	log    logr.Logger
}

var _ reconcile.Reconciler = (*reconciler)(nil)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.log.V(1).Info("reconciling request", "request", req)

	obj := r.base.NewReconcilingObject()
	if err := r.client.Get(ctx, req.NamespacedName, obj.AsKubeObject()); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	r.log.V(5).Info("Object retrieved", "object", obj)

	if !obj.GetDeletionTimestamp().IsZero() {
		// Return and let the ownership clean resources.
		return reconcile.Result{}, nil
	}

	// Perform the generic object rendering
	ro, err := r.base.RenderReconciling(ctx, obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	// create a copy, we will compare after reconciling
	// and decide if we need to update or not.
	cp := obj.AsKubeObject().DeepCopyObject()

	res, err := r.reconcileObjectInstance(ctx, obj, ro)

	// Update status if needed.
	// TODO find a better expression for this
	if !semantic.Semantic.DeepEqual(
		obj.AsKubeObject().(*unstructured.Unstructured).Object["status"],
		cp.(*unstructured.Unstructured).Object["status"]) {
		if uperr := r.client.Status().Update(ctx, obj.AsKubeObject()); uperr != nil {
			if err == nil {
				return reconcile.Result{}, uperr
			}
			r.log.Error(uperr, "could not update the object status")
		}
	}

	return res, err
}

func (r *reconciler) reconcileObjectInstance(ctx context.Context, obj recbase.ReconcilingObject, ro recbase.RenderedObject) (reconcile.Result, error) {
	r.log.V(1).Info("reconciling object instance", "object", obj)

	// Update generation if needed
	if g := obj.GetGeneration(); g != obj.StatusGetObservedGeneration() {
		r.log.V(1).Info("updating observed generation", "generation", g)
		obj.StatusSetObservedGeneration(g)
	}

	ksvc, err := r.reconcileKnativeService(ctx, obj, ro)
	if err != nil {
		return reconcile.Result{}, err
	}

	r.updateKnativeServiceStatus(obj, ksvc)

	return reconcile.Result{}, nil
}

func (r *reconciler) reconcileKnativeService(ctx context.Context, obj recbase.ReconcilingObject, ro recbase.RenderedObject) (*servingv1.Service, error) {
	r.log.V(1).Info("reconciling knative service", "object", obj)

	// render service
	desired, err := r.createKnServiceFromRegistered(obj, ro)
	if err != nil {
		return nil, fmt.Errorf("could not render knative service object: %w", err)
	}

	r.log.V(5).Info("desired knative service object", "object", *desired)

	existing := &servingv1.Service{}
	err = r.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		if semantic.Semantic.DeepEqual(desired, existing) {
			return existing, nil
		}

		r.log.Info("rendered knative service does not match the expected object", "object", desired)
		r.log.V(5).Info("mismatched knative service", "desired", *desired, "existing", *existing)

		// resourceVersion must be returned to the API server unmodified for
		// optimistic concurrency, as per Kubernetes API conventions
		desired.SetResourceVersion(existing.GetResourceVersion())

		if err = r.client.Update(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not update knative service object: %+w", err)
		}

	case apierrs.IsNotFound(err):
		r.log.Info("creating knative service", "object", desired)
		r.log.V(5).Info("desired knative service", "object", *desired)
		if err = r.client.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not create knative service object: %w", err)
		}

	default:
		return nil, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	return desired, nil
}

func (r *reconciler) updateKnativeServiceStatus(obj recbase.ReconcilingObject, ksvc *servingv1.Service) {
	r.log.V(1).Info("updating knativeService status", "object", obj)

	desired := &apicommon.Condition{
		Type:               ConditionTypeKnativeServiceReady,
		Reason:             ConditionReasonKnativeServiceUnknown,
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
	}

	address := ""
	if ksvc != nil {
		for _, c := range ksvc.Status.Conditions {
			if c.Type != servingv1.ServiceConditionReady {
				continue
			}
			switch c.Status {
			case corev1.ConditionTrue:
				desired.Status = metav1.ConditionTrue
				desired.Reason = ConditionReasonKnativeServiceReady

			case corev1.ConditionFalse:
				desired.Status = metav1.ConditionFalse
				desired.Reason = c.Reason
				desired.Message = c.Message
			default:
				desired.Message = fmt.Sprintf(
					"%q condition for knative service contains an unexpected status: %s",
					c.Type, c.Status)
			}
			break
		}

		if ksvc.Status.Address != nil {
			address = ksvc.Status.Address.URL.String()
		}
	}

	obj.StatusSetAddressURL(address)
	obj.StatusSetCondition(desired)
}

func (r *reconciler) createKnServiceFromRegistered(obj recbase.ReconcilingObject, ro recbase.RenderedObject) (*servingv1.Service, error) {
	metaopts := []resources.MetaOption{
		resources.MetaAddLabel(resources.AppNameLabel, r.base.RegisteredGetName()),
		resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
		resources.MetaAddLabel(resources.AppComponentLabel, recbase.ComponentWorkload),
		resources.MetaAddLabel(resources.AppPartOfLabel, recbase.PartOf),
		resources.MetaAddLabel(resources.AppManagedByLabel, recbase.ManagedBy),

		resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
	}

	revspecopts := []resources.RevisionTemplateOption{
		resources.RevisionSpecWithPodSpecOptions(ro.GetPodSpecOptions()...),
	}

	wkl := r.base.RegisteredGetWorkload()
	if ff := wkl.FormFactor.KnativeService; ff != nil {
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
	}

	return resources.NewKnativeService(obj.GetNamespace(), obj.GetName(),
		resources.KnativeServiceWithMetaOptions(metaopts...),
		resources.KnativeServiceWithRevisionOptions(revspecopts...)), nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
