package crd

import (
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Registered struct {
	statusFlag StatusFlag
	gvk        schema.GroupVersionKind
}

func NewRegisteredCRD(crd *apiextensionsv1.CustomResourceDefinition, reg common.Registration) *Registered {
	crdv := CRDPriotizedVersion(crd)

	return &Registered{
		statusFlag: CRDStatusFlag(crdv),
		gvk: schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: crdv.Name,
			Kind:    crd.Spec.Names.Kind,
		},
	}
}

func (r *Registered) GetStatusFlag() StatusFlag {
	return r.statusFlag
}

func (r *Registered) GetGVK() schema.GroupVersionKind {
	return r.gvk
}

// func (r *Registered) GetStatusManager() StatusManager{

// }