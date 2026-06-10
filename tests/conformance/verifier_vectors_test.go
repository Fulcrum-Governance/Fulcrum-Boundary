// Package conformance holds the cross-implementation conformance gate for the
// Boundary decision-record decision_hash.
//
// This test owns a frozen corpus of decision records under
// testdata/verifier-vectors/. For every vector it asserts that the committed
// decision_hash equals what the real governance.ComputeDecisionHash recomputes
// from the same record. That assertion is pure Go and ALWAYS runs, so it pins
// the RFC 8785 / JCS canonical form of a decision record forever: any drift in
// the canonicalization, field set, or hashing makes this test fail in CI,
// independent of whether a Python (or any other) verifier is present.
//
// The same committed files are read by the standalone Python verifier's test
// (verifiers/python/test_boundary_verify.py). Because both implementations
// assert against this one committed corpus, the corpus is the shared source of
// truth that mechanically keeps the Go and Python verifiers in agreement —
// without either side shelling out to the other.
//
// To regenerate the corpus after an intentional, reviewed change to the record
// schema or canonical form, run:
//
//	BOUNDARY_WRITE_VECTORS=1 go test ./tests/conformance/ -run TestVerifierVectors -count=1
//
// and commit the updated testdata/verifier-vectors/ files. Without that env var
// the test never writes; it only verifies the committed bytes.
package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// vectorsDir is the committed corpus directory. The Python verifier test reads
// the exact same files.
const vectorsDir = "testdata/verifier-vectors"

// fixedTimestamp is a frozen RFC 3339 instant used for every vector so the
// corpus is deterministic (no time.Now()) and the committed decision_hash
// values are stable across regenerations.
const fixedTimestamp = "2026-06-01T04:36:39.787222Z"

// vector is one named corpus entry plus a short note on what canonical-form risk
// it exercises. The record's decision_hash and record_id are filled in by
// buildVectors using the real governance functions before the corpus is written.
type vector struct {
	// name is the corpus file stem; the file is name + ".json".
	name string
	// why documents the canonical-form property this vector pins.
	why string
	// record is the decision record, with decision_hash/record_id populated.
	record governance.DecisionRecordV1
}

// buildVectors constructs the frozen corpus in memory: one record per
// canonical-form risk, each finished by computing decision_hash with the real
// governance.ComputeDecisionHash and deriving a representative record_id. The
// records deliberately span both schema versions, every action type, the
// parse-rejection shape, an HTML-significant reason, and a non-trivial float
// trust_score.
func buildVectors(t *testing.T) []vector {
	t.Helper()

	ts := mustParseTime(t, fixedTimestamp)

	vs := []vector{
		{
			name: "v1_allow",
			why:  "schema_version 1 (no route-context); action=allow",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-allow",
				Tool:          "query",
				Action:        "allow",
				Reason:        "permitted by default-allow policy",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_deny",
			why:  "schema_version 1; action=deny",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-deny",
				Tool:          "github.create_or_update_file",
				Action:        "deny",
				Reason:        "protected private-repo write denied before upstream execution",
				DecisionMode:  governance.DecisionModeDeterministic,
				MatchedRule:   "deny-github-write-after-taint-fixture",
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_warn",
			why:  "schema_version 1; action=warn",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportCLI,
				AgentID:       "agent-warn",
				Tool:          "rm",
				Action:        "warn",
				Reason:        "destructive command flagged for review",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    0.75,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_escalate",
			why:  "schema_version 1; action=escalate",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-escalate",
				Tool:          "payments.transfer",
				Action:        "escalate",
				Reason:        "high-value transfer escalated to human reviewer",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    0.5,
				TrustState:    "EVALUATING",
			},
		},
		{
			name: "v1_require_approval",
			why:  "schema_version 1; action=require_approval",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-approval",
				Tool:          "deploy.production",
				Action:        "require_approval",
				Reason:        "production deploy requires approval",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_reason_html_chars",
			why:  "HTML-escape coverage: reason contains & < > which JCS keeps literal (Go's encoder would escape them)",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-html",
				Tool:          "query",
				Action:        "deny",
				Reason:        "blocked DROP TABLE & SELECT < 1 > 0",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_float_trust_score",
			why:  "ECMAScript number formatting: non-trivial float64 trust_score (1.0/3.0) must serialize as 0.3333333333333333",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-float",
				Tool:          "query",
				Action:        "warn",
				Reason:        "borderline trust score",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1.0 / 3.0,
				TrustState:    "EVALUATING",
			},
		},
		{
			name: "parse_rejection",
			why:  "parse_rejected shape: event_type=parse_rejected, raw_shape_hash set, request_hash absent; still hashes through ComputeDecisionHash",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "parse_rejected",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				Action:        "deny",
				Reason:        "malformed tools/call payload rejected before parsing",
				DecisionMode:  governance.DecisionModeDeterministic,
				RawShapeHash:  governance.ComputeRawShapeHash([]byte("{not valid json")),
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v2_route_context",
			why:  "schema_version 2: route-context fields populated and covered by decision_hash; includes execution_claim self-report",
			record: governance.DecisionRecordV1{
				SchemaVersion:   governance.DecisionRecordSchemaV2,
				EventType:       "governance_decision",
				Timestamp:       ts,
				Adapter:         governance.TransportMCP,
				AgentID:         "agent-v2",
				Tool:            "github.create_or_update_file",
				Action:          "deny",
				Reason:          "denied on routed path before upstream execution",
				DecisionMode:    governance.DecisionModeDeterministic,
				MatchedRule:     "deny-github-write-after-taint-fixture",
				RequestHash:     "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:      1,
				TrustState:      "TRUSTED",
				AdapterID:       "mcp-primary",
				RouteID:         "route-github-write",
				TopologyProfile: "single-route-forced",
				ExecutionClaim: &governance.ExecutionClaim{
					UpstreamCalled: false,
					Executed:       false,
					Source:         "mcp-adapter",
				},
			},
		},
	}

	// Finish each record with the real hashing function so the committed
	// decision_hash is exactly what a verifier must reproduce, and derive a
	// representative record_id from it (record_id is blanked before hashing, so
	// its value does not affect verification; this just mirrors real output).
	for i := range vs {
		vs[i].record.DecisionHash = governance.ComputeDecisionHash(vs[i].record)
		vs[i].record.RecordID = recordIDFromHash(vs[i].record.DecisionHash)
	}
	return vs
}

