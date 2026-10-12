package securegithub

// FUL-708: BuildLiveEvidenceIndex must not copy a transcript digest it never
// checked. These tests pin the contract that the index is built from the
// transcript bytes on disk: the digest is recomputed from the file, and an
// entry whose file is missing, edited, or disagrees with the in-memory result
// is rejected. Same standard as the #231 fix: every positive case has a
// negative control that fails on the pre-fix code.
//
// Scope note (claims discipline): recomputation detects an edit only while the
// expected digest comes from somewhere the editor cannot also rewrite. A caller
// who rewrites the file and every copy of its digest is not detected here.

import (
	"os"
	"strings"
	"testing"
)

func writtenDeniedWriteResult(t *testing.T) LiveConformanceResult {
	t.Helper()
	return deniedWriteResult(t)
}

func rewriteTranscriptFile(t *testing.T, path, old, repl string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("fixture drift: %q not in transcript file", old)
	}
	out := strings.Replace(string(b), old, repl, 1)
	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Positive control: an untouched, genuinely written transcript is accepted and
// the entry carries the digest recomputed from the file.
func TestLiveEvidenceIndexAcceptsUntamperedTranscript(t *testing.T) {
	res := writtenDeniedWriteResult(t)
	idx, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res})
	if err != nil {
		t.Fatalf("untampered transcript must be accepted: %v", err)
	}
	if len(idx.Entries) != 1 || idx.Entries[0].TranscriptSHA256 != res.TranscriptSHA256 {
		t.Fatalf("entry digest = %+v, want %s", idx.Entries, res.TranscriptSHA256)
	}
}

// Negative control: edit a recorded fact in the on-disk file after it was
// written. The in-memory result still carries the original digest, which the
// current code copies without reading the file.
func TestLiveEvidenceIndexRejectsEditedTranscriptFile(t *testing.T) {
	res := writtenDeniedWriteResult(t)
	rewriteTranscriptFile(t, res.TranscriptPath, `"actual_action": "DENY"`, `"actual_action": "ALLOW"`)
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res}); err == nil {
		t.Fatal("index accepted a transcript whose bytes no longer match its digest")
	}
}

// Negative control: the file was edited and the embedded transcript_sha256 line
// was left stale. The recomputed digest must not equal the embedded one.
func TestLiveEvidenceIndexRejectsStaleEmbeddedDigest(t *testing.T) {
	res := writtenDeniedWriteResult(t)
	rewriteTranscriptFile(t, res.TranscriptPath, `"upstream_called": false`, `"upstream_called": true`)
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res}); err == nil {
		t.Fatal("index accepted a transcript with a stale embedded digest")
	}
}

// Negative control: the transcript file is gone, so nothing can be recomputed.
func TestLiveEvidenceIndexRejectsMissingTranscriptFile(t *testing.T) {
	res := writtenDeniedWriteResult(t)
	if err := os.Remove(res.TranscriptPath); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res}); err == nil {
		t.Fatal("index accepted an entry whose transcript file does not exist")
	}
}

// Negative control: no path at all is unverifiable, not trusted.
func TestLiveEvidenceIndexRejectsEmptyTranscriptPath(t *testing.T) {
	res := writtenDeniedWriteResult(t)
	res.TranscriptPath = ""
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res}); err == nil {
		t.Fatal("index accepted an entry with no transcript path to verify")
	}
}

// Negative control: the caller-supplied digest is not the file's digest. This is
// the case where a forged digest is attached to a real file.
func TestLiveEvidenceIndexRejectsForgedResultDigest(t *testing.T) {
	res := writtenDeniedWriteResult(t)
	forged := strings.Repeat("0", 64)
	res.TranscriptSHA256 = forged
	res.Transcript.TranscriptSHA256 = forged
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res}); err == nil {
		t.Fatal("index accepted a digest that does not match the file bytes")
	}
}

// Negative control: the in-memory transcript disagrees with the file on a fact
// the ladder consumes. Facts must come from the verified file, so a disagreement
// is rejected rather than silently preferring either side.
func TestLiveEvidenceIndexRejectsInMemoryFileMismatch(t *testing.T) {
	res := writtenDeniedWriteResult(t)
	res.Transcript.GitHubMutationCalled = true
	if _, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res}); err == nil {
		t.Fatal("index accepted an in-memory transcript that differs from the file")
	}
}

// The ladder must not report a recorded no-mutation proof built on an edited
// transcript. This pins the downstream consequence of the same defect.
func TestLadderFactsNotDerivedFromEditedTranscript(t *testing.T) {
	res := writtenDeniedWriteResult(t)
	rewriteTranscriptFile(t, res.TranscriptPath, `"github_mutation_called": false`, `"github_mutation_called": true`)
	idx, err := BuildLiveEvidenceIndex([]LiveConformanceResult{res})
	if err == nil && idx.LadderFacts().LiveNoMutationProven {
		t.Fatal("ladder reports LiveNoMutationProven from a transcript edited after writing")
	}
}
