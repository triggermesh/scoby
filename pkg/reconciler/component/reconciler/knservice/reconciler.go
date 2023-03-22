package knservice

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/reconciler"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

const (
	ConditionTypeKnativeServiceReady = "KnativeServiceReady"

	ConditionReasonKnativeServiceReady   = "KNSERVICEOK"
	ConditionReasonKnativeServiceUnknown = "KNSERVICEUNKOWN"
)

func New(formFactor *apicommon.KnativeServiceFormFactor, log logr.Logger) reconciler.FormFactorReconciler {
	sr := &knserviceReconciler{
		formFactor: formFactor,
		log:        log,
	}

	return sr
}

type knserviceReconciler struct {
	formFactor *apicommon.KnativeServiceFormFactor

	log logr.Logger
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
func (sr *knserviceReconciler) Reconcile(context.Context, reconciler.Object) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
