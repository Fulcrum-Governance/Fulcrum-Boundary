package demo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/governance/proofreceipt"
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

func TestWriteEvidencePackDirHashesEveryArtifact(t *testing.T) {
	pack, err := BuildEvidencePack(context.Background(), GitHubLethalTrifectaOptions{})
	if err != nil {
		t.Fatalf("build pack: %v", err)
	}
	dir := t.TempDir()
	if err := WriteEvidencePackDir(pack, dir); err != nil {
		t.Fatalf("write pack dir: %v", err)
	}
	// The manifest must list at least the receipt, record, conformance, tamper,
	// and caveats artifacts, and every listed artifact must exist with a SHA-256
	// that recomputes to the manifest value.
	if len(pack.Artifacts) < 5 {
		t.Fatalf("expected >=5 hashed artifacts, got %d", len(pack.Artifacts))
	}
	for _, a := range pack.Artifacts {
		body, err := os.ReadFile(filepath.Join(dir, a.Path))
		if err != nil {
			t.Fatalf("artifact %q missing on disk: %v", a.Path, err)
		}
		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); got != a.SHA256 {
			t.Fatalf("artifact %q hash mismatch: manifest=%s recomputed=%s", a.Path, a.SHA256, got)
		}
		if a.SizeBytes != int64(len(body)) {
			t.Fatalf("artifact %q size mismatch: manifest=%d actual=%d", a.Path, a.SizeBytes, len(body))
		}
	}
	// proof-receipt.json on disk must parse back into a receipt bound to the record.
	receiptBody, err := os.ReadFile(filepath.Join(dir, "proof-receipt.json"))
	if err != nil {
		t.Fatalf("read proof-receipt.json: %v", err)
	}
	var rt proofreceipt.ProofReceipt
	if err := json.Unmarshal(receiptBody, &rt); err != nil {
		t.Fatalf("parse proof-receipt.json: %v", err)
	}
	if rt.DecisionHash != pack.DecisionRecord.DecisionHash {
		t.Fatalf("on-disk receipt binding broken: %q != %q", rt.DecisionHash, pack.DecisionRecord.DecisionHash)
	}
	if err := rt.VerifyBinding(pack.DecisionRecord); err != nil {
		t.Fatalf("on-disk receipt fails VerifyBinding: %v", err)
	}
	// pack.json must be present on disk, parse correctly, and list the same
	// artifacts (by count and SHA-256) as pack.Artifacts.
	manifestBody, err := os.ReadFile(filepath.Join(dir, "pack.json"))
	if err != nil {
		t.Fatalf("pack.json missing: %v", err)
	}
	var mf struct {
		Artifacts []EvidenceArtifact `json:"artifacts"`
	}
	if err := json.Unmarshal(manifestBody, &mf); err != nil {
		t.Fatalf("parse pack.json: %v", err)
	}
	if len(mf.Artifacts) != len(pack.Artifacts) {
		t.Fatalf("pack.json artifact count mismatch: got %d want %d", len(mf.Artifacts), len(pack.Artifacts))
	}
	// Spot-check: every SHA-256 in the manifest matches the in-memory pack entry.
	for i, a := range mf.Artifacts {
		if a.SHA256 != pack.Artifacts[i].SHA256 {
			t.Fatalf("pack.json artifact[%d] %q SHA-256 mismatch: manifest=%s pack=%s",
				i, a.Path, a.SHA256, pack.Artifacts[i].SHA256)
		}
	}
}
