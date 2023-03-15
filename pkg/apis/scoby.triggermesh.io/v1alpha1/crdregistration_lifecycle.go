// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
)

func (r *CRDRegistration) GetWorkload() *common.Workload {
	return &r.Spec.Workload
}

const (
	CRDRegistrationConditionCRDExists       = "CRDExists"
	CRDRegistrationConditionControllerReady = "ControllerReady"
)

func (s *CRDRegistration) GetStatusManager() *common.StatusManager {
	sm := common.NewStatusManager(
		&s.Status.Status,
		"Ready",
		map[string]struct{}{
			CRDRegistrationConditionControllerReady: {},
			CRDRegistrationConditionCRDExists:       {},
			"Ready":                                 {},
		})

	return sm
}
