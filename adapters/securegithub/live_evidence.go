package securegithub

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// LiveEvidenceSchemaVersion identifies the sanitized live-evidence index shape.
const LiveEvidenceSchemaVersion = "boundary.secure_github.live_evidence_index.v1"

// LiveEvidenceEntry is one sanitized live-conformance result, reduced to hashes
// and booleans. It never carries raw GitHub content or credentials.
type LiveEvidenceEntry struct {
	Mode                 string `json:"mode"`
	ExpectedAction       string `json:"expected_action"`
	ActualAction         string `json:"actual_action"`
	Reason               string `json:"reason"`
	MatchedRule          string `json:"matched_rule,omitempty"`
	UpstreamCalled       bool   `json:"upstream_called"`
	GitHubMutationCalled bool   `json:"github_mutation_called"`
	DecisionRecordHash   string `json:"decision_record_hash,omitempty"`
	TranscriptSHA256     string `json:"transcript_sha256"`
}

// LiveEvidenceIndex is the operator-owned, sanitized record that one or more
// live-conformance runs happened, identified by transcript hashes. It is the
// durable "live evidence recorded" artifact the bypass-proof packet and the
// ladder consume. It is hash-only by construction and asserts nothing about
// deployment bypass resistance.
type LiveEvidenceIndex struct {
	SchemaVersion string              `json:"schema_version"`
	ProfileID     string              `json:"profile_id"`
	ProfileStatus string              `json:"profile_status"`
	GeneratedAt   time.Time           `json:"generated_at"`
	Sanitized     bool                `json:"sanitized"`
	Entries       []LiveEvidenceEntry `json:"entries"`
}

// VerifyTranscriptFile reads a conformance transcript from disk and recomputes
// its digest from those bytes. It returns the decoded transcript and the
// recomputed digest, and fails closed when the path is empty, the file cannot be
// read or strictly decoded, carries no embedded digest, the embedded digest does
// not equal the recomputed one, or the bytes are not the canonical rendering the
// writer produces (so appended content or reordered fields are rejected).
//
// Scope: this detects an edit to the file relative to the digest the file
// itself carries, and (via BuildLiveEvidenceIndex) relative to the digest the
// caller holds. It does not authenticate who wrote the file: a party who can
// rewrite the file and every stored copy of its digest is not detected.
func VerifyTranscriptFile(path string) (LiveConformanceTranscript, string, error) {
	if strings.TrimSpace(path) == "" {
		return LiveConformanceTranscript{}, "", fmt.Errorf("transcript path is empty: digest cannot be recomputed")
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- path is the operator-selected conformance transcript path; reading it is the purpose of this verifier.
	if err != nil {
		return LiveConformanceTranscript{}, "", fmt.Errorf("read transcript for digest recomputation: %w", err)
	}
	var tr LiveConformanceTranscript
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tr); err != nil {
		return LiveConformanceTranscript{}, "", fmt.Errorf("decode transcript: %w", err)
	}
	embedded := tr.TranscriptSHA256
	if embedded == "" {
		return LiveConformanceTranscript{}, "", fmt.Errorf("transcript carries no embedded digest")
	}
	tr.TranscriptSHA256 = ""
	body, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		return LiveConformanceTranscript{}, "", err
	}
	sum := sha256.Sum256(body)
	recomputed := hex.EncodeToString(sum[:])
	if recomputed != embedded {
		return LiveConformanceTranscript{}, "", fmt.Errorf("transcript digest mismatch: embedded %s, recomputed %s", embedded, recomputed)
	}
	tr.TranscriptSHA256 = recomputed
	canonical, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		return LiveConformanceTranscript{}, "", err
	}
	if !bytes.Equal(canonical, raw) {
		return LiveConformanceTranscript{}, "", fmt.Errorf("transcript bytes are not the canonical rendering of the transcript")
	}
	return tr, recomputed, nil
}

func sameTranscript(a, b LiveConformanceTranscript) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ab, bb)
}

