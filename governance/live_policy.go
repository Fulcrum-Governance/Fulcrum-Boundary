package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// policyBundle is one immutable view of the static rules and the hash of the
// bundle they came from. Evaluate reads one bundle up front, so a single
// decision never mixes rules from two bundles, and its record names the
// bundle that decided. err is set when a live reload failed; the pipeline
// then blocks instead of reusing older rules.
//
// fingerprint covers file names and raw bytes, which the receipt hash
// deliberately does not: a rename changes each rule's policy_file without
// changing the bundle hash, and the cached rules must not keep the old name.
// An empty fingerprint never matches, so the first live evaluation always
// loads from disk.
type policyBundle struct {
	rules       []StaticPolicyRule
	hash        string
	fingerprint string
	err         error
}

// activePolicies returns the bundle that decides this evaluation. Without a
// PolicyDir it is the construction-time bundle. With one, it fingerprints the
// directory and reloads when anything moved, so the next evaluation after a
// policy write sees the new rules (FUL-771).
func (p *Pipeline) activePolicies() *policyBundle {
	current := p.policies.Load()
	if p.policyDir == "" {
		return current
	}
	before, err := policyDirFingerprint(p.policyDir)
	if err != nil {
		return p.storeBundle(&policyBundle{err: fmt.Errorf("read policy directory: %w", err)})
	}
	if current.err == nil && current.fingerprint != "" && before == current.fingerprint {
		return current
	}
	hash, err := PolicyBundleHashFromDir(p.policyDir)
	if err != nil {
		return p.storeBundle(&policyBundle{err: fmt.Errorf("hash policy directory: %w", err)})
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
	if err := requireRules(rules, p.requiredRules); err != nil {
		return p.storeBundle(&policyBundle{err: err})
	}
	// The files may have moved while they were hashed and loaded; then the
	// rules cannot be tied to one hash, so this evaluation blocks and the
	// next one reloads.
	after, err := policyDirFingerprint(p.policyDir)
	if err != nil || after != before {
		return &policyBundle{err: fmt.Errorf("policy bundle changed during reload")}
	}
	return p.storeBundle(&policyBundle{rules: rules, hash: hash, fingerprint: before})
}

func (p *Pipeline) storeBundle(b *policyBundle) *policyBundle {
	p.policies.Store(b)
	return b
}

// policyDirFingerprint hashes the name and raw bytes of every policy file the
// loader reads (non-symlinked .yaml/.yml entries), in name order.
func policyDirFingerprint(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	sum := sha256.New()
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		// #nosec G304 -- path is assembled from os.ReadDir entries in the operator-selected policy directory; symlinks are skipped above.
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(sum, "%d:%s\x00%d:", len(entry.Name()), entry.Name(), len(body))
		sum.Write(body)
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// requireRules reports the first required rule that rules does not contain
// exactly, ignoring which file it came from.
func requireRules(rules, required []StaticPolicyRule) error {
	for _, want := range required {
		want.PolicyFile = ""
		found := false
		for _, have := range rules {
			have.PolicyFile = ""
			if reflect.DeepEqual(have, want) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("policy bundle is missing or alters required rule %q", want.Name)
		}
	}
	return nil
}
