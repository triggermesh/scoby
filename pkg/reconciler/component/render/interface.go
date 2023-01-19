package render

import "sigs.k8s.io/controller-runtime/pkg/client"

type Renderer interface {
	EnsureCreated(obj client.Object) error
}
