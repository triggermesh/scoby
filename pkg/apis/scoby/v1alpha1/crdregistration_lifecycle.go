// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
)

func (r *CRDRegistration) GetWorkload() *commonv1alpha1.Workload {
	return &r.Spec.Workload
}

const (
	CRDRegistrationConditionCRDExists       = "CRDExists"
	CRDRegistrationConditionControllerReady = "ControllerReady"

	CRDRegistrationAnnotationHookURL = "hookURL"
)

func (s *CRDRegistration) GetStatusManager() *commonv1alpha1.StatusManager {
	sm := commonv1alpha1.NewStatusManager(
		&s.Status.Status,
		"Ready",
		map[string]struct{}{
			CRDRegistrationConditionControllerReady: {},
			CRDRegistrationConditionCRDExists:       {},
			"Ready":                                 {},
		})

	return sm
}
