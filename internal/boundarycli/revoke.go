package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// revokableAgentID bounds what an agent ID may contain, because it becomes
// part of a file name inside the policy directory.
var revokableAgentID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// RevocationRuleName is the matched_rule a revoked agent's denials carry.
func RevocationRuleName(agentID string) string { return "revoked:" + agentID }

func revocationFile(policyDir, agentID string) string {
	return filepath.Join(policyDir, "revoked-"+agentID+".yaml")
}

// runRevoke writes (or with --lift removes) a deny-everything rule scoped to
// one agent in the policy bundle. A `boundary serve` started with the same
// policy directory picks it up on its next evaluation (FUL-771).
func runRevoke(args []string, stdout, stderr io.Writer) int {
	flags := newHelpFlagSet("boundary revoke", stderr, commandHelp{
		Purpose: "Revoke an agent: every later routed action from it is denied.",
		Usage:   "boundary revoke --agent ID [--policies DIR] [--reason TEXT] [--lift]",
		Common: []string{
			"boundary revoke --agent agent-7 --reason \"approval withdrawn\"",
			"boundary revoke --agent agent-7 --lift",
		},
		Notes: []string{
			"Writes revoked-<agent>.yaml into the policy directory. A running `boundary serve` using that directory applies it to the next evaluation; no restart.",
			"Scope: routed actions only. Revoking does not undo actions that already ran.",
			"--lift deletes the revocation file; the agent is then governed by the remaining rules.",
		},
	})
	policyDir := flags.String("policies", "./policies/", "policy directory the gateway serves")
	agentID := flags.String("agent", "", "agent ID to revoke")
	reason := flags.String("reason", "approval withdrawn", "reason recorded on every denial")
	lift := flags.Bool("lift", false, "remove the revocation instead of writing it")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if !revokableAgentID.MatchString(*agentID) {
		fmt.Fprintln(stderr, "revoke: --agent must be 1-128 characters of letters, digits, '.', '_' or '-'")
		return 1
	}
	path := revocationFile(*policyDir, *agentID)
	if *lift {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "revoke: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "lifted revocation for %s (%s)\n", *agentID, path)
		return 0
	}
	if err := writeRevocation(*policyDir, *agentID, *reason); err != nil {
		fmt.Fprintf(stderr, "revoke: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "revoked %s: %s\n", *agentID, path)
	return 0
}

// writeRevocation lands the rule with a rename so a gateway re-reading the
// directory never parses a half-written file.
func writeRevocation(policyDir, agentID, reason string) error {
	doc := struct {
		Name    string                        `yaml:"name"`
		Version string                        `yaml:"version"`
		Rules   []governance.StaticPolicyRule `yaml:"rules"`
	}{
		Name:    "revocation-" + agentID,
		Version: "v1",
		Rules: []governance.StaticPolicyRule{{
			Name:       RevocationRuleName(agentID),
			Tool:       "*",
			Action:     "deny",
			Reason:     "revoked: " + reason,
			AgentScope: []string{agentID},
		}},
	}
	body, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(policyDir, ".revoke-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), revocationFile(policyDir, agentID))
}
