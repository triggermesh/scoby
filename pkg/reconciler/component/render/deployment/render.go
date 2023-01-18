package deployment

import "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"

type Renderer struct {
	ff *common.DeploymentFormFactor
}

func New(ff *common.DeploymentFormFactor) *Renderer {
	return &Renderer{
		ff: ff,
	}
}

func (r *Renderer) EnsureCreated() {

	r.ff.Replicas
}

func (r *Renderer) EnsureRemoved() {

}
