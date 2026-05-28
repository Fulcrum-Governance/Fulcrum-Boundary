package editboundary

import "github.com/fulcrum-governance/fulcrum-boundary/governance"

// DefaultPreviewPolicyRules maps Edit Boundary classes to the v0.6 preview
// enforcement posture. E0/E1 have no rules: the shared pipeline's default
// allow decision remains in place for no-op and safe content edits.
func DefaultPreviewPolicyRules() []governance.StaticPolicyRule {
	return []governance.StaticPolicyRule{
		editClassRule("edit-e2-require-approval", ClassSourceConfig, "require_approval", "source or config edit requires approval"),
		editClassRule("edit-e3-require-approval", ClassDeploymentInfra, "require_approval", "deployment or infrastructure edit requires approval"),
		editClassRule("edit-e4-deny", ClassSecretBearing, "deny", "secret-bearing edit denied"),
		editClassRule("edit-e5-deny", ClassDestructive, "deny", "destructive edit denied"),
		editClassRule("edit-e6-require-approval", ClassExecutionBehavior, "require_approval", "execution behavior edit requires approval"),
		editClassRule("edit-e7-deny", ClassOutsideProjectScope, "deny", "outside-project edit denied"),
	}
}

func NewDefaultPreviewPipeline() *governance.Pipeline {
	return governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: DefaultPreviewPolicyRules(),
	}, nil, nil, nil)
}

func editClassRule(name string, class Class, action, reason string) governance.StaticPolicyRule {
	return governance.StaticPolicyRule{
		Name:      name,
		Tool:      "edit.apply",
		Transport: string(governance.TransportCLI),
		Action:    action,
		Reason:    reason,
		Match: &governance.StaticPolicyMatch{
			Type:  "equals",
			Field: "arguments.edit_class",
			Value: string(class),
		},
	}
}
