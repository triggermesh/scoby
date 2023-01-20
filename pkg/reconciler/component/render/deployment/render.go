package deployment

import (
	appsv1 "k8s.io/api/apps/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render/podspec"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

type Renderer struct {
	ff *common.DeploymentFormFactor
	po *common.ParameterOptions

	client client.Client
	psr    *podspec.Renderer
	log    logr.Logger
}

func New(ff common.DeploymentFormFactor, image string, log logr.Logger) *Renderer {
	return &Renderer{
		ff:  &ff,
		psr: podspec.New("adapter", image),
		log: log,
	}
}

func (r *Renderer) RenderControlledObjects(obj client.Object) ([]client.Object, error) {
	d, err := r.createDeploymentFrom(obj)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("DEBUG DELETEME deployment: %+v", *d)

	return []client.Object{d}, nil
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