// TestVerifierVectors is the always-on cross-implementation gate. It
// (re)generates the frozen corpus only when BOUNDARY_WRITE_VECTORS=1, and on
// every run asserts that each committed vector's decision_hash equals what the
// real governance.ComputeDecisionHash recomputes. Drift fails CI.
func TestVerifierVectors(t *testing.T) {
	vs := buildVectors(t)

	if os.Getenv("BOUNDARY_WRITE_VECTORS") == "1" {
		writeCorpus(t, vs)
	}

	// Assert the committed bytes match the recomputed hashes. This reads what is
	// on disk (the committed corpus the Python verifier also reads), not the
	// in-memory build, so a stale or hand-edited corpus file is caught.
	committed := loadCommittedVectorFiles(t)
	if len(committed) != len(vs) {
		t.Fatalf("corpus drift: %d committed files, %d expected vectors; regenerate with BOUNDARY_WRITE_VECTORS=1", len(committed), len(vs))
	}

	for _, vec := range vs {
		t.Run(vec.name, func(t *testing.T) {
			raw, ok := committed[vec.name+".json"]
			if !ok {
				t.Fatalf("committed corpus missing %s.json; regenerate with BOUNDARY_WRITE_VECTORS=1", vec.name)
			}
			var rec governance.DecisionRecordV1
			if err := json.Unmarshal(raw, &rec); err != nil {
				t.Fatalf("decode committed %s.json: %v", vec.name, err)
			}
			if rec.DecisionHash == "" {
				t.Fatalf("committed %s.json has empty decision_hash", vec.name)
			}
			recomputed := governance.ComputeDecisionHash(rec)
			if recomputed != rec.DecisionHash {
				t.Fatalf("decision_hash drift for %s (%s)\n committed: %s\nrecomputed: %s\nregenerate with BOUNDARY_WRITE_VECTORS=1 after confirming the change is intended",
					vec.name, vec.why, rec.DecisionHash, recomputed)
			}
			// Sanity: the in-memory build that produced the committed file must
			// agree with the committed file's stored hash too, so the corpus is
			// not silently out of step with buildVectors.
			if vec.record.DecisionHash != rec.DecisionHash {
				t.Fatalf("in-memory vector and committed file disagree for %s\n in-memory: %s\ncommitted: %s\nregenerate with BOUNDARY_WRITE_VECTORS=1",
					vec.name, vec.record.DecisionHash, rec.DecisionHash)
			}
		})
	}

	// A forgery on the committed bytes must change the recomputed hash: flip
	// action allow->deny on a known-allow vector and confirm the digest moves.
	t.Run("forgery_changes_hash", func(t *testing.T) {
		raw, ok := committed["v1_allow.json"]
		if !ok {
			t.Skip("v1_allow.json not present")
		}
		var rec governance.DecisionRecordV1
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatalf("decode v1_allow.json: %v", err)
		}
		original := governance.ComputeDecisionHash(rec)
		rec.Action = "deny"
		forged := governance.ComputeDecisionHash(rec)
		if forged == original {
			t.Fatalf("forgery did not change decision_hash; hash is not covering action")
		}
		if forged == rec.DecisionHash {
			t.Fatalf("forged record's recomputed hash still equals the stored decision_hash; verification would not catch the edit")
		}
	})
}

