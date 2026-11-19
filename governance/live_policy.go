package governance

import (
	"fmt"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// policyBundle is one immutable view of the static rules and the hash of the
// bundle they came from. Evaluate reads one bundle up front, so a single
// decision never mixes rules from two bundles, and its record names the
// bundle that decided. err is set when a live reload failed; the pipeline
// then blocks instead of reusing older rules.
type policyBundle struct {
	rules []StaticPolicyRule
	hash  string
	err   error
}

// activePolicies returns the bundle that decides this evaluation. Without a
// PolicyDir it is the construction-time bundle. With one, it re-hashes the
// directory and reloads when the hash moved, so the next evaluation after a
// policy write sees the new rules (FUL-771).
func (p *Pipeline) activePolicies() *policyBundle {
	current := p.policies.Load()
	if p.policyDir == "" {
		return current
	}
	hash, err := PolicyBundleHashFromDir(p.policyDir)
	if err != nil {
		return p.storeBundle(&policyBundle{err: fmt.Errorf("hash policy directory: %w", err)})
	}
	if current.err == nil && hash == current.hash {
		return current
	}
	rules, err := LoadStaticPoliciesFromDir(p.policyDir)
	if err != nil {
		return p.storeBundle(&policyBundle{err: err})
	}
	for i, rule := range rules {
		if rule.Action != strings.ToLower(strings.TrimSpace(rule.Action)) || !policyeval.IsRuleAction(rule.Action) {
			return p.storeBundle(&policyBundle{err: fmt.Errorf("%w: rule %d (%q) has action %q", ErrInvalidStaticRuleAction, i, rule.Name, rule.Action)})
		}
	}
	// The files may have moved between hashing and loading; then the rules
	// cannot be tied to one hash, so this evaluation blocks and the next
	// one reloads.
	after, err := PolicyBundleHashFromDir(p.policyDir)
	if err != nil || after != hash {
		return &policyBundle{err: fmt.Errorf("policy bundle changed during reload")}
	}
	return p.storeBundle(&policyBundle{rules: rules, hash: hash})
}

func (p *Pipeline) storeBundle(b *policyBundle) *policyBundle {
	p.policies.Store(b)
	return b
}
