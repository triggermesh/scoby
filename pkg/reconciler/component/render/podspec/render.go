package podspec

import (
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Renderer struct {
	name  string
	image string
}

func New(name, image string) *Renderer {
	return &Renderer{
		name:  name,
		image: image,
	}
}

func (r *Renderer) Render(obj client.Object) ([]resources.PodSpecOption, error) {
	return []resources.PodSpecOption{resources.PodSpecAddContainer(
		resources.NewContainer(r.name, r.image),
	)}, nil
}
