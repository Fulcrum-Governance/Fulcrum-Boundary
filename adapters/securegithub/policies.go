package securegithub

import "github.com/fulcrum-governance/fulcrum-boundary/governance"

func DefaultPolicyRules() []governance.StaticPolicyRule {
	return []governance.StaticPolicyRule{
		{
			Name:         "deny-github-repo-scope-violation",
			Tool:         "github.*",
			Action:       "deny",
			Reason:       "Secure GitHub fixture denies cross-repo tool calls when one-repo-per-session is enabled",
			Transport:    string(governance.TransportMCP),
			DecisionMode: governance.DecisionModeDeterministic,
			Conditions: []governance.StaticPolicyMatch{
				{Type: "equals", Field: "arguments.profile_id", Value: ProfileID},
				{Type: "equals", Field: "arguments.repo_scope_violation", Value: "true"},
			},
			Metadata: map[string]string{"profile": ProfileID},
		},
		{
			Name:         "deny-github-write-after-taint-fixture",
			Tool:         "github.*",
			Action:       "deny",
			Reason:       "GitHub fixture denies protected private-repo write after external collaborator taint before upstream execution",
			Transport:    string(governance.TransportMCP),
			DecisionMode: governance.DecisionModeDeterministic,
			Conditions: []governance.StaticPolicyMatch{
				{Type: "equals", Field: "arguments.profile_id", Value: ProfileID},
				{Type: "equals", Field: "arguments.tainted", Value: "true"},
				{Type: "equals", Field: "arguments.target_sink", Value: "private_repo"},
				{Type: "equals", Field: "arguments.capability_class", Value: "W1"},
			},
			Metadata: map[string]string{
				"profile":   ProfileID,
				"risk_path": "github.issue_body -> private_repo.write",
			},
		},
		{
			Name:         "deny-github-critical-write-after-taint-fixture",
			Tool:         "github.*",
			Action:       "deny",
			Reason:       "GitHub fixture denies critical private-repo mutation after external collaborator taint before upstream execution",
			Transport:    string(governance.TransportMCP),
			DecisionMode: governance.DecisionModeDeterministic,
			Conditions: []governance.StaticPolicyMatch{
				{Type: "equals", Field: "arguments.profile_id", Value: ProfileID},
				{Type: "equals", Field: "arguments.tainted", Value: "true"},
				{Type: "equals", Field: "arguments.target_sink", Value: "private_repo"},
				{Type: "equals", Field: "arguments.capability_class", Value: "W2"},
			},
			Metadata: map[string]string{
				"profile":   ProfileID,
				"risk_path": "github.issue_body -> private_repo.critical_mutation",
			},
		},
	}
}
