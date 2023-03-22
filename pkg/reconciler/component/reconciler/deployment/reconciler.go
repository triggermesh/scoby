package deployment

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
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
		client:     client,
		log:        log,
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
	dr.log.V(5).Info("Reconciling object", "object", obj)
	return ctrl.Result{}, nil
}
