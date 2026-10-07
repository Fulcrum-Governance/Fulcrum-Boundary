package governance

import (
	"errors"
	"testing"
)

// TestDecodeDecisionRecord_StrictIngest exercises the ingest contract that the
// cross-language verifier corpus also pins through committed files: duplicate
// member names and trailing bytes are rejected with their shared reason class,
// non-object input is rejected, and well-formed records decode.
func TestDecodeDecisionRecord_StrictIngest(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		reason string
	}{
		{"top-level duplicate key", `{"a": 1, "a": 2}`, RecordRejectDuplicateKey},
		{"nested duplicate key", `{"a": {"b": 1, "b": 2}}`, RecordRejectDuplicateKey},
		{"duplicate key inside array element", `{"a": [{"x": 1, "x": 2}]}`, RecordRejectDuplicateKey},
		{"escaped duplicate key", `{"a": 1, "a": 2}`, RecordRejectDuplicateKey},
		{"trailing object", `{"a": 1} {"b": 2}`, RecordRejectTrailingData},
		{"trailing garbage", `{"a": 1} garbage`, RecordRejectTrailingData},
		{"array is not a record", `[{"a": 1}]`, RecordRejectNotJSONObject},
		{"scalar is not a record", `42`, RecordRejectNotJSONObject},
		{"null is not a record", `null`, RecordRejectNotJSONObject},
		// Empty (or whitespace-only) input is a parse error, matching the
		// Python/TypeScript/Rust verifiers: there is no top-level value at all,
		// so "not a JSON object" never gets reached.
		{"empty input", ``, RecordRejectParse},
		{"malformed json", `{"a":`, RecordRejectParse},
		// The record's member set is closed: member names the schema does
		// not define are rejected at every object position, not dropped.
		{"unknown top-level member", `{"action": "deny", "smuggled": 1}`, RecordRejectUnknownField},
		{"unknown member holding an object", `{"action": "deny", "extra": {"nested": 1}}`, RecordRejectUnknownField},
		{"unknown execution_claim member", `{"action": "deny", "execution_claim": {"upstream_called": true, "extra": 1}}`, RecordRejectUnknownField},
		// A duplicate member name is still classified duplicate-key even
		// when the member itself is unknown: the ambiguity check runs
		// before the schema check. Uniqueness is case-folded (mirroring the
		// Python, TypeScript, and Rust verifiers), so "zz" and "ZZ" collide.
		{"duplicate unknown member", `{"zz": 1, "zz": 2}`, RecordRejectDuplicateKey},
		{"case-folded duplicate unknown member", `{"zz": 1, "ZZ": 2}`, RecordRejectDuplicateKey},
		// encoding/json binds member names case-insensitively, so without
		// the exact-case scan "ACTION" would alias action and "Action" would
		// silently overwrite "action". The closed member set is exact-case:
		// a lone case variant is unknown-field; a variant colliding with a
		// declared name already present is duplicate-key. Both match the
		// other three verifiers.
		{"case-variant declared member", `{"ACTION": "deny"}`, RecordRejectUnknownField},
		{"case-variant member colliding with declared", `{"action": "deny", "Action": "allow"}`, RecordRejectDuplicateKey},
		{"case-variant member inside execution_claim", `{"action": "deny", "execution_claim": {"upstream_called": true, "EXECUTED": false}}`, RecordRejectUnknownField},
		// The folded duplicate check also runs at object positions the
		// schema does not constrain, matching the other verifiers (whose
		// uniqueness hooks run at every depth).
		{"case-folded duplicate at unconstrained depth", `{"action": "deny", "reason": {"X": 1, "x": 2}}`, RecordRejectDuplicateKey},
		// A member-name problem never outranks a structural failure: with a
		// duplicate later in the stream the class stays duplicate-key, and
		// with garbage after the value it stays trailing-data — the same
		// ordering the standalone verifiers produce (uniqueness is checked
		// during the parse, the member set after it).
		{"unknown member then duplicate", `{"ZZ": 1, "action": "deny", "action": "allow"}`, RecordRejectDuplicateKey},
		{"unknown member then trailing data", `{"ZZ": 1} tail`, RecordRejectTrailingData},
		// NaN/Infinity are not JSON; encoding/json rejects them outright.
		{"NaN literal", `{"trust_score": NaN}`, RecordRejectParse},
		{"Infinity literal", `{"trust_score": Infinity}`, RecordRejectParse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeDecisionRecord([]byte(tc.body))
			if err == nil {
				t.Fatalf("DecodeDecisionRecord(%q) unexpectedly succeeded", tc.body)
			}
			if got := RecordRejectReason(err); got != tc.reason {
				t.Fatalf("DecodeDecisionRecord(%q) reason = %q, want %q (err: %v)", tc.body, got, tc.reason, err)
			}
		})
	}
}

// TestDecodeDecisionRecord_AcceptsWellFormed confirms a normal record and a
// record followed only by whitespace both decode.
func TestDecodeDecisionRecord_AcceptsWellFormed(t *testing.T) {
	record, err := DecodeDecisionRecord([]byte(`{"schema_version": "1", "action": "deny"}`))
	if err != nil {
		t.Fatalf("decode well-formed record: %v", err)
	}
	if record.Action != "deny" {
		t.Fatalf("action = %q, want deny", record.Action)
	}

	if _, err := DecodeDecisionRecord([]byte("{\"action\": \"deny\"}\n\t ")); err != nil {
		t.Fatalf("trailing whitespace must be tolerated: %v", err)
	}
}

// TestRecordRejectReason_DefaultsToParse covers errors that are not one of the
// sentinel-typed rejections.
func TestRecordRejectReason_DefaultsToParse(t *testing.T) {
	if got := RecordRejectReason(errors.New("some other failure")); got != RecordRejectParse {
		t.Fatalf("reason = %q, want %q", got, RecordRejectParse)
	}
}
