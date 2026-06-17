package securegithub

import (
	"encoding/json"
	"fmt"
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

// BuildLiveEvidenceIndex reduces live-conformance results to a sanitized,
// hash-only index. It fails closed if any transcript is unsanitized, declares
// raw content or credential data, or serializes to bytes that look secret-like.
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
		idx.Entries = append(idx.Entries, LiveEvidenceEntry{
			Mode:                 tr.Mode,
			ExpectedAction:       tr.ExpectedAction,
			ActualAction:         tr.ActualAction,
			Reason:               tr.Reason,
			MatchedRule:          tr.MatchedRule,
			UpstreamCalled:       tr.UpstreamCalled,
			GitHubMutationCalled: tr.GitHubMutationCalled,
			DecisionRecordHash:   firstNonEmpty(tr.DecisionRecordHash, r.DecisionRecord),
			TranscriptSHA256:     firstNonEmpty(tr.TranscriptSHA256, r.TranscriptSHA256),
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
