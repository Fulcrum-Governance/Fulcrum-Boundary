package governance

import (
	"fmt"
	"time"
)

// TrustMode selects which trust backend (if any) the pipeline uses. See
// docs/TRUST_INTEGRATION.md for the wire-level details of each mode.
type TrustMode string

const (
	// TrustModeDisabled skips the trust stage entirely (no backend).
	TrustModeDisabled TrustMode = "disabled"
	// TrustModeStandalone uses the in-process Beta evaluator
	// (StandaloneTrustBackend); no external dependencies.
	TrustModeStandalone TrustMode = "standalone"
	// TrustModeKernel uses the Redis-backed fulcrum-trust IPC state
	// (RedisTrustBackend).
	TrustModeKernel TrustMode = "kernel"
)

// ProductionTrustConfig is the top-level trust configuration. Mode selects the
// backend; the matching nested section (Standalone or Kernel) supplies its
// parameters. The unused section is ignored.
type ProductionTrustConfig struct {
	Mode       TrustMode             `yaml:"mode" json:"mode"`
	Standalone StandaloneTrustConfig `yaml:"standalone" json:"standalone"`
	Kernel     KernelTrustConfig     `yaml:"kernel" json:"kernel"`
}

// StandaloneTrustConfig tunes the in-process Beta(alpha,beta) trust evaluator.
// Zero-valued fields are replaced with documented defaults at construction
// (see withDefaults); Theta and DegradedThreshold default to
// DefaultTrustIsolationThreshold and DefaultTrustDegradedThreshold.
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

// KernelTrustConfig configures the Redis-backed (kernel) trust backend.
// RedisURL is the redis://host:port endpoint; IPCPrefix is the agent key prefix
// (default "agent:"); Timeout is the per-command deadline (derived from
// TimeoutMS when set, otherwise 100ms). FailClosed is forced true by
// withDefaults: kernel mode always denies on a store fault by design.
type KernelTrustConfig struct {
	RedisURL   string        `yaml:"redis_url" json:"redis_url"`
	IPCPrefix  string        `yaml:"ipc_prefix" json:"ipc_prefix"`
	Timeout    time.Duration `yaml:"-" json:"-"`
	TimeoutMS  int           `yaml:"timeout_ms" json:"timeout_ms"`
	FailClosed bool          `yaml:"fail_closed" json:"fail_closed"`
}

// NewProductionTrustBackend constructs the TrustBackend selected by cfg.Mode:
// disabled (or unset) returns (nil, nil) so the pipeline skips the trust stage;
// standalone returns a StandaloneTrustBackend; kernel returns a
// RedisTrustBackend (and may error if the Redis URL is invalid). An unknown
// mode returns an error.
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
