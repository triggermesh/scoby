package component

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
)

func NewController(ctx context.Context, gvk schema.GroupVersionKind, workload *common.Workload, mgr manager.Manager) (*reconciler, error) {
	logger := mgr.GetLogger()

	r := &reconciler{
		log:      logger.WithName(gvk.GroupKind().String()),
		gvk:      gvk,
		workload: workload,
	}

	if err := builder.ControllerManagedBy(mgr).
		For(r.newObject()).
		Complete(r); err != nil {
		return nil, fmt.Errorf("could not build controller for %q: %w", gvk, err)
	}
	return r, nil
}
