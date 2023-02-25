// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package base

import "github.com/triggermesh/scoby/pkg/reconciler/component/reconciler/base/object"

// Label values for generated workload
const (
	PartOf            = "scoby-component"
	ManagedBy         = "scoby-controller"
	ComponentWorkload = "workload"
)

// Common status conditions
const (
	ConditionTypeReady = "Ready"
)

// Aliases: Make it easy for consumers of the base component reconciler to
// use their internals without importing its internals

var (
	NewRenderer = object.NewRenderer
)

type ReconcilingObject = object.Reconciling
type RenderedObject = object.Rendered
