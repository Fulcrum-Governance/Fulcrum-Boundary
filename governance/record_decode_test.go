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
		// before the schema check.
		{"duplicate unknown member", `{"zz": 1, "zz": 2}`, RecordRejectDuplicateKey},
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
