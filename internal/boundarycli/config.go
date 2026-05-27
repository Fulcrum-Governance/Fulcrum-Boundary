package boundarycli

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type RuntimeConfig struct {
	Version    string                 `yaml:"version"`
	Mode       string                 `yaml:"mode"`
	Server     ServerConfig           `yaml:"server"`
	Standalone StandaloneConfig       `yaml:"standalone"`
	Kernel     KernelConnectionConfig `yaml:"kernel"`
	Security   SecurityConfig         `yaml:"security"`
}

type ServerConfig struct {
	Listen   string `yaml:"listen"`
	Upstream string `yaml:"upstream"`
}

type StandaloneConfig struct {
	PolicyDir string `yaml:"policy_dir"`
}

type KernelConnectionConfig struct {
	PolicyEngine RedisConfig `yaml:"policy_engine"`
	Trust        RedisConfig `yaml:"trust"`
	Budget       APIConfig   `yaml:"budget"`
	Escalation   NATSConfig  `yaml:"escalation"`
	Audit        NATSConfig  `yaml:"audit"`
	Envelope     NATSConfig  `yaml:"envelope"`
}

type RedisConfig struct {
	Type      string `yaml:"type"`
	RedisURL  string `yaml:"redis_url"`
	KeyPrefix string `yaml:"key_prefix"`
}

type APIConfig struct {
	Type     string `yaml:"type"`
	Endpoint string `yaml:"endpoint"`
}

type NATSConfig struct {
	Type    string `yaml:"type"`
	NATSURL string `yaml:"nats_url"`
	Subject string `yaml:"subject"`
}

type SecurityConfig struct {
	RequireAgentID bool `yaml:"require_agent_id"`
}

func LoadRuntimeConfig(path string) (*RuntimeConfig, error) {
	// #nosec G304 -- config path is an explicit operator-selected CLI input.
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg RuntimeConfig
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c RuntimeConfig) Validate() error {
	mode := strings.TrimSpace(c.Mode)
	if mode == "" {
		return fmt.Errorf("mode is required")
	}
	switch mode {
	case "standalone":
		if c.Standalone.PolicyDir == "" {
			return fmt.Errorf("standalone.policy_dir is required")
		}
	case "kernel":
		if c.Kernel.PolicyEngine.Type != "redis" || c.Kernel.PolicyEngine.RedisURL == "" || c.Kernel.PolicyEngine.KeyPrefix == "" {
			return fmt.Errorf("kernel.policy_engine requires type=redis, redis_url, and key_prefix")
		}
		if c.Kernel.Trust.Type != "redis_ipc" || c.Kernel.Trust.RedisURL == "" || c.Kernel.Trust.KeyPrefix == "" {
			return fmt.Errorf("kernel.trust requires type=redis_ipc, redis_url, and key_prefix")
		}
		if c.Kernel.Budget.Type != "api" || c.Kernel.Budget.Endpoint == "" {
			return fmt.Errorf("kernel.budget requires type=api and endpoint")
		}
		if err := validateNATS("kernel.escalation", c.Kernel.Escalation); err != nil {
			return err
		}
		if err := validateNATS("kernel.audit", c.Kernel.Audit); err != nil {
			return err
		}
		if err := validateNATS("kernel.envelope", c.Kernel.Envelope); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported mode %q", c.Mode)
	}
	return nil
}

func validateNATS(name string, cfg NATSConfig) error {
	if cfg.Type != "nats" || cfg.NATSURL == "" || cfg.Subject == "" {
		return fmt.Errorf("%s requires type=nats, nats_url, and subject", name)
	}
	return nil
}
