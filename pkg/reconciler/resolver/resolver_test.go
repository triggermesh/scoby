// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0
package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	tAPIVersion = "test/v1"
	tKind       = "TestKind"
	tNamespace  = "test-namespace"
	tName       = "test-name"

	tAddress = "http://test"
)

func TestObjectRender(t *testing.T) {
	testCases := map[string]struct {
		objects []client.Object
		ref     *v1alpha1.Reference

		expectedErr string
		expectedURL string
	}{
		"status not informed": {
			objects: []client.Object{
				newObject(),
			},
			ref: &v1alpha1.Reference{
				APIVersion: tAPIVersion,
				Kind:       tKind,
				Name:       tName,
				Namespace:  tNamespace,
			},
			expectedErr: `object does not inform "status.address.url"`,
			expectedURL: "",
		},
		"status informed": {
			objects: []client.Object{
				newObject(withStatusAddress(tAddress)),
			},
			ref: &v1alpha1.Reference{
				APIVersion: tAPIVersion,
				Kind:       tKind,
				Name:       tName,
				Namespace:  tNamespace,
			},
			expectedErr: "",
			expectedURL: tAddress,
		},
		"corev1 service": {
			objects: []client.Object{
				newService(),
			},
			ref: &v1alpha1.Reference{
				APIVersion: "v1",
				Kind:       "Service",
				Name:       tName,
				Namespace:  tNamespace,
			},
			expectedErr: "",
			expectedURL: "http://test-name.test-namespace.svc.cluster.local",
		},

		"wrong status value": {
			objects: []client.Object{
				newObject(withStatusAddress(true)),
			},
			ref: &v1alpha1.Reference{
				APIVersion: tAPIVersion,
				Kind:       tKind,
				Name:       tName,
				Namespace:  tNamespace,
			},
			expectedErr: `unexpected value at "status.address.url": ` +
				`.status.address.url accessor error: true is of the type bool, expected string`,
			expectedURL: "",
		},
		"object not found": {
			objects: []client.Object{},
			ref: &v1alpha1.Reference{
				APIVersion: tAPIVersion,
				Kind:       tKind,
				Name:       tName,
				Namespace:  tNamespace,
			},
			expectedErr: `testkinds.test "test-name" not found`,
			expectedURL: "",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cb := fake.NewClientBuilder()
			r := New(cb.WithObjects(tc.objects...).Build())
			url, err := r.Resolve(context.Background(), tc.ref)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expectedURL, url)
		})
	}
}

type objectOption func(*unstructured.Unstructured)

func newObject(opts ...objectOption) client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(tAPIVersion)
	u.SetKind(tKind)
	u.SetNamespace(tNamespace)
	u.SetName(tName)

	for _, opt := range opts {
		opt(u)
	}
	return u
}

func withStatusAddress(value interface{}) objectOption {
	return func(u *unstructured.Unstructured) {
		if err := unstructured.SetNestedField(u.Object, value, "status", "address", "url"); err != nil {
			panic(err)
		}
	}
}

func newService() *corev1.Service {
	s := &corev1.Service{}
	s.SetName(tName)
	s.SetNamespace(tNamespace)
	s.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))
	return s
}
