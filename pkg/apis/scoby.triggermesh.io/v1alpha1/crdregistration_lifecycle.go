// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/common"
)

func (r *CRDRegistration) GetWorkload() *common.Workload {
	return &r.Spec.Workload
}

// func (r *CRDRegistration) GetConfiguration() *common.Configuration {
// 	return r.Spec.Configuration
// }

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

	// Status manager is being retrieved to update the state,
	// set the status generation to the object's to reflect the
	// reconciled generation.
	// sm.InitializeforUpdate(s.GetGeneration())

	return sm

}
