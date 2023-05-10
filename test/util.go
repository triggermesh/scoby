package test

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

func ReadCRD(crd string) *apiextensionsv1.CustomResourceDefinition {
	sch := runtime.NewScheme()

	err := scheme.AddToScheme(sch)
	if err != nil {
		panic(err)
	}

	err = apiextensionsv1.AddToScheme(sch)
	if err != nil {
		panic(err)
	}

	obj, _, err := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode([]byte(crd), nil, nil)
	if err != nil {
		panic(err)
	}

	return obj.(*apiextensionsv1.CustomResourceDefinition)
}
