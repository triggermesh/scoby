// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"sync"

	"github.com/kelseyhightower/envconfig"
)

type ScobyConfig interface {
	ScobyNamespace() string
	WorkingNamespaces() []string
}

// ParseFromEnvironment loads the configuration into a singleton.
// This function must be called soon at `main()` before using any
// configuration item.
func ParseFromEnvironment() {
	cfg = &scobyConfig{}
	cfg.m.Lock()
	defer cfg.m.Unlock()

	envconfig.MustProcess("", cfg)
}

type scobyConfig struct {
	ScobyNs   string   `envconfig:"SCOBY_NAMESPACE" required:"true"`
	WorkingNs []string `envconfig:"WORKING_NAMESPACES"`

	m sync.RWMutex
}

var cfg *scobyConfig

func (sc *scobyConfig) ScobyNamespace() string {
	sc.m.RLock()
	defer sc.m.RUnlock()

	return sc.ScobyNs
}

func (sc *scobyConfig) WorkingNamespaces() []string {
	sc.m.RLock()
	defer sc.m.RUnlock()

	return sc.WorkingNs
}

// Get Scoby configuration
func Get() ScobyConfig {
	return cfg
}
