package governance

import (
	"fmt"
	"time"
)

type TrustMode string

const (
	TrustModeDisabled   TrustMode = "disabled"
	TrustModeStandalone TrustMode = "standalone"
	TrustModeKernel     TrustMode = "kernel"
)

type ProductionTrustConfig struct {
	Mode       TrustMode             `yaml:"mode" json:"mode"`
	Standalone StandaloneTrustConfig `yaml:"standalone" json:"standalone"`
	Kernel     KernelTrustConfig     `yaml:"kernel" json:"kernel"`
}

type StandaloneTrustConfig struct {
	Theta                     float64 `yaml:"theta" json:"theta"`
	DegradedThreshold         float64 `yaml:"degraded_threshold" json:"degraded_threshold"`
	InitialAlpha              float64 `yaml:"initial_alpha" json:"initial_alpha"`
	InitialBeta               float64 `yaml:"initial_beta" json:"initial_beta"`
	DecayRate                 float64 `yaml:"decay_rate" json:"decay_rate"`
	SuccessWeight             float64 `yaml:"success_weight" json:"success_weight"`
	FailureWeight             float64 `yaml:"failure_weight" json:"failure_weight"`
	PartialAlphaWeight        float64 `yaml:"partial_alpha_weight" json:"partial_alpha_weight"`
	PartialBetaWeight         float64 `yaml:"partial_beta_weight" json:"partial_beta_weight"`
	DegradedFailureMultiplier float64 `yaml:"degraded_failure_multiplier" json:"degraded_failure_multiplier"`
}

type KernelTrustConfig struct {
	RedisURL   string        `yaml:"redis_url" json:"redis_url"`
	IPCPrefix  string        `yaml:"ipc_prefix" json:"ipc_prefix"`
	Timeout    time.Duration `yaml:"-" json:"-"`
	TimeoutMS  int           `yaml:"timeout_ms" json:"timeout_ms"`
	FailClosed bool          `yaml:"fail_closed" json:"fail_closed"`
}

func NewProductionTrustBackend(cfg ProductionTrustConfig) (TrustBackend, error) {
	switch cfg.Mode {
	case "", TrustModeDisabled:
		return nil, nil
	case TrustModeStandalone:
		return NewStandaloneTrustBackend(cfg.Standalone), nil
	case TrustModeKernel:
		return NewRedisTrustBackendFromConfig(cfg.Kernel)
	default:
		return nil, fmt.Errorf("unsupported trust mode %q", cfg.Mode)
	}
}

func (c StandaloneTrustConfig) withDefaults() StandaloneTrustConfig {
	if c.Theta == 0 {
		c.Theta = DefaultTrustIsolationThreshold
	}
	if c.DegradedThreshold == 0 {
		c.DegradedThreshold = DefaultTrustDegradedThreshold
	}
	if c.InitialAlpha == 0 {
		c.InitialAlpha = 1
	}
	if c.InitialBeta == 0 {
		c.InitialBeta = 1
	}
	if c.SuccessWeight == 0 {
		c.SuccessWeight = 1
	}
	if c.FailureWeight == 0 {
		c.FailureWeight = 1
	}
	if c.PartialAlphaWeight == 0 {
		c.PartialAlphaWeight = 0.5
	}
	if c.PartialBetaWeight == 0 {
		c.PartialBetaWeight = 0.5
	}
	if c.DegradedFailureMultiplier == 0 {
		c.DegradedFailureMultiplier = 2
	}
	return c
}

func (c KernelTrustConfig) withDefaults() KernelTrustConfig {
	if c.RedisURL == "" {
		c.RedisURL = "redis://localhost:6379"
	}
	if c.IPCPrefix == "" {
		c.IPCPrefix = "agent:"
	}
	if c.Timeout == 0 {
		if c.TimeoutMS > 0 {
			c.Timeout = time.Duration(c.TimeoutMS) * time.Millisecond
		} else {
			c.Timeout = 100 * time.Millisecond
		}
	}
	c.FailClosed = true
	return c
}
