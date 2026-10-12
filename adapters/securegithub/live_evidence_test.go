package securegithub

import (
	"testing"
	"time"
)

// deniedWriteResult returns a genuinely written transcript (real file, real
// digest) so the evidence index can recompute the digest from disk (FUL-708).
func deniedWriteResult(t *testing.T) LiveConformanceResult {
	t.Helper()
	return deniedWriteResultWith(t, nil)
}

// deniedWriteResultWith applies mutate to the transcript BEFORE it is written,
// so the file and its digest stay consistent with the mutated facts.
func deniedWriteResultWith(t *testing.T, mutate func(*LiveConformanceTranscript)) LiveConformanceResult {
	t.Helper()
	tr := LiveConformanceTranscript{
		SchemaVersion:        LiveConformanceSchemaVersion,
		Sanitized:            true,
		Mode:                 "denied-write-after-taint",
		GeneratedAt:          time.Now().UTC(),
		ProfileID:            ProfileID,
		ProfileStatus:        StatusPreview,
		Owner:                "fixture-org",
		Repo:                 "fixture-private-repo",
		IssueNumber:          7,
		ExpectedAction:       "DENY",
		ActualAction:         "DENY",
		Reason:               LiveConformanceReason,
		MatchedRule:          "deny-github-write-after-taint-fixture",
		UpstreamCalled:       false,
		GitHubMutationCalled: false,
		DecisionRecordHash:   "abc123",
	}
	if mutate != nil {
		mutate(&tr)
	}
	res, err := writeConformanceTranscript(
		LiveConfig{TranscriptDir: t.TempDir()}, tr, "denied-write-after-taint.sanitized.json")
	if err != nil {
		t.Fatalf("writeConformanceTranscript: %v", err)
	}
	return res
}

func TestBuildLiveEvidenceIndexHashOnly(t *testing.T) {
	res := deniedWriteResult(t)
	idx, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res})
	if err != nil {
		t.Fatalf("BuildLiveEvidenceIndex: %v", err)
	}
	if idx.SchemaVersion != LiveEvidenceSchemaVersion {
		t.Fatalf("schema = %q", idx.SchemaVersion)
	}
	if idx.ProfileStatus != StatusPreview {
		t.Fatalf("profile status = %q, want preview", idx.ProfileStatus)
	}
	if len(idx.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(idx.Entries))
	}
	e := idx.Entries[0]
	if e.Mode != "denied-write-after-taint" || e.TranscriptSHA256 != res.TranscriptSHA256 || len(e.TranscriptSHA256) != 64 || e.DecisionRecordHash != "abc123" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if e.UpstreamCalled || e.GitHubMutationCalled {
		t.Fatalf("denied-write entry must record no upstream/mutation: %+v", e)
	}
}

func TestBuildLiveEvidenceIndexRejectsUnsanitized(t *testing.T) {
	bad := deniedWriteResult(t)
	bad.Transcript.Sanitized = false
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{bad}); err == nil {
		t.Fatal("expected error for unsanitized transcript")
	}
	leak := deniedWriteResult(t)
	leak.Transcript.RawContentIncluded = true
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{leak}); err == nil {
		t.Fatal("expected error for raw-content-included transcript")
	}
	cred := deniedWriteResult(t)
	cred.Transcript.CredentialDataIncluded = true
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{cred}); err == nil {
		t.Fatal("expected error for credential-data-included transcript")
	}
}

func TestLiveEvidenceIndexLadderFactsL1(t *testing.T) {
	idx, err := BuildLiveEvidenceIndex([]LiveConformanceResult{deniedWriteResult(t)})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	f := idx.LadderFacts()
	if !f.LiveDeniedWriteRecorded || !f.LiveNoMutationProven || !f.TranscriptSanitized || !f.DecisionRecordHashPresent {
		t.Fatalf("L1 facts not derived: %+v", f)
	}
	// The L2 facts must NOT be derivable from routed evidence.
	if f.AgentHasNoDirectToken || f.AppCredentialRuntimeOnly || f.UpstreamMCPUnavailable ||
		f.NoUnmanagedGitOrGH || f.EgressPolicyEnforced {
		t.Fatalf("L2 facts must stay false (operator-attested only): %+v", f)
	}
	level, _ := ClassifyBypassLevel(f)
	if level != BypassL1 {
		t.Fatalf("indexed denied-write evidence classified as %s, want L1", level)
	}
}

func TestLiveEvidenceIndexLadderFactsNotL1WhenMutationCalled(t *testing.T) {
	r := deniedWriteResultWith(t, func(tr *LiveConformanceTranscript) { tr.GitHubMutationCalled = true })
	idx, err := BuildLiveEvidenceIndex([]LiveConformanceResult{r})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	f := idx.LadderFacts()
	if f.LiveNoMutationProven {
		t.Fatal("no-mutation must be false when the transcript reports a mutation call")
	}
	level, _ := ClassifyBypassLevel(f)
	if level != BypassL0 {
		t.Fatalf("a mutation-called transcript must not reach L1; got %s", level)
	}
}
