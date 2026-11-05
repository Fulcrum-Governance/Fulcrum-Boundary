package verifier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// vectorsDir returns the pinned copy of Boundary's decision-record conformance
// corpus (tests/conformance/testdata/verifier-vectors). A root-module test
// fails if this copy diverges from Boundary's, so running every vector here
// proves this verifier agrees with Boundary's own verdicts on the same bytes.
func vectorsDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "testdata", "verifier-vectors"))
}

type vectorManifest struct {
	Vectors []struct {
		File         string `json:"file"`
		DecisionHash string `json:"decision_hash"`
		Expect       string `json:"expect"`
		Why          string `json:"why"`
	} `json:"vectors"`
}

// TestConformanceVectors runs EVERY vector in the pinned corpus through the
// verifier's decision-record decode and hash recomputation and asserts the
// manifest's expected verdict:
//
//	verify               -> decodes; recomputed decision_hash == decision_hash
//	reject:hash-mismatch -> decodes; recomputed decision_hash != decision_hash
//	reject:<class>       -> decode fails with that class
func TestConformanceVectors(t *testing.T) {
	dir := vectorsDir(t)
	manifestPath := filepath.Join(dir, "manifest.json")
	raw, err := os.ReadFile(manifestPath) // #nosec G304 -- fixed test-data path.
	if err != nil {
		t.Fatalf("read corpus manifest: %v", err)
	}
	var manifest vectorManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode corpus manifest: %v", err)
	}
	if len(manifest.Vectors) == 0 {
		t.Fatal("corpus manifest lists no vectors")
	}

	// Every vector file on disk must be listed, and every listed file must
	// exist: a vector that is silently skipped cannot protect anything.
	listed := map[string]bool{}
	for _, vec := range manifest.Vectors {
		listed[vec.File] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}
	var onDisk []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "manifest.json" {
			continue
		}
		onDisk = append(onDisk, entry.Name())
		if !listed[entry.Name()] {
			t.Errorf("corpus file %s is not listed in manifest.json", entry.Name())
		}
	}
	sort.Strings(onDisk)
	if len(onDisk) != len(manifest.Vectors) {
		t.Errorf("corpus has %d files, manifest lists %d vectors", len(onDisk), len(manifest.Vectors))
	}

	counts := map[string]int{}
	for _, vec := range manifest.Vectors {
		counts[vec.Expect]++
		t.Run(strings.TrimSuffix(vec.File, ".json"), func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(dir, vec.File)) // #nosec G304 -- fixed test-data path.
			if err != nil {
				t.Fatalf("read vector: %v", err)
			}
			record, decodeErr := decodeDecisionRecord(body)

			switch {
			case vec.Expect == "verify":
				if decodeErr != nil {
					t.Fatalf("verifier rejected a record Boundary verifies (%s): %v", vec.Why, decodeErr)
				}
				if record.DecisionHash != vec.DecisionHash {
					t.Fatalf("decoded decision_hash %s != manifest %s", record.DecisionHash, vec.DecisionHash)
				}
				got, err := computeDecisionHash(record)
				if err != nil {
					t.Fatalf("compute decision_hash: %v", err)
				}
				if got != record.DecisionHash {
					t.Fatalf("recomputed decision_hash %s != stored %s (%s)", got, record.DecisionHash, vec.Why)
				}
			case vec.Expect == "reject:hash-mismatch":
				if decodeErr != nil {
					t.Fatalf("verifier rejected a well-formed tampered record at decode (%s): %v", vec.Why, decodeErr)
				}
				got, err := computeDecisionHash(record)
				if err != nil {
					t.Fatalf("compute decision_hash: %v", err)
				}
				if got == record.DecisionHash {
					t.Fatalf("recomputed decision_hash still matches a tampered record (%s)", vec.Why)
				}
			case strings.HasPrefix(vec.Expect, "reject:"):
				want := strings.TrimPrefix(vec.Expect, "reject:")
				if decodeErr == nil {
					t.Fatalf("verifier accepted a record the corpus requires rejected as %s (%s)", want, vec.Why)
				}
				if got := recordRejectReason(decodeErr); got != want {
					t.Fatalf("rejected as %s, corpus expects %s (%s): %v", got, want, vec.Why, decodeErr)
				}
			default:
				t.Fatalf("unrecognized manifest expect %q", vec.Expect)
			}
		})
	}

	// Pin the corpus shape so an accidentally truncated copy cannot pass.
	if counts["verify"] == 0 || counts["reject:hash-mismatch"] == 0 {
		t.Errorf("corpus lacks verify or hash-mismatch vectors: %v", counts)
	}
}

// TestRecordMemberSetsAreClosed pins the closed member sets the decoder
// enforces, so adding or removing a record member is a deliberate, reviewed
// change.
func TestRecordMemberSetsAreClosed(t *testing.T) {
	wantRecord := []string{
		"schema_version", "event_type", "record_id", "timestamp", "boundary_version",
		"boundary_build_digest", "adapter", "agent_id", "tenant_id", "trace_id",
		"request_id", "tool", "action", "reason", "decision_mode", "matched_rule",
		"policy_file", "policy_bundle_hash", "request_hash", "raw_shape_hash",
		"decision_hash", "trust_score", "trust_state", "signature", "signature_key_id",
		"check", "adapter_id", "route_id", "topology_profile", "execution_claim",
	}
	assertSet(t, "record", recordMemberSet, wantRecord)
	assertSet(t, "check", checkMemberSet, []string{"stage", "class", "category", "cause"})
	assertSet(t, "execution_claim", executionClaimMemberSet, []string{"upstream_called", "executed", "source"})
}

func assertSet(t *testing.T, label string, got map[string]struct{}, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s member set has %d members, want %d", label, len(got), len(want))
	}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("%s member set is missing %q", label, name)
		}
	}
}
