package config

import (
	"sync"

	"github.com/kelseyhightower/envconfig"
)

type ScobyConfig interface {
	ScobyNamespace() string
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
	Namespace string `envconfig:"SCOBY_NAMESPACE" required:"true"`

	m sync.RWMutex
}

var cfg *scobyConfig

func (sc *scobyConfig) ScobyNamespace() string {
	sc.m.RLock()
	defer sc.m.RUnlock()

	return sc.Namespace
}

// Get Scoby configuration
func Get() ScobyConfig {
	return cfg
}