// BuildLiveEvidenceIndex reduces live-conformance results to a sanitized,
// hash-only index. It fails closed if any transcript is unsanitized, declares
// raw content or credential data, or serializes to bytes that look secret-like.
// Each entry is built from the transcript file on disk, never from the
// in-memory digest alone: the digest is recomputed from the file bytes, must
// equal every digest the caller supplied, and the in-memory transcript must
// equal the file's (FUL-708).
func BuildLiveEvidenceIndex(results []LiveConformanceResult) (LiveEvidenceIndex, error) {
	idx := LiveEvidenceIndex{
		SchemaVersion: LiveEvidenceSchemaVersion,
		ProfileID:     ProfileID,
		ProfileStatus: StatusPreview,
		GeneratedAt:   time.Now().UTC(),
		Sanitized:     true,
	}
	for _, r := range results {
		tr := r.Transcript
		if !tr.Sanitized {
			return LiveEvidenceIndex{}, fmt.Errorf("live evidence entry %q is not sanitized", tr.Mode)
		}
		if tr.RawContentIncluded || tr.CredentialDataIncluded {
			return LiveEvidenceIndex{}, fmt.Errorf("live evidence entry %q includes raw content or credential data", tr.Mode)
		}
		fileTr, digest, err := VerifyTranscriptFile(r.TranscriptPath)
		if err != nil {
			return LiveEvidenceIndex{}, fmt.Errorf("live evidence entry %q: %w", tr.Mode, err)
		}
		for _, supplied := range []string{r.TranscriptSHA256, tr.TranscriptSHA256} {
			if supplied != "" && supplied != digest {
				return LiveEvidenceIndex{}, fmt.Errorf("live evidence entry %q: supplied digest %s does not match recomputed %s", tr.Mode, supplied, digest)
			}
		}
		if !sameTranscript(tr, fileTr) {
			return LiveEvidenceIndex{}, fmt.Errorf("live evidence entry %q: in-memory transcript differs from the verified file", tr.Mode)
		}
		if !fileTr.Sanitized || fileTr.RawContentIncluded || fileTr.CredentialDataIncluded {
			return LiveEvidenceIndex{}, fmt.Errorf("live evidence entry %q: transcript file is not sanitized or declares raw/credential data", tr.Mode)
		}
		// The decision hash earns ladder credit, so it must come from the verified
		// file. A caller-supplied hash that the file does not carry, or that
		// disagrees with it, is rejected rather than trusted.
		if r.DecisionRecord != "" && r.DecisionRecord != fileTr.DecisionRecordHash {
			return LiveEvidenceIndex{}, fmt.Errorf("live evidence entry %q: supplied decision record hash is not the one in the verified transcript", tr.Mode)
		}
		tr = fileTr
		idx.Entries = append(idx.Entries, LiveEvidenceEntry{
			Mode:                 tr.Mode,
			ExpectedAction:       tr.ExpectedAction,
			ActualAction:         tr.ActualAction,
			Reason:               tr.Reason,
			MatchedRule:          tr.MatchedRule,
			UpstreamCalled:       tr.UpstreamCalled,
			GitHubMutationCalled: tr.GitHubMutationCalled,
			DecisionRecordHash:   tr.DecisionRecordHash,
			TranscriptSHA256:     digest,
		})
	}
	body, err := json.Marshal(idx)
	if err != nil {
		return LiveEvidenceIndex{}, err
	}
	if containsSecretLikeData(string(body)) {
		return LiveEvidenceIndex{}, fmt.Errorf("refusing to build Secure GitHub live evidence index with secret-like data")
	}
	return idx, nil
}

// LadderFacts derives the L1 ladder facts from the indexed denied-write entry. It
// returns zero-value (L0) facts when no denied-write entry proves the no-mutation
// path. It never sets L2 facts: those are operator-attested and are not derivable
// from routed evidence.
func (idx LiveEvidenceIndex) LadderFacts() LadderFacts {
	var f LadderFacts
	for _, e := range idx.Entries {
		if e.Mode != "denied-write-after-taint" {
			continue
		}
		if e.ActualAction != "DENY" {
			continue
		}
		f.LiveDeniedWriteRecorded = true
		f.LiveNoMutationProven = !e.UpstreamCalled && !e.GitHubMutationCalled
		f.TranscriptSanitized = idx.Sanitized
		f.DecisionRecordHashPresent = e.DecisionRecordHash != ""
	}
	return f
}
