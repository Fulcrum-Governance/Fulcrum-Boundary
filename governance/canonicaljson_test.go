package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// TestMustCanonicalJSONConformsToRFC8785 pins the canonicalization helper's
// output byte-for-byte against RFC 8785 (JSON Canonicalization Scheme)
// expectations. It is the conformance gate that keeps every Boundary stable
// hash reproducible by an independent, stock JCS implementation in any
// language: if mustCanonicalJSON ever stops emitting JCS, one of these vectors
// breaks.
//
// The expectations are derived from the RFC 8785 rules, not from the current
// output of the code, so the test would catch a regression even if the helper
// were rewritten:
//   - object members are ordered lexicographically by UTF-16 code unit;
//   - strings keep "<", ">", and "&" literal (no HTML escaping);
//   - numbers use the ECMAScript Number-to-string (shortest round-trip) form;
//   - only the JSON standard two-character escapes and \uXXXX for control
//     characters appear; all other code points are literal UTF-8.
func TestMustCanonicalJSONConformsToRFC8785(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{
			// Generic object: keys arrive out of order and must be sorted
			// lexicographically; the literal "<", ">", "&" must survive
			// (Go's encoding/json would HTML-escape them); the number must
			// keep its ES6 shortest-round-trip form.
			name: "key-order, html-literal, number",
			value: map[string]any{
				"zeta":  "z",
				"alpha": "a < b & c > d",
				"beta":  json.Number("0.1"),
			},
			want: `{"alpha":"a < b & c > d","beta":0.1,"zeta":"z"}`,
		},
		{
			// RFC 8785 escaping rules: backspace, tab, newline, form feed,
			// carriage return, quote, and backslash use the JSON two-character
			// escapes; a non-ASCII rune (U+00E9) is emitted literally as UTF-8.
			name: "string escaping",
			value: map[string]any{
				"s": "\b\t\n\f\r\"\\é",
			},
			want: `{"s":"\b\t\n\f\r\"\\é"}`,
		},
		{
			// Nested structure with arrays: array element order is
			// preserved, object keys inside are sorted, booleans and null
			// pass through.
			name: "nested arrays and objects",
			value: map[string]any{
				"list": []any{json.Number("3"), json.Number("1"), json.Number("2")},
				"obj":  map[string]any{"b": true, "a": nil},
			},
			want: `{"list":[3,1,2],"obj":{"a":null,"b":true}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(mustCanonicalJSON(tc.value))
			if got != tc.want {
				t.Fatalf("mustCanonicalJSON mismatch\n got: %s\nwant: %s", got, tc.want)
			}
			// JCS is idempotent: canonicalizing already-canonical bytes must
			// be a fixed point. This guards against any double-escaping or
			// re-ordering drift.
			again := string(mustCanonicalJSON(json.RawMessage(got)))
			if again != tc.want {
				t.Fatalf("mustCanonicalJSON not idempotent\n got: %s\nwant: %s", again, tc.want)
			}
		})
	}
}

// TestDecisionRecordCanonicalFormIsRFC8785 pins the canonical form of a real
// DecisionRecordV1 byte-for-byte, with the two live regression risks exercised:
// a reason containing "&", "<", ">" (the HTML-escape guard), and a non-trivial
// float64 trust_score (the ECMAScript number-formatting guard). The record is
// run through the same blanking ComputeDecisionHash applies, so the pinned
// string is exactly the preimage that gets SHA-256'd. The companion
// decision_hash is asserted too, so the whole record_id/decision_hash chain is
// covered.
func TestDecisionRecordCanonicalFormIsRFC8785(t *testing.T) {
	record := DecisionRecordV1{
		SchemaVersion: DecisionRecordSchemaVersion,
		EventType:     "governance_decision",
		Action:        "deny",
		// Reason carries &, <, > — exactly what fmt.Sprintf-built reasons can
		// contain and what Go's default encoder would HTML-escape.
		Reason:     "blocked DROP TABLE & SELECT < 1 > 0",
		Tool:       "query",
		TrustScore: 1.0 / 3.0, // 0.3333333333333333 — arbitrary float, ES6 form
		TrustState: "TRUSTED",
	}
	// Mirror ComputeDecisionHash's blanking so the canonical preimage matches.
	record.RecordID = ""
	record.DecisionHash = ""
	record.Signature = ""
	record.SignatureKeyID = ""

	// Go's json.Marshal would HTML-escape "&", "<", ">" in reason; JCS undoes
	// that, so the canonical form keeps all three literal. The zero-value
	// Timestamp (a non-omitempty time.Time) serializes as
	// "0001-01-01T00:00:00Z" and so is part of the canonical preimage. The
	// trust_score keeps its ECMAScript shortest-round-trip form.
	const wantCanonical = `{"action":"deny","decision_hash":"","event_type":"governance_decision","reason":"blocked DROP TABLE & SELECT < 1 > 0","record_id":"","schema_version":"1","timestamp":"0001-01-01T00:00:00Z","tool":"query","trust_score":0.3333333333333333,"trust_state":"TRUSTED"}`

	got := string(mustCanonicalJSON(record))
	if got != wantCanonical {
		t.Fatalf("decision record canonical form mismatch\n got: %s\nwant: %s", got, wantCanonical)
	}

	// The decision_hash is the SHA-256 of exactly that canonical preimage.
	sum := sha256.Sum256([]byte(wantCanonical))
	wantHash := "sha256:" + hex.EncodeToString(sum[:])
	if gotHash := ComputeDecisionHash(record); gotHash != wantHash {
		t.Fatalf("ComputeDecisionHash mismatch\n got: %s\nwant: %s", gotHash, wantHash)
	}
}

// TestSiblingHashesRouteThroughJCS proves the request/raw-request/policy-bundle
// hashes share the JCS canonicalization path with the decision hash: hashing an
// in-memory request and hashing the equivalent raw JSON with deliberately
// reordered keys and HTML-significant characters yield the same digest, which
// can only happen if both go through the canonicalizing helper.
func TestSiblingHashesRouteThroughJCS(t *testing.T) {
	req := &GovernanceRequest{
		RequestID: "req-1",
		Transport: TransportMCP,
		ToolName:  "query",
		Action:    "tools/call",
		Arguments: map[string]any{"sql": "a < b & c > d"},
	}
	inMemory := ComputeRequestHash(req)

	// Same logical request, raw bytes, keys out of canonical order, with the
	// HTML-significant characters present. The field set is exactly what the
	// in-memory request marshals (the non-omitempty identity/context fields
	// serialize as empty strings); only the key order and the HTML-significant
	// characters differ from canonical. A non-JCS path would order keys
	// differently, so the digests only agree if both sides canonicalize
	// identically.
	raw := []byte(`{"tool_name":"query","action":"tools/call","transport":"mcp","request_id":"req-1","arguments":{"sql":"a < b & c > d"},"agent_id":"","tenant_id":"","envelope_id":"","trace_id":"","budget_key":""}`)
	fromRaw, err := ComputeRawRequestHash(raw)
	if err != nil {
		t.Fatalf("ComputeRawRequestHash: %v", err)
	}
	if inMemory != fromRaw {
		t.Fatalf("request hashes disagree across JCS path\n in-memory: %s\n       raw: %s", inMemory, fromRaw)
	}
}
