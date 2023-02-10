package base

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/client"

	apicommon "github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
	"github.com/triggermesh/scoby/pkg/reconciler/resources"
)

type ReconciledObject interface {
	client.Object

	RenderPodSpecOptions() ([]resources.PodSpecOption, error)
	AsKubeObject() client.Object

	StatusGetObservedGeneration() int64
	StatusSetObservedGeneration(generation int64)
	StatusGetCondition(conditionType string) *apicommon.Condition
	StatusSetCondition(condition *apicommon.Condition)
}

type ReconciledObjectFactory interface {
	NewReconciledObject() ReconciledObject
}

func NewReconciledObjectFactory(gvk schema.GroupVersionKind, smf StatusManagerFactory, psr PodSpecRenderer) ReconciledObjectFactory {
	return &reconciledObjectFactory{
		gvk: gvk,
		smf: smf,
		psr: psr,
	}
}

type reconciledObjectFactory struct {
	gvk schema.GroupVersionKind
	smf StatusManagerFactory
	psr PodSpecRenderer
}

func (rof *reconciledObjectFactory) NewReconciledObject() ReconciledObject {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(rof.gvk)
	ro := &reconciledObject{
		Unstructured: *u,

		sm:  rof.smf.ForObject(u),
		psr: rof.psr,
	}

	return ro
}

// func (rof *reconciledObjectFactory) NewObjectLegacy() client.Object {
// 	fmt.Printf("DEBUG DELETEME this is the legacy call\n")
// 	obj := &unstructured.Unstructured{}
// 	obj.SetGroupVersionKind(rof.gvk)
// 	fmt.Printf("DEBUG DELETEME this is the obj: %+v\n", obj)
// 	return obj
// }

// smf := reccommon.NewStatusManagerFactory(crd.GetStatusFlag(), "Ready", []string{ConditionTypeDeploymentReady, ConditionTypeServiceReady, ConditionTypeReady}, log)

// func NewReconciledObject ()ReconciledObject {
// 	u := &unstructured.Unstructured{}
// 	u.SetGroupVersionKind(r.gvk)

// 	return &reconciledObject{
// 		ro := &reconciledObject{
// 			Unstructured: u,
// 			sm:           r.smf.ForObject(u),
// 		}

// 	}
// }

type reconciledObject struct {
	unstructured.Unstructured
	sm  StatusManager
	psr PodSpecRenderer
}

func (rof *reconciledObject) AsKubeObject() client.Object {
	return &rof.Unstructured
}

func (ro *reconciledObject) RenderPodSpecOptions() ([]resources.PodSpecOption, error) {
	return ro.psr.Render(ro)
}

func (ro *reconciledObject) StatusGetObservedGeneration() int64 {
	return ro.sm.GetObservedGeneration()
}

func (ro *reconciledObject) StatusSetObservedGeneration(generation int64) {
	ro.sm.SetObservedGeneration(generation)
}

func (ro *reconciledObject) StatusSetCondition(condition *apicommon.Condition) {
	ro.sm.SetCondition(condition)
}

func (ro *reconciledObject) StatusGetCondition(conditionType string) *apicommon.Condition {
	return ro.sm.GetCondition(conditionType)
}

// func (ro *reconciledObject) GetObject() client.Object {
// 	return ro.unstructured
// }

// func (ro *reconciledObject) StatusEqual(reconciledObject ReconciledObject) bool {
// 	// uIn := reconciledObject.GetObject().(*unstructured.Unstructured)
// 	uIn := reconciledObject.(*unstructured.Unstructured)
// 	// uIn := objIn.(*unstructured.Unstructured)
// 	stIn, okIn := uIn.Object["status"]
// 	st, ok := ro.unstructured.Object["status"]

// 	if okIn != ok {
// 		return false
// 	}

// 	return !semantic.Semantic.DeepEqual(&stIn, &st)
// }
