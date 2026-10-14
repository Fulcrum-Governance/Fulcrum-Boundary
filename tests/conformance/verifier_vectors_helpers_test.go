package conformance

import (
	"fmt"
	"regexp"
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

// escapeAllForJSON returns s with every code point spelled as a JSON \uXXXX
// escape (or a surrogate pair for astral code points) — the maximally
// non-literal spelling of the same string, used to build corpus bytes that
// exercise escape decoding in every verifier.
func escapeAllForJSON(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 0xFFFF {
			r -= 0x10000
			fmt.Fprintf(&b, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
			continue
		}
		fmt.Fprintf(&b, `\u%04x`, r)
	}
	return b.String()
}

// storedHashPattern matches the record's stored "decision_hash" member in raw
// file bytes, so a value can be read even from files a strict decoder rejects
// (duplicate members, trailing data).
var storedHashPattern = regexp.MustCompile(`"decision_hash"\s*:\s*"(sha256:[0-9a-f]{64})"`)

// storedDecisionHash extracts the decision_hash a vector file stores, without
// decoding the file. It fails the test when the file stores no well-formed
// sha256-prefixed hash.
func storedDecisionHash(t *testing.T, raw []byte, name string) string {
	t.Helper()
	match := storedHashPattern.FindSubmatch(raw)
	if match == nil {
		t.Fatalf("%s has no stored decision_hash of the form sha256:<64 hex>", name)
	}
	return string(match[1])
}
