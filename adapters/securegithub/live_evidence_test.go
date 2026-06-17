package securegithub

import (
	"testing"
	"time"
)

func deniedWriteResult() LiveConformanceResult {
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
		TranscriptSHA256:     "f0f0",
	}
	return LiveConformanceResult{
		Transcript:       tr,
		TranscriptPath:   "/tmp/x/denied-write-after-taint.sanitized.json",
		TranscriptSHA256: "f0f0",
		DecisionRecord:   "abc123",
	}
}

func TestBuildLiveEvidenceIndexHashOnly(t *testing.T) {
	idx, err := BuildLiveEvidenceIndex([]LiveConformanceResult{deniedWriteResult()})
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
	if e.Mode != "denied-write-after-taint" || e.TranscriptSHA256 != "f0f0" || e.DecisionRecordHash != "abc123" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if e.UpstreamCalled || e.GitHubMutationCalled {
		t.Fatalf("denied-write entry must record no upstream/mutation: %+v", e)
	}
}

func TestBuildLiveEvidenceIndexRejectsUnsanitized(t *testing.T) {
	bad := deniedWriteResult()
	bad.Transcript.Sanitized = false
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{bad}); err == nil {
		t.Fatal("expected error for unsanitized transcript")
	}
	leak := deniedWriteResult()
	leak.Transcript.RawContentIncluded = true
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{leak}); err == nil {
		t.Fatal("expected error for raw-content-included transcript")
	}
	cred := deniedWriteResult()
	cred.Transcript.CredentialDataIncluded = true
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{cred}); err == nil {
		t.Fatal("expected error for credential-data-included transcript")
	}
}

func TestLiveEvidenceIndexLadderFactsL1(t *testing.T) {
	idx, err := BuildLiveEvidenceIndex([]LiveConformanceResult{deniedWriteResult()})
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
	r := deniedWriteResult()
	r.Transcript.GitHubMutationCalled = true
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
