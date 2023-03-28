// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
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

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/resolver"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

const (
	defaultReplicas = 1

	ConditionTypeDeploymentReady = "DeploymentReady"
	ConditionTypeServiceReady    = "ServiceReady"
)

func New(name string, wkl *apicommon.Workload, client client.Client, log logr.Logger) reconciler.FormFactorReconciler {
	dr := &deploymentReconciler{
		name:       name,
		formFactor: wkl.FormFactor.Deployment,
		fromImage:  &wkl.FromImage,

		client: client,
		log:    log,
	}

	if dr.formFactor != nil && dr.formFactor.Service != nil {
		dr.serviceOptions = dr.formFactor.Service
	}

	return dr
}

type deploymentReconciler struct {
	name           string
	formFactor     *apicommon.DeploymentFormFactor
	fromImage      *apicommon.RegistrationFromImage
	serviceOptions *apicommon.DeploymentService

	client client.Client
	log    logr.Logger
}

var _ reconciler.FormFactorReconciler = (*deploymentReconciler)(nil)

func (dr *deploymentReconciler) GetStatusConditions() (happy string, all []string) {
	happy = reconciler.ConditionTypeReady
	all = []string{ConditionTypeDeploymentReady}

	// If a service associated to the deployment add the
	// status condition.
	if dr.formFactor != nil && dr.formFactor.Service != nil {
		all = append(all, ConditionTypeServiceReady)
	}

	return
}

func (dr *deploymentReconciler) SetupController(name string, c controller.Controller, owner runtime.Object) error {
	dr.log.Info("Setting up deployment styled reconciler", "registration", name)
	if err := c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    owner}); err != nil {
		return fmt.Errorf("could not set watcher on deployments owned by registered object %q: %w", name, err)
	}

	if err := c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    owner}); err != nil {
		return fmt.Errorf("could not set watcher on services owned by registered object %q: %w", name, err)
	}

	return nil
}
func (dr *deploymentReconciler) Reconcile(ctx context.Context, obj reconciler.Object) (ctrl.Result, error) {
	dr.log.V(1).Info("reconciling object instance", "object", obj)

	// Update generation if needed
	if g := obj.GetGeneration(); g != obj.GetStatusManager().GetObservedGeneration() {
		dr.log.V(1).Info("updating observed generation", "generation", g)
		obj.GetStatusManager().SetObservedGeneration(g)
	}

	d, err := dr.reconcileDeployment(ctx, obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	dr.updateDeploymentStatus(obj, d)

	if dr.serviceOptions != nil {
		dr.log.V(1).Info("reconciling service", "object", obj)
		s, err := dr.reconcileService(ctx, obj)
		if err != nil {
			return reconcile.Result{}, err
		}

		dr.log.V(1).Info("updating deployment status", "object", obj)
		dr.updateServiceStatus(obj, s)
	}

	return reconcile.Result{}, nil
}

func (dr *deploymentReconciler) reconcileDeployment(ctx context.Context, obj reconciler.Object) (*appsv1.Deployment, error) {
	dr.log.V(1).Info("reconciling deployment", "object", obj)

	desired, err := dr.createDeploymentFromRegistered(obj)
	if err != nil {
		return nil, fmt.Errorf("could not render deployment object: %w", err)
	}

	dr.log.V(5).Info("desired deployment object", "object", *desired)

	existing := &appsv1.Deployment{}
	err = dr.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		if semantic.Semantic.DeepEqual(desired, existing) {
			return existing, nil
		}

		dr.log.Info("existing deployment does not match the expected", "object", desired)
		dr.log.V(5).Info("mismatched deployment", "desired", *desired, "existing", *existing)

		// resourceVersion must be returned to the API server unmodified for
		// optimistic concurrency, as per Kubernetes API conventions
		desired.SetResourceVersion(existing.GetResourceVersion())

		if err = dr.client.Update(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not update deployment object: %+w", err)
		}

	case apierrs.IsNotFound(err):
		dr.log.Info("creating deployment", "object", desired)
		dr.log.V(5).Info("desired deployment", "object", *desired)
		if err = dr.client.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not create deployment object: %w", err)
		}

	default:
		return nil, fmt.Errorf("could not retrieve controlled object %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	return desired, nil
}

func (dr *deploymentReconciler) updateDeploymentStatus(obj reconciler.Object, d *appsv1.Deployment) {
	dr.log.V(1).Info("updating deployment status", "object", obj)

	desired := &apicommon.Condition{
		Type:               ConditionTypeDeploymentReady,
		Reason:             "DeploymentUnknown",
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
	}

	if d != nil {
		for _, c := range d.Status.Conditions {
			if c.Type != appsv1.DeploymentAvailable {
				continue
			}
			switch c.Status {
			case corev1.ConditionTrue:
				desired.Status = metav1.ConditionTrue
				desired.Reason = c.Reason

			case corev1.ConditionFalse:
				desired.Status = metav1.ConditionFalse
				desired.Reason = c.Reason
				desired.Message = c.Message
			default:
				desired.Message = fmt.Sprintf(
					"%q condition for deployment contains an unexpected status: %s",
					c.Type, c.Status)
			}
			break
		}
	}

	obj.GetStatusManager().SetCondition(desired)
}

func (dr *deploymentReconciler) createDeploymentFromRegistered(obj reconciler.Object) (*appsv1.Deployment, error) {
	replicas := defaultReplicas
	if dr.formFactor != nil {
		replicas = dr.formFactor.Replicas
	}

	return resources.NewDeployment(obj.GetNamespace(), obj.GetName(),
		resources.DeploymentWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, dr.name),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, reconciler.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, reconciler.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, reconciler.ManagedBy),

			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.DeploymentSetReplicas(int32(replicas)),
		resources.DeploymentAddSelectorForTemplate(resources.AppNameLabel, dr.name),
		resources.DeploymentAddSelectorForTemplate(resources.AppInstanceLabel, obj.GetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppComponentLabel, reconciler.ComponentWorkload),

		resources.DeploymentWithTemplateSpecOptions(
			resources.PodTemplateSpecWithPodSpecOptions(
				resources.PodSpecAddContainer(
					resources.NewContainer(
						reconciler.DefaultContainerName,
						dr.fromImage.Repo,
						obj.AsContainerOptions()...,
					))))), nil
}

