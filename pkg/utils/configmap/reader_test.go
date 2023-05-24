// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0
package configmap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	tNamespace = "test-namespace"
	tName      = "test-name"
	tKey       = "test-key"
	tContents  = "I became insane, with long intervals of horrible sanity."
)

func TestNamespacedReader(t *testing.T) {
	testCases := map[string]struct {
		objects []client.Object

		expectedErr      string
		expectedContents string
	}{
		"configmap read": {
			objects: []client.Object{
				newConfigMap(tName, tKey),
			},
			expectedContents: tContents,
		},
		"configmap not found": {
			objects:     []client.Object{},
			expectedErr: `configmaps "` + tName + `" not found`,
		},
		"configmap key not found": {
			objects: []client.Object{
				newConfigMap(tName, tKey+"-miss"),
			},
			expectedErr: `configmap "` + tName + `" does not contain key "` + tKey + `"`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cb := fake.NewClientBuilder()
			nr := NewNamespacedReader(tNamespace, cb.WithObjects(tc.objects...).Build())
			read, err := nr.Read(context.Background(), tName, tKey)

			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
				assert.Nil(t, read)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expectedContents, *read)
			}
		})
	}
}

func newConfigMap(name, key string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{}
	cm.SetName(tName)
	cm.SetNamespace(tNamespace)
	cm.Data = map[string]string{
		key: tContents,
	}

	return cm
}
