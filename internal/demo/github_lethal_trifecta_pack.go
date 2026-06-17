package demo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/governance/proofreceipt"
)

// EvidencePackSchemaVersion identifies the JSON shape of the lethal-trifecta
// evidence pack manifest. It is demo-local and distinct from
// evidence.ManifestSchemaVersion (the generic bundle).
const EvidencePackSchemaVersion = "boundary.demo.github_lethal_trifecta.evidence_pack.v1"

// fixtureCheckerID/fixtureCheckerBuild identify the checker build that validated
// the demo's invariants. They are fixture-only labels: the demo runs the real
// in-process WS-1 checkers, so the receipt is wired (not mocked), but the build
// identity is a fixed demo string rather than a release artifact digest.
const (
	fixtureCheckerID    = "fulcrum-proof-checker/demo"
	fixtureCheckerBuild = "sha256:demo-fixture-build"
)

// EvidencePack is the fixture-only evidence pack for the GitHub lethal-trifecta
// wedge: the denied-mutation DecisionRecord, a WS-1 checker-validated proof
// receipt (the wired witness for the budget + static-privilege invariants, bound
// by decision_hash), route-conformance assertions, tamper-negative cases, and
// caveats. It performs no live mutation, no network, and needs no credentials.
// Secure GitHub stays preview at fixture bypass-ladder level L0.
type EvidencePack struct {
	SchemaVersion       string                      `json:"schema_version"`
	Status              string                      `json:"status"` // "pass" | "fail"
	Passed              bool                        `json:"passed"`
	FixtureOnly         bool                        `json:"fixture_only"`
	RequiresCredentials bool                        `json:"requires_credentials"`
	RequiresNetwork     bool                        `json:"requires_network"`
	MutatesLiveSystems  bool                        `json:"mutates_live_systems"`
	SecureGitHubStatus  string                      `json:"secure_github_status"` // always "preview"
	BypassLadderLevel   string                      `json:"bypass_ladder_level"`  // always "L0"
	DecisionRecord      governance.DecisionRecordV1 `json:"decision_record"`
	ProofReceipt        proofreceipt.ProofReceipt   `json:"proof_receipt"`
	ReceiptVerified     bool                        `json:"receipt_verified"`
	RecordVerified      bool                        `json:"record_verified"`
	TamperCases         []EvidenceTamperCase        `json:"tamper_cases"`
	RouteConformance    []EvidenceConformanceItem   `json:"route_conformance"`
	Artifacts           []EvidenceArtifact          `json:"artifacts"`
	Caveats             []string                    `json:"caveats"`
}

// EvidenceTamperCase is one negative case: mutate exactly one bound input and show
// the matching verifier rejects it. Name is the case; Detected is true when
// re-verification fails (the success condition for the case).
type EvidenceTamperCase struct {
	Name        string `json:"name"`
	Target      string `json:"target"` // "decision_record" | "proof_receipt"
	Detected    bool   `json:"detected"`
	VerifyError string `json:"verify_error,omitempty"`
}

// EvidenceConformanceItem is one route-conformance assertion (e.g. the write did
// not reach upstream, the read did, the record carries no proved mode).
type EvidenceConformanceItem struct {
	ID     string `json:"id"`
	Status string `json:"status"` // "pass" | "fail"
	Detail string `json:"detail"`
}

