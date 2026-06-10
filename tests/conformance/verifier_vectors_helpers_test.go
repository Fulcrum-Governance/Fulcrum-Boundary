package conformance

import (
	"strings"
	"testing"
	"time"
)

// mustParseTime parses an RFC 3339 timestamp for the frozen corpus, failing the
// test on any error. Using a fixed parsed instant (rather than time.Now) keeps
// the corpus deterministic so committed decision_hash values are stable.
func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatalf("parse fixed timestamp %q: %v", value, err)
	}
	return ts.UTC()
}

// recordIDFromHash derives a representative record_id from a decision_hash, in
// the same shape Boundary uses (rec_ + first 12 hex chars after the sha256:
// prefix). record_id is blanked before hashing, so its exact value never affects
// verification; this only makes the committed corpus look like real output.
func recordIDFromHash(hash string) string {
	trimmed := strings.TrimPrefix(hash, "sha256:")
	if len(trimmed) < 12 {
		return "rec_" + trimmed
	}
	return "rec_" + trimmed[:12]
}
