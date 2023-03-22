package deployment

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
)

func New() reconciler.FormFactor {
	return &ffReconciler{}
}

type ffReconciler struct {
}

var _ reconciler.FormFactor = (*ffReconciler)(nil)

func (d *ffReconciler) SetupController(name string, c controller.Controller, owner runtime.Object) error {

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
func (d *ffReconciler) Reconcile(context.Context, reconciler.Object) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