// TestManifestMatchesCorpus asserts the committed manifest.json enumerates
// exactly the committed vector files, with the stored decision_hash for each.
// The Python verifier uses this manifest to discover vectors, so it must stay in
// lockstep with the corpus.
func TestManifestMatchesCorpus(t *testing.T) {
	manifestPath := filepath.Join(vectorsDir, "manifest.json")
	raw, err := os.ReadFile(manifestPath) // #nosec G304 -- fixed test-data path.
	if err != nil {
		t.Fatalf("read manifest: %v (regenerate with BOUNDARY_WRITE_VECTORS=1)", err)
	}
	var manifest corpusManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	committed := loadCommittedVectorFiles(t)
	if len(manifest.Vectors) != len(committed) {
		t.Fatalf("manifest lists %d vectors, corpus has %d files; regenerate with BOUNDARY_WRITE_VECTORS=1", len(manifest.Vectors), len(committed))
	}
	for _, entry := range manifest.Vectors {
		raw, ok := committed[entry.File]
		if !ok {
			t.Fatalf("manifest references missing file %s", entry.File)
		}
		var rec governance.DecisionRecordV1
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatalf("decode %s: %v", entry.File, err)
		}
		if entry.DecisionHash != rec.DecisionHash {
			t.Fatalf("manifest decision_hash for %s disagrees with file\n manifest: %s\n     file: %s", entry.File, entry.DecisionHash, rec.DecisionHash)
		}
	}
}

// corpusManifest is the committed manifest.json structure consumed by the Python
// verifier to enumerate the corpus.
type corpusManifest struct {
	// Description is a human note on what the corpus is for.
	Description string `json:"description"`
	// Vectors lists every committed vector file with its expected decision_hash
	// and a short note on the canonical-form property it pins.
	Vectors []manifestEntry `json:"vectors"`
}

type manifestEntry struct {
	// File is the corpus file name (relative to the corpus directory).
	File string `json:"file"`
	// DecisionHash is the expected stored decision_hash of that record.
	DecisionHash string `json:"decision_hash"`
	// Why documents the canonical-form risk the vector exercises.
	Why string `json:"why"`
}

// writeCorpus serializes every vector to its own pretty-printed JSON file under
// vectorsDir and writes manifest.json. It runs only under BOUNDARY_WRITE_VECTORS=1.
func writeCorpus(t *testing.T, vs []vector) {
	t.Helper()
	if err := os.MkdirAll(vectorsDir, 0o755); err != nil {
		t.Fatalf("mkdir corpus dir: %v", err)
	}
	manifest := corpusManifest{
		Description: "Frozen decision-record conformance corpus. Each file's decision_hash is " +
			"reproduced by governance.ComputeDecisionHash (Go) and by a stock RFC 8785 / JCS " +
			"verifier (see verifiers/python). Regenerate with BOUNDARY_WRITE_VECTORS=1.",
	}
	for _, vec := range vs {
		body, err := json.MarshalIndent(vec.record, "", "  ")
		if err != nil {
			t.Fatalf("marshal vector %s: %v", vec.name, err)
		}
		body = append(body, '\n')
		path := filepath.Join(vectorsDir, vec.name+".json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatalf("write vector %s: %v", vec.name, err)
		}
		manifest.Vectors = append(manifest.Vectors, manifestEntry{
			File:         vec.name + ".json",
			DecisionHash: vec.record.DecisionHash,
			Why:          vec.why,
		})
	}
	sort.Slice(manifest.Vectors, func(i, j int) bool {
		return manifest.Vectors[i].File < manifest.Vectors[j].File
	})
	mb, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	mb = append(mb, '\n')
	if err := os.WriteFile(filepath.Join(vectorsDir, "manifest.json"), mb, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Logf("wrote %d vectors + manifest.json to %s", len(vs), vectorsDir)
}

// loadCommittedVectorFiles reads every *.json vector file under vectorsDir
// (excluding manifest.json) and returns them keyed by file name.
func loadCommittedVectorFiles(t *testing.T) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(vectorsDir)
	if err != nil {
		t.Fatalf("read corpus dir %s: %v (regenerate with BOUNDARY_WRITE_VECTORS=1)", vectorsDir, err)
	}
	out := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || name == "manifest.json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(vectorsDir, name)) // #nosec G304 -- fixed test-data path.
		if err != nil {
			t.Fatalf("read corpus file %s: %v", name, err)
		}
		out[name] = raw
	}
	return out
}
