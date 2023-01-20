package deployment

import (
	appsv1 "k8s.io/api/apps/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render"
	"github.com/triggermesh/scoby/pkg/reconciler/component/render/podspec"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

const defaultReplicas = 1

type Renderer struct {
	ff  *common.DeploymentFormFactor
	po  *common.ParameterOptions
	reg common.Registration

	client client.Client
	psr    *podspec.Renderer
	log    logr.Logger
}

func New(reg common.Registration, log logr.Logger) *Renderer {
	return &Renderer{
		reg: reg,
		psr: podspec.New("adapter", reg.GetWorkload().FromImage.Repo),
		log: log,
	}
}

func (r *Renderer) RenderControlledObjects(obj client.Object) ([]client.Object, error) {
	d, err := r.createDeploymentFrom(obj)
	if err != nil {
		return nil, err
	}

	return []client.Object{d}, nil
}

func (r *Renderer) createDeploymentFrom(obj client.Object) (*appsv1.Deployment, error) {
	// TODO generate names

	// use parameter options to define parameters policy
	// use obj to gather
	ps, _ := r.psr.Render(obj)

	replicas := defaultReplicas
	if ffd := r.reg.GetWorkload().FormFactor.Deployment; ffd != nil {
		replicas = ffd.Replicas
	}

	return resources.NewDeployment(obj.GetNamespace(), obj.GetName(),
		resources.DeploymentWithMetaOptions(
			resources.MetaAddLabel(resources.AppNameLabel, r.reg.GetName()),
			resources.MetaAddLabel(resources.AppInstanceLabel, obj.GetName()),
			resources.MetaAddLabel(resources.AppComponentLabel, render.ComponentWorkload),
			resources.MetaAddLabel(resources.AppPartOfLabel, render.PartOf),
			resources.MetaAddLabel(resources.AppManagedByLabel, render.ManagedBy),

			resources.MetaAddOwner(obj, obj.GetObjectKind().GroupVersionKind()),
		),
		resources.DeploymentSetReplicas(int32(replicas)),
		resources.DeploymentAddSelectorForTemplate(resources.AppNameLabel, r.reg.GetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppInstanceLabel, obj.GetName()),
		resources.DeploymentAddSelectorForTemplate(resources.AppComponentLabel, render.ComponentWorkload),

		resources.DeploymentWithTemplateSpecOptions(
			// resources.PodTemplateSpecWithMetaOptions(),
			resources.PodTemplateSpecWithPodSpecOptions(ps...),
		)), nil
}

func (r *Renderer) EnsureRemoved() {

}
