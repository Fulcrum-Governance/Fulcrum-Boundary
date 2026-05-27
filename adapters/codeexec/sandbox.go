package codeexec

import "fmt"

// Capability represents a class of sandbox permission.
type Capability string

const (
	CapabilityNetwork         Capability = "network"
	CapabilityFilesystemRead  Capability = "filesystem_read"
	CapabilityFilesystemWrite Capability = "filesystem_write"
	CapabilitySubprocess      Capability = "subprocess"
	CapabilityEnvAccess       Capability = "env_access"
)

// SandboxPolicy defines the constraints enforced on code execution.
type SandboxPolicy struct {
	AllowedCapabilities map[Capability]bool // capability -> allowed
	MaxOutputSize       int64               // bytes; default 50 KB
	AllowedLanguages    []string            // e.g. "python", "javascript"
}

// DefaultSandboxPolicy returns a restrictive baseline policy that only permits
// file reads and environment variable access, with a 50 KB output limit.
func DefaultSandboxPolicy() SandboxPolicy {
	return SandboxPolicy{
		AllowedCapabilities: map[Capability]bool{
			CapabilityNetwork:         false,
			CapabilityFilesystemRead:  true,
			CapabilityFilesystemWrite: false,
			CapabilitySubprocess:      false,
			CapabilityEnvAccess:       true,
		},
		MaxOutputSize:    50 * 1024, // 50 KB
		AllowedLanguages: []string{"python", "javascript", "typescript"},
	}
}

// operationCapability maps an operation type to the capability it requires.
var operationCapability = map[string]Capability{
	"network_call":      CapabilityNetwork,
	"file_read":         CapabilityFilesystemRead,
	"file_write":        CapabilityFilesystemWrite,
	"file_delete":       CapabilityFilesystemWrite,
	"subprocess":        CapabilitySubprocess,
	"system_call":       CapabilitySubprocess,
	"restricted_import": CapabilitySubprocess,
	"obfuscated_exec":   CapabilitySubprocess,
	"eval_delegation":   CapabilitySubprocess,
	"env_access":        CapabilityEnvAccess,
}

func normalizeSandboxPolicy(policy SandboxPolicy) SandboxPolicy {
	defaults := DefaultSandboxPolicy()
	if policy.AllowedCapabilities == nil {
		policy.AllowedCapabilities = defaults.AllowedCapabilities
	}
	if policy.MaxOutputSize == 0 {
		policy.MaxOutputSize = defaults.MaxOutputSize
	}
	if len(policy.AllowedLanguages) == 0 {
		policy.AllowedLanguages = defaults.AllowedLanguages
	}
	return policy
}

// LanguageAllowed reports whether a requested language is permitted.
func (p SandboxPolicy) LanguageAllowed(language string) bool {
	p = normalizeSandboxPolicy(p)
	for _, allowed := range p.AllowedLanguages {
		if allowed == language {
			return true
		}
	}
	return false
}

// EnforcePolicy evaluates a list of detected operations against a sandbox
// policy. It returns the operations that are allowed and a list of human-
// readable reasons for any denied operations.
func EnforcePolicy(policy SandboxPolicy, ops []Operation) (allowed []Operation, denied []string) {
	for _, op := range ops {
		cap, ok := operationCapability[op.Type]
		if !ok {
			// Unknown operation type — deny by default.
			denied = append(denied, fmt.Sprintf("unknown operation %q denied by default", op.Type))
			continue
		}
		if policy.AllowedCapabilities[cap] {
			allowed = append(allowed, op)
		} else {
			denied = append(denied, fmt.Sprintf("%s denied: capability %q not allowed (%s)", op.Type, cap, op.Detail))
		}
	}
	return allowed, denied
}