func (dr *deploymentReconciler) reconcileService(ctx context.Context, obj reconciler.Object) (*corev1.Service, error) {
	dr.log.V(1).Info("reconciling service", "object", obj)
	desired, err := dr.createServiceFromRegistered(obj)
	if err != nil {
		return nil, fmt.Errorf("could not render service object: %w", err)
	}

	dr.log.V(5).Info("desired service object", "object", *desired)

	existing := &corev1.Service{}
	err = dr.client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case err == nil:
		if semantic.Semantic.DeepEqual(desired, existing) {
			return desired, nil
		}

		dr.log.Info("existing service does not match the expected", "object", desired)
		dr.log.V(5).Info("mismatched service", "desired", *desired, "existing", *existing)

		// resourceVersion must be returned to the API server unmodified for
		// optimistic concurrency, as per Kubernetes API conventions
		desired.SetResourceVersion(existing.GetResourceVersion())

		if err = dr.client.Update(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not update service object: %+w", err)
		}

	case apierrs.IsNotFound(err):
		dr.log.Info("creating service", "object", desired)
		dr.log.V(5).Info("desired service", "object", *desired)
		if err = dr.client.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("could not create service object: %w", err)
		}
	default:
		return nil, fmt.Errorf("could not retrieve controlled service %s: %w", client.ObjectKeyFromObject(desired), err)
	}

	return desired, nil
}

func (dr *deploymentReconciler) updateServiceStatus(obj reconciler.Object, s *corev1.Service) {
	dr.log.V(1).Info("updating service status", "object", obj)

	desired := &apicommon.Condition{
		Type:               ConditionTypeServiceReady,
		Reason:             "ServiceExist",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}

	address := ""
	if s == nil {
		desired.Status = metav1.ConditionFalse
		desired.Reason = "ServiceDoesNotExist"

	} else {
		address = fmt.Sprintf("http://%s.%s.svc.%s", s.Name, s.Namespace, resolver.ClusterDomain)
	}

	sm := obj.GetStatusManager()
	sm.SetAddressURL(address)
	sm.SetCondition(desired)
}

func (dr *deploymentReconciler) createServiceFromRegistered(obj reconciler.Object) (*corev1.Service, error) {
	return resources.NewService(obj.GetNamespace(), obj.GetName(),
		resources.ServiceWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, dr.name),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, reconciler.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, reconciler.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, reconciler.ManagedBy),
			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.ServiceAddSelectorLabel(resources.AppNameLabel, dr.name),
		resources.ServiceAddSelectorLabel(resources.AppInstanceLabel, obj.GetName()),
		resources.ServiceAddSelectorLabel(resources.AppComponentLabel, reconciler.ComponentWorkload),
		resources.ServiceAddPort("", dr.serviceOptions.Port, dr.serviceOptions.TargetPort),
	), nil
}
