package deployment

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render/podspec"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

type Renderer struct {
	ff *common.DeploymentFormFactor
	po *common.ParameterOptions

	psr *podspec.Renderer
}

func New(ff common.DeploymentFormFactor, image string) *Renderer {
	return &Renderer{
		ff:  &ff,
		psr: podspec.New("adapter", image),
	}
}

func (r *Renderer) EnsureCreated(obj client.Object) error {
	d, err := r.createDeploymentFrom(obj)
	if err != nil {
		return err
	}

	fmt.Printf("DEBUG DELETEME deployment: %+v", *d)

	return nil

}

func (r *Renderer) createDeploymentFrom(obj client.Object) (*appsv1.Deployment, error) {
	// TODO generate names
	// use form factor for replicas
	// use parameter options to define parameters policy
	// use obj to gather
	ps, _ := r.psr.Render(obj)

	return resources.NewDeployment(obj.GetNamespace(), obj.GetName(),
		resources.DeploymentWithMetaOptions(
			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind())),
		resources.DeploymentWithTemplateSpecOptions(
			// resources.PodTemplateSpecWithMetaOptions(),
			resources.PodTemplateSpecWithPodSpecOptions(ps...))), nil
}

func (r *Renderer) EnsureRemoved() {

}
