package demo

import (
	"context"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestBuildEvidencePackIsFixtureSafeAndWired(t *testing.T) {
	pack, err := BuildEvidencePack(context.Background(), GitHubLethalTrifectaOptions{})
	if err != nil {
		t.Fatalf("build evidence pack: %v", err)
	}
	if !pack.Passed || pack.Status != "pass" {
		t.Fatalf("pack did not pass: %#v", pack)
	}
	if !pack.FixtureOnly || pack.RequiresCredentials || pack.RequiresNetwork || pack.MutatesLiveSystems {
		t.Fatalf("pack must be fixture-only and local: %#v", pack)
	}
	// Secure GitHub stays preview / L0 — never production language in the pack.
	if pack.SecureGitHubStatus != "preview" || pack.BypassLadderLevel != "L0" {
		t.Fatalf("pack must hold preview/L0: status=%q level=%q", pack.SecureGitHubStatus, pack.BypassLadderLevel)
	}
	// Hard invariant: the wired witness is shown, but the decision mode is NOT proved.
	if pack.DecisionRecord.DecisionMode == governance.DecisionModeProved {
		t.Fatalf("decision mode must not be proved: %q", pack.DecisionRecord.DecisionMode)
	}
	// The receipt binds to the record by decision_hash and is checker-verified.
	if pack.ProofReceipt.DecisionHash != pack.DecisionRecord.DecisionHash {
		t.Fatalf("receipt decision_hash %q != record decision_hash %q",
			pack.ProofReceipt.DecisionHash, pack.DecisionRecord.DecisionHash)
	}
	if pack.ProofReceipt.ReceiptVersion != "proof-receipt-v0.1" {
		t.Fatalf("receipt version = %q, want proof-receipt-v0.1", pack.ProofReceipt.ReceiptVersion)
	}
	// The wired witness carries the budget + static-privilege invariants, both passing.
	if len(pack.ProofReceipt.Invariants) < 2 {
		t.Fatalf("receipt must carry >=2 invariants (budget + privilege), got %d", len(pack.ProofReceipt.Invariants))
	}
	for _, inv := range pack.ProofReceipt.Invariants {
		if inv.Result != "pass" {
			t.Fatalf("fixture invariant %q did not pass: %q", inv.TheoremID, inv.Result)
		}
	}
	if !pack.ReceiptVerified || !pack.RecordVerified {
		t.Fatalf("both record and receipt must verify clean: rec=%t receipt=%t", pack.RecordVerified, pack.ReceiptVerified)
	}
	// Tamper-negative cases: record-hash flip and receipt-binding break, both detected.
	if len(pack.TamperCases) < 2 {
		t.Fatalf("expected >=2 tamper cases, got %d", len(pack.TamperCases))
	}
	for _, c := range pack.TamperCases {
		if !c.Detected {
			t.Fatalf("tamper case %q was not detected (verifier failed to reject forgery)", c.Name)
		}
	}
	// Every caveat repeats the bypass/preview framing somewhere.
	joined := strings.ToLower(strings.Join(pack.Caveats, "\n"))
	for _, want := range []string{"bypass", "preview"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("caveats missing %q framing:\n%s", want, joined)
		}
	}
}
