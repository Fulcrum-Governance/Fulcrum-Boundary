package commandboundary

import "github.com/fulcrum-governance/fulcrum-boundary/governance"

// DefaultPreviewPolicyRules maps Command Boundary classes to the preview
// enforcement posture documented for v0.4. C0 has no rule: the shared
// governance pipeline's default allow decision remains in place.
func DefaultPreviewPolicyRules() []governance.StaticPolicyRule {
	return []governance.StaticPolicyRule{
		commandClassRule("command-c1-warn", ClassLocalFileWrite, "warn", "local file write routed through Command Boundary preview"),
		commandClassRule("command-c2-require-approval", ClassNetworkEgress, "require_approval", "network egress requires approval"),
		commandClassRule("command-c3-require-approval", ClassRepositoryMutation, "require_approval", "repo mutation requires approval"),
		commandClassRule("command-c4-deny", ClassDestructiveMutation, "deny", "destructive local mutation denied"),
		commandClassRule("command-c5-deny", ClassInfrastructureMutation, "deny", "infrastructure or runtime mutation denied"),
		commandClassRule("command-c6-deny", ClassCredentialAccess, "deny", "credential or secret access denied"),
		commandClassRule("command-c7-require-approval", ClassPackageLifecycle, "require_approval", "package lifecycle execution requires approval"),
	}
}

func NewDefaultPreviewPipeline() *governance.Pipeline {
	return governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: DefaultPreviewPolicyRules(),
	}, nil, nil, nil)
}

func commandClassRule(name string, class Class, action, reason string) governance.StaticPolicyRule {
	return governance.StaticPolicyRule{
		Name:      name,
		Tool:      "*",
		Transport: string(governance.TransportCLI),
		Action:    action,
		Reason:    reason,
		Match: &governance.StaticPolicyMatch{
			Type:  "equals",
			Field: "arguments.command_class",
			Value: string(class),
		},
	}
}
