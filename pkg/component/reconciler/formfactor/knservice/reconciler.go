// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package knservice

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"knative.dev/networking/pkg/apis/networking"
	"knative.dev/serving/pkg/apis/autoscaling"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"github.com/triggermesh/scoby/pkg/component/reconciler"
	"github.com/triggermesh/scoby/pkg/utils/resources"
	"github.com/triggermesh/scoby/pkg/utils/semantic"
)

const (
	ConditionTypeKnativeServiceReady = "KnativeServiceReady"

	ConditionReasonKnativeServiceReady   = "KNSERVICEOK"
	ConditionReasonKnativeServiceUnknown = "KNSERVICEUNKOWN"
)

func New(name string, wkl *commonv1alpha1.Workload, client client.Client, log logr.Logger) reconciler.FormFactorReconciler {

	sr := &knserviceReconciler{
		name:       name,
		formFactor: wkl.FormFactor.KnativeService,
		fromImage:  &wkl.FromImage,

		client: client,
		log:    log,
	}

	return sr
}

type knserviceReconciler struct {
	name       string
	formFactor *commonv1alpha1.KnativeServiceFormFactor
	fromImage  *commonv1alpha1.RegistrationFromImage

	client client.Client
	log    logr.Logger
}

var _ reconciler.FormFactorReconciler = (*knserviceReconciler)(nil)

func (sr *knserviceReconciler) GetStatusConditions() (happy string, all []string) {
	happy = reconciler.ConditionTypeReady
	all = []string{ConditionTypeKnativeServiceReady}

	return
}

func (sr *knserviceReconciler) SetupController(name string, c controller.Controller, owner runtime.Object) error {
	if err := c.Watch(&source.Kind{Type: resources.NewKnativeService("", "")}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    owner}); err != nil {
		return fmt.Errorf("could not set watcher on knative services owned by registered object %q: %w", name, err)
	}

	return nil
}

func (dr *knserviceReconciler) InitializeStatus(obj reconciler.Object) {
	// Make sure Deployment and Status conditions exist set
	sm := obj.GetStatusManager()

	if sm.GetCondition(ConditionTypeKnativeServiceReady) == nil {
		sm.SetCondition(&commonv1alpha1.Condition{
			Type:               ConditionTypeKnativeServiceReady,
			Status:             metav1.ConditionUnknown,
			LastTransitionTime: metav1.Now(),
			Reason:             ConditionReasonKnativeServiceUnknown,
		})
	}

}

func (sr *knserviceReconciler) Reconcile(ctx context.Context, obj reconciler.Object) (ctrl.Result, error) {
	sr.log.V(1).Info("reconciling object instance", "object", obj)

	ksvc, err := sr.reconcileKnativeService(ctx, obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	sr.updateKnativeServiceStatus(obj, ksvc)

	return reconcile.Result{}, nil
}

func (sr *knserviceReconciler) reconcileKnativeService(ctx context.Context, obj reconciler.Object) (*servingv1.Service, error) {
	sr.log.V(1).Info("reconciling knative service", "object", obj)

	// render service
	desired, err := sr.createKnServiceFromRegistered(obj)
	if err != nil {
		return nil, fmt.Errorf("could not render knative service object: %w", err)
	}

	sr.log.V(5).Info("desired knative service object", "object", *desired)

	existing := &servingv1.Service{}
	err = sr.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		if semantic.Semantic.DeepEqual(desired, existing) {
			return existing, nil
		}

		sr.log.Info("rendered knative service does not match the expected object", "object", desired)
		sr.log.V(5).Info("mismatched knative service", "desired", *desired, "existing", *existing)

		// resourceVersion must be returned to the API server unmodified for
		// optimistic concurrency, as per Kubernetes API conventions
		desired.SetResourceVersion(existing.GetResourceVersion())

		if err = sr.client.Update(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not update knative service object: %+w", err)
		}

	case apierrs.IsNotFound(err):
		sr.log.Info("creating knative service", "object", desired)
		sr.log.V(5).Info("desired knative service", "object", *desired)
		if err = sr.client.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not create knative service object: %w", err)
		}

	default:
		return nil, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	return desired, nil
}

func (sr *knserviceReconciler) updateKnativeServiceStatus(obj reconciler.Object, ksvc *servingv1.Service) {
	sr.log.V(1).Info("updating knativeService status", "object", obj)

	desired := &commonv1alpha1.Condition{
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

	sm := obj.GetStatusManager()
	sm.SetAddressURL(address)
	sm.SetCondition(desired)
}

func (sr *knserviceReconciler) createKnServiceFromRegistered(obj reconciler.Object) (*servingv1.Service, error) {
	metaopts := []resources.MetaOption{
		resources.MetaAddLabel(resources.AppNameLabel, sr.name),
		resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
		resources.MetaAddLabel(resources.AppComponentLabel, reconciler.ComponentWorkload),
		resources.MetaAddLabel(resources.AppPartOfLabel, reconciler.PartOf),
		resources.MetaAddLabel(resources.AppManagedByLabel, reconciler.ManagedBy),

		resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
	}

	revspecopts := []resources.RevisionTemplateOption{
		resources.RevisionSpecWithPodSpecOptions(
			resources.PodSpecAddContainer(
				resources.NewContainer(
					reconciler.DefaultContainerName,
					sr.fromImage.Repo,
					obj.AsContainerOptions()...,
				))),
	}

	if sr.formFactor != nil {
		if sr.formFactor.Visibility != nil {
			metaopts = append(metaopts, resources.MetaAddLabel(networking.VisibilityLabelKey, *sr.formFactor.Visibility))
		}

		if sr.formFactor.MinScale != nil {
			revspecopts = append(revspecopts, resources.RevisionWithMetaOptions(
				resources.MetaAddAnnotation(autoscaling.MinScaleAnnotationKey, strconv.Itoa(*sr.formFactor.MinScale))))
		}

		if sr.formFactor.MaxScale != nil {
			revspecopts = append(revspecopts, resources.RevisionWithMetaOptions(
				resources.MetaAddAnnotation(autoscaling.MaxScaleAnnotationKey, strconv.Itoa(*sr.formFactor.MaxScale))))
		}
	}

	return resources.NewKnativeService(obj.GetNamespace(), obj.GetName(),
		resources.KnativeServiceWithMetaOptions(metaopts...),
		resources.KnativeServiceWithRevisionOptions(revspecopts...)), nil
}