// EvidenceArtifact mirrors evidence.Artifact for on-disk pack files. It is
// intentionally local to avoid an import cycle (internal/evidence imports
// internal/demo). Populated in WS-3.2 (the on-disk writer); empty here.
type EvidenceArtifact struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"size_bytes"`
	SchemaVersion string `json:"schema_version,omitempty"`
}

// fixtureWitnesses returns the budget and static-privilege witnesses the
// lethal-trifecta fixture implies. The demo engine does not expose its internal
// budget/cap inputs, so these are documented fixture values: a denied write that
// stays within budget (so the budget invariant passes for the recorded decision)
// and a requested capability set that is a subset of the authorized set (so the
// static-privilege invariant passes). Both are bound to the record by
// decision_hash. They are fixture-only and never read from a live deployment.
func fixtureWitnesses(record governance.DecisionRecordV1) (proofreceipt.BudgetWitness, proofreceipt.PrivilegeWitness) {
	budget := proofreceipt.BudgetWitness{
		BudgetKey:      "fixture:github-lethal-trifecta",
		TenantID:       "fixture-tenant",
		AgentID:        "fixture-agent",
		Limit:          100,
		SpentBefore:    10,
		Requested:      0, // a denied write consumes no budget
		SpentAfter:     10,
		PolicyHash:     "sha256:fixture-policy",
		DecisionHash:   record.DecisionHash,
		TheoremID:      proofreceipt.TheoremBudgetLocal,
		CheckerVersion: "0.1.0",
	}
	privilege := proofreceipt.PrivilegeWitness{
		AgentID:        "fixture-agent",
		TenantID:       "fixture-tenant",
		RequestedCaps:  []string{"repo:read"},
		AuthorizedCaps: []string{"repo:read", "repo:write"},
		PolicyHash:     "sha256:fixture-policy",
		DecisionHash:   record.DecisionHash,
		TheoremID:      proofreceipt.TheoremPrivilegeStatic,
		CheckerVersion: "0.1.0",
	}
	return budget, privilege
}

// BuildEvidencePack runs the lethal-trifecta engine, attaches a WS-1
// checker-validated proof receipt (the wired witness) to the denial record, runs
// the negative (tamper) cases against both the record verifier and the receipt
// verifier, and returns the assembled pack. opts is the same options struct the
// demo uses. It never mutates the record or recomputes its decision_hash.
func BuildEvidencePack(ctx context.Context, opts GitHubLethalTrifectaOptions) (*EvidencePack, error) {
	result, err := RunGitHubLethalTrifecta(ctx, opts)
	if err != nil {
		return nil, err
	}
	record := result.DecisionRecord
	if record.RecordID == "" || record.DecisionHash == "" {
		return nil, fmt.Errorf("evidence pack: denial record missing record_id or decision_hash")
	}

	// 1) The untouched record verifies (decision_hash matches its body).
	recordVerified := governance.VerifyDecisionRecord(record, nil, "", "") == nil

	// 2) Run the real WS-1 checkers over the fixture witnesses, then attach the
	// receipt bound to this exact record by decision_hash. This is the WIRED
	// witness: the same checkers WS-1 ships, validating the budget + static-
	// privilege invariants, not a mock. (WS-1 coupling point.)
	budgetWitness, privilegeWitness := fixtureWitnesses(record)
	invariants := []proofreceipt.Invariant{
		proofreceipt.CheckBudget(budgetWitness),
		proofreceipt.CheckPrivilege(privilegeWitness),
	}
	receipt := proofreceipt.AttachAll(record, fixtureCheckerID, fixtureCheckerBuild, invariants, time.Time{})
	receiptVerified := receipt.VerifyBinding(record) == nil

	// 3) Negative cases. Case A: forge the verdict in the record, leave the stored
	// decision_hash, show VerifyDecisionRecord rejects it (mirrors tamper_evidence.go).
	forgedRecord := record
	if forgedRecord.Action == "deny" {
		forgedRecord.Action = "allow"
	} else {
		forgedRecord.Action = "deny"
	}
	recErr := governance.VerifyDecisionRecord(forgedRecord, nil, "", "")
	caseA := EvidenceTamperCase{Name: "record_verdict_flip", Target: "decision_record", Detected: recErr != nil}
	if recErr != nil {
		caseA.VerifyError = recErr.Error()
	}

	// Case B: break the receipt's binding by re-verifying it against a record
	// whose content has been altered (changing Tool alters the recomputed hash),
	// so VerifyBinding sees receipt.DecisionHash != ComputeDecisionHash(brokenRecord)
	// and rejects it.
	brokenRecord := record
	brokenRecord.Tool = record.Tool + "-tampered"
	rcErr := receipt.VerifyBinding(brokenRecord)
	caseB := EvidenceTamperCase{Name: "receipt_binding_break", Target: "proof_receipt", Detected: rcErr != nil}
	if rcErr != nil {
		caseB.VerifyError = rcErr.Error()
	}

	allInvariantsPass := true
	for _, inv := range invariants {
		if inv.Result != proofreceipt.ResultPass {
			allInvariantsPass = false
		}
	}

	conformance := []EvidenceConformanceItem{
		conformanceItem("write_denied_before_upstream", !result.Scenario.UpstreamCalled,
			fmt.Sprintf("upstream_called=%t", result.Scenario.UpstreamCalled)),
		conformanceItem("read_reached_upstream", result.Scenario.ReadUpstreamCalled,
			fmt.Sprintf("read_upstream_called=%t", result.Scenario.ReadUpstreamCalled)),
		conformanceItem("decision_mode_not_proved", record.DecisionMode != governance.DecisionModeProved,
			fmt.Sprintf("decision_mode=%q", record.DecisionMode)),
		conformanceItem("record_hash_verifies", recordVerified, "VerifyDecisionRecord ok"),
		conformanceItem("receipt_checker_verifies", receiptVerified, "ProofReceipt.VerifyBinding ok"),
	}

	passed := result.Passed && recordVerified && receiptVerified &&
		allInvariantsPass && caseA.Detected && caseB.Detected &&
		record.DecisionMode != governance.DecisionModeProved
	status := "pass"
	if !passed {
		status = "fail"
	}

	pack := &EvidencePack{
		SchemaVersion:       EvidencePackSchemaVersion,
		Status:              status,
		Passed:              passed,
		FixtureOnly:         true,
		RequiresCredentials: false,
		RequiresNetwork:     false,
		MutatesLiveSystems:  false,
		SecureGitHubStatus:  "preview",
		BypassLadderLevel:   "L0",
		DecisionRecord:      record,
		ProofReceipt:        receipt,
		ReceiptVerified:     receiptVerified,
		RecordVerified:      recordVerified,
		TamperCases:         []EvidenceTamperCase{caseA, caseB},
		RouteConformance:    conformance,
		Caveats: []string{
			"Fixture mode does not prove live GitHub App conformance (bypass ladder L0).",
			"Secure GitHub remains preview; this pack does not prove production deployment bypass resistance.",
			"Direct GitHub API or upstream MCP access remains a bypass unless operators remove those paths.",
			"The proof receipt is a checker-validated witness for the budget and static-privilege invariants, bound by decision_hash; it is not a `proved` decision mode.",
		},
	}
	return pack, nil
}

func conformanceItem(id string, ok bool, detail string) EvidenceConformanceItem {
	status := "pass"
	if !ok {
		status = "fail"
	}
	return EvidenceConformanceItem{ID: id, Status: status, Detail: detail}
}

// WriteEvidencePackJSON writes the pack as indented JSON (the machine-readable
// manifest). Plain output, no colorizer.
func WriteEvidencePackJSON(w io.Writer, pack *EvidencePack) error {
	if pack == nil {
		return fmt.Errorf("evidence pack is required")
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(pack)
}

// WriteEvidencePackDir writes the evidence pack to outDir as a fixture-safe,
// operator-pokable artifact set: pack.json (the manifest with per-artifact
// SHA-256), decision-record.json, proof-receipt.json, route-conformance.json,
// tamper-cases.json, caveats.md. It populates pack.Artifacts with the hashed
// manifest entries. Local files only — no credentials, no network, no live
// mutation. outDir must be non-empty.
func WriteEvidencePackDir(pack *EvidencePack, outDir string) error {
	if pack == nil {
		return fmt.Errorf("evidence pack is required")
	}
	if strings.TrimSpace(outDir) == "" {
		return fmt.Errorf("evidence pack output directory is required")
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return err
	}
	pack.Artifacts = nil

	writeJSON := func(name, kind string, payload any) error {
		body, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		body = append(body, '\n')
		return writePackArtifact(pack, outDir, name, kind, body)
	}

	if err := writeJSON("proof-receipt.json", "proof_receipt", pack.ProofReceipt); err != nil {
		return err
	}
	if err := writeJSON("decision-record.json", "decision_record", pack.DecisionRecord); err != nil {
		return err
	}
	if err := writeJSON("route-conformance.json", "route_conformance", pack.RouteConformance); err != nil {
		return err
	}
	if err := writeJSON("tamper-cases.json", "tamper_cases", pack.TamperCases); err != nil {
		return err
	}
	var caveats strings.Builder
	caveats.WriteString("# What this evidence pack does not prove\n\n")
	for _, c := range pack.Caveats {
		caveats.WriteString("- " + c + "\n")
	}
	if err := writePackArtifact(pack, outDir, "caveats.md", "caveats", []byte(caveats.String())); err != nil {
		return err
	}

	// The manifest is written last and lists every preceding artifact, so it is
	// not self-referential. It mirrors evidence.Manifest's fixture-safety fields
	// while carrying the pack-specific preview/L0 literals.
	manifest := struct {
		SchemaVersion       string             `json:"schema_version"`
		Status              string             `json:"status"`
		SecureGitHubStatus  string             `json:"secure_github_status"`
		BypassLadderLevel   string             `json:"bypass_ladder_level"`
		FixtureOnly         bool               `json:"fixture_only"`
		RequiresCredentials bool               `json:"requires_credentials"`
		RequiresNetwork     bool               `json:"requires_network"`
		MutatesLiveSystems  bool               `json:"mutates_live_systems"`
		Artifacts           []EvidenceArtifact `json:"artifacts"`
	}{
		SchemaVersion:       pack.SchemaVersion,
		Status:              pack.Status,
		SecureGitHubStatus:  pack.SecureGitHubStatus,
		BypassLadderLevel:   pack.BypassLadderLevel,
		FixtureOnly:         pack.FixtureOnly,
		RequiresCredentials: pack.RequiresCredentials,
		RequiresNetwork:     pack.RequiresNetwork,
		MutatesLiveSystems:  pack.MutatesLiveSystems,
		Artifacts:           pack.Artifacts,
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	// #nosec G306 -- pack.json is a fixture-safe local manifest in an operator-selected directory.
	return os.WriteFile(filepath.Join(outDir, "pack.json"), body, 0o600)
}

// writePackArtifact writes one fixture-safe artifact and records its hashed
// manifest entry on the pack.
func writePackArtifact(pack *EvidencePack, outDir, name, kind string, body []byte) error {
	// #nosec G306 -- evidence-pack artifacts are fixture-safe local files in an operator-selected directory.
	if err := os.WriteFile(filepath.Join(outDir, name), body, 0o600); err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	pack.Artifacts = append(pack.Artifacts, EvidenceArtifact{
		Path:      name,
		Kind:      kind,
		SHA256:    hex.EncodeToString(sum[:]),
		SizeBytes: int64(len(body)),
	})
	return nil
}
