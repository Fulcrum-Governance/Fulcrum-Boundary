package governance

import (
	"errors"
	"testing"
)

func TestValidateRejectsStaticRuleActionsThePipelineIgnores(t *testing.T) {
	for _, action := range []string{"Deny", " deny", "block", ""} {
		cfg := PipelineConfig{StaticPolicies: []StaticPolicyRule{{Name: "r", Tool: "*", Action: action}}}
		if err := cfg.Validate(); !errors.Is(err, ErrInvalidStaticRuleAction) {
			t.Errorf("action %q: Validate() = %v, want ErrInvalidStaticRuleAction", action, err)
		}
	}
	for _, action := range []string{"allow", "deny", "warn", "audit", "escalate", "require_approval"} {
		cfg := PipelineConfig{StaticPolicies: []StaticPolicyRule{{Name: "r", Tool: "*", Action: action}}}
		if err := cfg.Validate(); err != nil {
			t.Errorf("action %q: Validate() = %v, want nil", action, err)
		}
	}
}
