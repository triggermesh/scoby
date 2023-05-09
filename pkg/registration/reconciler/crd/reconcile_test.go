// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package crd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	scobyv1alpha1 "github.com/triggermesh/scoby/pkg/apis/scoby/v1alpha1"
)

var _ = Describe("Running CRD Registration controller", func() {
	const (
		tCRDRegName = "test-crdred"
	)

	Context("when the referenced CRD does not exist", func() {
		It("should fail to reconcile", func() {
			crdreg := &scobyv1alpha1.CRDRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: tCRDRegName,
				},
				Spec: scobyv1alpha1.CRDRegistrationSpec{
					CRD: "non.existing.crd",
				},
			}
			Expect(k8sClient.Create(ctx, crdreg)).Should(Succeed())
			// when status is filled, the registration should
			// reflect that the CRD does not exist.
		})
	})
})
