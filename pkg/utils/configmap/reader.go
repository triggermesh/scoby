package configmap

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reader interface {
	Read(ctx context.Context, name string, key string) (*string, error)
}

type reader struct {
	namespace string
	client    client.Client
}

func NewNamespacedReader(namespace string, client client.Client) Reader {
	return &reader{
		namespace: namespace,
		client:    client,
	}
}

func (r *reader) Read(ctx context.Context, name string, key string) (*string, error) {
	cm := &corev1.ConfigMap{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: r.namespace, Name: name}, cm); err != nil {
		return nil, err
	}

	if data, ok := cm.Data[key]; ok {
		return &data, nil
	}

	return nil, fmt.Errorf("configmap %q does not contain key %q", name, key)
}
