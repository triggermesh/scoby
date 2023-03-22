package base

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/go-logr/logr"
	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
	"github.com/triggermesh/scoby/pkg/reconciler/semantic"
)

func NewController(
	om reconciler.ObjectManager,
	reg apicommon.Registration,
	ffr reconciler.FormFactorReconciler,
	mgr ctrl.Manager,
	log logr.Logger) (controller.Controller, error) {

	r := &base{
		objectManager:        om,
		formFactorReconciler: ffr,
		client:               mgr.GetClient(),
		log:                  log,
	}

	c, err := controller.NewUnmanaged(reg.GetName(), mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", reg.GetName(), err)
	}

	if err := ffr.SetupController(reg.GetName(), c, om.NewObject()); err != nil {
		return nil, fmt.Errorf("could not setup form factor controller for %q: %w", reg.GetName(), err)
	}

	return c, nil
}

var _ reconciler.Base = (*base)(nil)

type base struct {
	objectManager        reconciler.ObjectManager
	formFactorReconciler reconciler.FormFactorReconciler

	client client.Client
	log    logr.Logger
}

func (b *base) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	b.log.V(1).Info("Reconciling request", "request", req)

	obj := b.objectManager.NewObject()
	if err := b.client.Get(ctx, req.NamespacedName, obj.AsKubeObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	b.log.V(5).Info("Object retrieved", "object", obj)

	if !obj.GetDeletionTimestamp().IsZero() {
		// Return and let the ownership clean resources.
		return ctrl.Result{}, nil
	}

	// create a copy, we will compare after reconciling
	// and decide if we need to update or not.
	cp := obj.AsKubeObject().DeepCopyObject()

	// Render using the object data and configuration
	if err := b.objectManager.GetRenderer().Render(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	// If there are changes to status, update it.
	// Update status if needed.
	// TODO find a better expression for this
	if !semantic.Semantic.DeepEqual(
		obj.AsKubeObject().(*unstructured.Unstructured).Object["status"],
		cp.(*unstructured.Unstructured).Object["status"]) {
		if uperr := b.client.Status().Update(ctx, obj.AsKubeObject()); uperr != nil {
			// if err == nil {
			// 	return ctrl.Result{}, uperr
			// }
			b.log.Error(uperr, "could not update the object status")
		}
	}

	return ctrl.Result{}, nil
}
