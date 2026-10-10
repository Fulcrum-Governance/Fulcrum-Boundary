package managedagentsconformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCheckBudgetRequiresObservedUsage locks in the fail-closed budget rule:
// a recorded Used of 0 is not evidence unless upstream usage actually
// arrived, and spend marked unknown can never satisfy the criterion.
func TestCheckBudgetRequiresObservedUsage(t *testing.T) {
	tr := Transcript{Budget: budgetEvidence{Ceiling: 5.0, Used: 0}}
	if err := CheckBudget(tr); err == nil {
		t.Fatal("used=0 without usage_observed must fail")
	}
	tr.Budget.UsageObserved = true
	tr.Budget.SpendUnknown = true
	if err := CheckBudget(tr); err == nil {
		t.Fatal("spend_unknown must fail even with usage_observed")
	}
	tr.Budget.SpendUnknown = false
	if err := CheckBudget(tr); err != nil {
		t.Fatalf("observed usage inside ceiling must pass: %v", err)
	}
}

// writeTranscriptWithDigest serializes tr exactly the way the session driver
// does: transcript_sha256 is computed over the MarshalIndent bytes with the
// hash field absent, then the document (hash included) is written. It
// returns the path and the recorded digest.
func writeTranscriptWithDigest(t *testing.T, dir string, tr Transcript) (string, string) {
	t.Helper()
	tr.TranscriptHash = ""
	unsigned, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(unsigned)
	digest := hex.EncodeToString(sum[:])
	tr.TranscriptHash = digest
	final, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	trPath := filepath.Join(dir, "transcript.sanitized.json")
	if err := os.WriteFile(trPath, append(final, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return trPath, digest
}

// writeEvidenceSet fabricates a consistent transcript + provenance.json +
// raw event log under a temp dir and returns the parsed transcript and its
// path. mode controls both documents' mode fields.
func writeEvidenceSet(t *testing.T, mode string) (Transcript, string) {
	t.Helper()
	dir := t.TempDir()

	raw := []byte(`{"kind":"event","data":{"type":"session.status_running"}}` + "\n")
	rawSum := sha256.Sum256(raw)
	rawHash := hex.EncodeToString(rawSum[:])
	if err := os.WriteFile(filepath.Join(dir, "events.raw.jsonl"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	tr := Transcript{
		Sanitized:      true,
		Mode:           mode,
		SessionCreated: true,
		SessionID:      "sess-fake0001",
		Provenance: &provenanceLinkage{
			Mode:         mode,
			SessionID:    "sess-fake0001",
			RawLogSHA256: rawHash,
		},
	}
	trPath, digest := writeTranscriptWithDigest(t, dir, tr)
	tr.TranscriptHash = digest

	prov := provenanceFile{
		Mode:                mode,
		TranscriptSessionID: "sess-fake0001",
		TranscriptSHA256:    digest,
		RawLogFile:          "events.raw.jsonl",
		RawLogSHA256:        rawHash,
	}
	provBytes, err := json.Marshal(prov)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "provenance.json"), provBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return tr, trPath
}

func TestCheckLiveProvenanceAcceptsConsistentLiveSet(t *testing.T) {
	tr, path := writeEvidenceSet(t, "live")
	if err := CheckLiveProvenance(tr, path); err != nil {
		t.Fatalf("consistent live evidence set must pass: %v", err)
	}
}

func TestCheckLiveProvenanceRejectsStubSet(t *testing.T) {
	tr, path := writeEvidenceSet(t, "stub")
	if err := CheckLiveProvenance(tr, path); err == nil {
		t.Fatal("a stub evidence set must be rejected as live evidence")
	}
}

// TestCheckLiveProvenanceRejectsStubProvenance covers the accident case: a
// mode=live transcript sitting next to a stub provenance file.
func TestCheckLiveProvenanceRejectsStubProvenance(t *testing.T) {
	tr, path := writeEvidenceSet(t, "live")
	provPath := filepath.Join(filepath.Dir(path), "provenance.json")
	data, err := os.ReadFile(provPath)
	if err != nil {
		t.Fatal(err)
	}
	var prov map[string]any
	if err := json.Unmarshal(data, &prov); err != nil {
		t.Fatal(err)
	}
	prov["mode"] = "stub"
	out, err := json.Marshal(prov)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provPath, out, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckLiveProvenance(tr, path); err == nil {
		t.Fatal("stub provenance must be rejected for a live transcript")
	}
}

func TestCheckLiveProvenanceRejectsRawLogMismatch(t *testing.T) {
	tr, path := writeEvidenceSet(t, "live")
	rawPath := filepath.Join(filepath.Dir(path), "events.raw.jsonl")
	if err := os.WriteFile(rawPath, []byte("tampered contents\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckLiveProvenance(tr, path); err == nil {
		t.Fatal("a raw log that does not match the recorded hash must be rejected")
	}
}

func TestCheckLiveProvenanceRejectsSessionMismatch(t *testing.T) {
	tr, path := writeEvidenceSet(t, "live")
	tr.SessionID = "sess-different"
	if err := CheckLiveProvenance(tr, path); err == nil {
		t.Fatal("session id disagreement must be rejected")
	}
}

func TestCheckLiveProvenanceRejectsMissingProvenance(t *testing.T) {
	dir := t.TempDir()
	tr := Transcript{
		Mode:      "live",
		SessionID: "sess-fake0001",
		Provenance: &provenanceLinkage{
			Mode:      "live",
			SessionID: "sess-fake0001",
		},
	}
	trPath, digest := writeTranscriptWithDigest(t, dir, tr)
	tr.TranscriptHash = digest
	if err := CheckLiveProvenance(tr, trPath); err == nil {
		t.Fatal("a live transcript without provenance.json must be rejected")
	}
}

// TestCheckLiveProvenanceRejectsModifiedTranscript proves the recorded
// transcript digest is recomputed against the bytes on disk: a transcript
// file altered after its hash was recorded must be rejected even though the
// stored hashes in the transcript and provenance.json still agree.
func TestCheckLiveProvenanceRejectsModifiedTranscript(t *testing.T) {
	tr, path := writeEvidenceSet(t, "live")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	// Alter evidence the linkage checks do not otherwise inspect, leaving
	// the recorded transcript_sha256 untouched: only a recomputed digest
	// can catch this change.
	doc["agent_id"] = "agent-tampered"
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckLiveProvenance(tr, path); err == nil {
		t.Fatal("a transcript modified after its hash was recorded must be rejected")
	}
}

// TestCheckLiveProvenanceRejectsAbsentTranscriptHash pins the explicit
// fail-closed decision: a live evidence set missing the transcript digest in
// either document cannot be verified and is rejected.
func TestCheckLiveProvenanceRejectsAbsentTranscriptHash(t *testing.T) {
	t.Run("transcript hash absent", func(t *testing.T) {
		tr, path := writeEvidenceSet(t, "live")
		tr.TranscriptHash = ""
		if err := CheckLiveProvenance(tr, path); err == nil {
			t.Fatal("a live transcript carrying no transcript_sha256 must be rejected")
		}
	})
	t.Run("provenance hash absent", func(t *testing.T) {
		tr, path := writeEvidenceSet(t, "live")
		provPath := filepath.Join(filepath.Dir(path), "provenance.json")
		data, err := os.ReadFile(provPath)
		if err != nil {
			t.Fatal(err)
		}
		var prov map[string]any
		if err := json.Unmarshal(data, &prov); err != nil {
			t.Fatal(err)
		}
		delete(prov, "transcript_sha256")
		out, err := json.Marshal(prov)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(provPath, out, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := CheckLiveProvenance(tr, path); err == nil {
			t.Fatal("a provenance.json carrying no transcript_sha256 must be rejected")
		}
	})
}
