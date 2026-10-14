// Package conformance holds the cross-implementation conformance gate for the
// Boundary decision-record decision_hash.
//
// This test owns a frozen corpus of decision records under
// testdata/verifier-vectors/. For every vector it asserts that the committed
// decision_hash equals what the real governance.ComputeDecisionHash recomputes
// from the same record. That assertion is pure Go and ALWAYS runs, so it pins
// the RFC 8785 / JCS canonical form of a decision record forever: any drift in
// the canonicalization, field set, or hashing makes this test fail in CI,
// independent of whether a Python (or any other) verifier is present.
//
// The same committed files are read by the standalone Python verifier's test
// (verifiers/python/test_boundary_verify.py). Because both implementations
// assert against this one committed corpus, the corpus is the shared source of
// truth that mechanically keeps the Go and Python verifiers in agreement —
// without either side shelling out to the other.
//
// To regenerate the corpus after an intentional, reviewed change to the record
// schema or canonical form, run:
//
//	BOUNDARY_WRITE_VECTORS=1 go test ./tests/conformance/ -run TestVerifierVectors -count=1
//
// and commit the updated testdata/verifier-vectors/ files. Without that env var
// the test never writes; it only verifies the committed bytes.
package conformance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// vectorsDir is the committed corpus directory. The Python verifier test reads
// the exact same files.
const vectorsDir = "testdata/verifier-vectors"

// fixedTimestamp is a frozen RFC 3339 instant used for every vector so the
// corpus is deterministic (no time.Now()) and the committed decision_hash
// values are stable across regenerations.
const fixedTimestamp = "2026-06-01T04:36:39.787222Z"

// vectorExpect names the machine-readable outcome every verifier must produce
// for a corpus vector. "verify" exits 0; "reject:<reason>" exits non-zero with
// reason=<reason> (the governance.RecordReject* vocabulary shared with the
// Python, TypeScript, and Rust verifiers and documented in
// docs/VERIFIER_PARITY.md).
const (
	vectorExpectVerify       = "verify"
	vectorRejectDuplicateKey = "reject:" + governance.RecordRejectDuplicateKey
	vectorRejectTrailingData = "reject:" + governance.RecordRejectTrailingData
	vectorRejectUnknownField = "reject:" + governance.RecordRejectUnknownField
	vectorRejectParse        = "reject:" + governance.RecordRejectParse
	vectorRejectHashMismatch = "reject:" + governance.RecordRejectHashMismatch
)

// vector is one named corpus entry plus a short note on what canonical-form risk
// it exercises. The record's decision_hash and record_id are filled in by
// buildVectors using the real governance functions before the corpus is written.
type vector struct {
	// name is the corpus file stem; the file is name + ".json".
	name string
	// why documents the canonical-form property this vector pins.
	why string
	// expect is the machine-readable outcome every verifier must produce for
	// this file; empty means vectorExpectVerify.
	expect string
	// record is the decision record whose canonical decision_hash the committed
	// file stores. For reject vectors it is the interpretation a lenient
	// verifier would settle on (last-wins for duplicate keys, the pre-tamper
	// value for a mismatched hash): the committed file's stored decision_hash
	// is exactly its hash, so acceptance would be silent, not an error.
	record governance.DecisionRecordV1
	// raw, when non-empty, is the literal file body committed instead of the
	// marshaled record. The placeholders @DECISION_HASH@ and @RECORD_ID@ are
	// replaced with record.DecisionHash / record.RecordID at write time. Raw
	// exists because several vectors are deliberately non-canonical or
	// malformed bytes (duplicate keys, escapes, reordering, trailing data)
	// that cannot be produced by marshaling a struct.
	raw string
}

// buildVectors constructs the frozen corpus in memory: one record per
// canonical-form risk, each finished by computing decision_hash with the real
// governance.ComputeDecisionHash and deriving a representative record_id. The
// records deliberately span both schema versions, every action type, the
// parse-rejection shape, an HTML-significant reason, and a non-trivial float
// trust_score.
func buildVectors(t *testing.T) []vector {
	t.Helper()

	ts := mustParseTime(t, fixedTimestamp)

	vs := []vector{
		{
			name: "v1_allow",
			why:  "schema_version 1 (no route-context); action=allow",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-allow",
				Tool:          "query",
				Action:        "allow",
				Reason:        "permitted by default-allow policy",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_deny",
			why:  "schema_version 1; action=deny",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-deny",
				Tool:          "github.create_or_update_file",
				Action:        "deny",
				Reason:        "protected private-repo write denied before upstream execution",
				DecisionMode:  governance.DecisionModeDeterministic,
				MatchedRule:   "deny-github-write-after-taint-fixture",
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_warn",
			why:  "schema_version 1; action=warn",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportCLI,
				AgentID:       "agent-warn",
				Tool:          "rm",
				Action:        "warn",
				Reason:        "destructive command flagged for review",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    0.75,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_escalate",
			why:  "schema_version 1; action=escalate",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-escalate",
				Tool:          "payments.transfer",
				Action:        "escalate",
				Reason:        "high-value transfer escalated to human reviewer",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    0.5,
				TrustState:    "EVALUATING",
			},
		},
		{
			name: "v1_require_approval",
			why:  "schema_version 1; action=require_approval",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-approval",
				Tool:          "deploy.production",
				Action:        "require_approval",
				Reason:        "production deploy requires approval",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_reason_html_chars",
			why:  "HTML-escape coverage: reason contains & < > which JCS keeps literal (Go's encoder would escape them)",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-html",
				Tool:          "query",
				Action:        "deny",
				Reason:        "blocked DROP TABLE & SELECT < 1 > 0",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			name: "v1_float_trust_score",
			why:  "ECMAScript number formatting: non-trivial float64 trust_score (1.0/3.0) must serialize as 0.3333333333333333",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-float",
				Tool:          "query",
				Action:        "warn",
				Reason:        "borderline trust score",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1.0 / 3.0,
				TrustState:    "EVALUATING",
			},
		},
		{
			name: "parse_rejection",
			why:  "parse_rejected shape: event_type=parse_rejected, raw_shape_hash set, request_hash absent; still hashes through ComputeDecisionHash",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "parse_rejected",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				Action:        "deny",
				Reason:        "malformed tools/call payload rejected before parsing",
				DecisionMode:  governance.DecisionModeDeterministic,
				RawShapeHash:  governance.ComputeRawShapeHash([]byte("{not valid json")),
				TrustState:    "TRUSTED",
			},
		},
		{
			// The committed bytes carry "action" twice ("deny" then "allow").
			// A lenient last-wins parse settles on allow and the stored hash
			// matches that view, so a lax verifier would silently accept a
			// record whose verdict is ambiguous. All verifiers must reject.
			name:   "v1_duplicate_keys",
			why:    "duplicate object member at top level (action twice, deny then allow): must be rejected, not last-wins",
			expect: vectorRejectDuplicateKey,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-dup-top",
				Tool:          "query",
				Action:        "allow",
				Reason:        "duplicate key ambiguity: last-wins would flip the recorded verdict",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-dup-top",
  "tool": "query",
  "action": "deny",
  "action": "allow",
  "reason": "duplicate key ambiguity: last-wins would flip the recorded verdict",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			// Same ambiguity nested inside execution_claim: the member
			// upstream_called appears twice (true then false). A lenient
			// parse settles on false and the stored hash matches, so a lax
			// verifier would accept a record asserting upstream was never
			// called while the bytes also say it was. Must reject.
			name:   "v2_duplicate_keys_nested",
			why:    "duplicate object member nested in execution_claim (upstream_called twice): must be rejected at any depth",
			expect: vectorRejectDuplicateKey,
			record: governance.DecisionRecordV1{
				SchemaVersion:   governance.DecisionRecordSchemaV2,
				EventType:       "governance_decision",
				Timestamp:       ts,
				Adapter:         governance.TransportMCP,
				AgentID:         "agent-dup-nested",
				Tool:            "github.create_or_update_file",
				Action:          "deny",
				Reason:          "nested duplicate key ambiguity in execution_claim",
				DecisionMode:    governance.DecisionModeDeterministic,
				MatchedRule:     "deny-github-write-after-taint-fixture",
				RequestHash:     "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:      1,
				TrustState:      "TRUSTED",
				AdapterID:       "mcp-primary",
				RouteID:         "route-github-write",
				TopologyProfile: "single-route-forced",
				ExecutionClaim: &governance.ExecutionClaim{
					UpstreamCalled: false,
					Executed:       true,
					Source:         "mcp-adapter",
				},
			},
			raw: `{
  "schema_version": "2",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-dup-nested",
  "tool": "github.create_or_update_file",
  "action": "deny",
  "reason": "nested duplicate key ambiguity in execution_claim",
  "decision_mode": "deterministic",
  "matched_rule": "deny-github-write-after-taint-fixture",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED",
  "adapter_id": "mcp-primary",
  "route_id": "route-github-write",
  "topology_profile": "single-route-forced",
  "execution_claim": {
    "upstream_called": true,
    "upstream_called": false,
    "executed": true,
    "source": "mcp-adapter"
  }
}
`,
		},
		{
			// trust_score is written 1e2 — legal JSON, not the JCS canonical
			// spelling (which is 100). Parsing normalizes it, so the record
			// verifies. Go's struct decode lands on float64(100); the
			// standalone verifiers land on the same ECMAScript value.
			name: "v1_number_noncanonical",
			why:  "non-canonical number form (trust_score written 1e2): parses to 100 and must verify",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-num",
				Tool:          "query",
				Action:        "allow",
				Reason:        "non-canonical number form 1e2 canonicalizes to 100",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    100,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-num",
  "tool": "query",
  "action": "allow",
  "reason": "non-canonical number form 1e2 canonicalizes to 100",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1e2,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			// The committed bytes spell every code point of reason as a JSON
			// escape (é as é, 😀 as the surrogate pair 😀, even
			// ASCII and the angle brackets). Every spelling decodes to the
			// code points JCS emits literally, so the record verifies.
			name: "v1_unicode_escapes",
			why:  "reason stored with every code point escaped (\\u00e9, surrogate pair, \\u003c): decodes to the same string, must verify",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-unicode",
				Tool:          "query",
				Action:        "deny",
				Reason:        "café 😀 <ok>",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: strings.Replace(`{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-unicode",
  "tool": "query",
  "action": "deny",
  "reason": "@REASON@",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED"
}
`, "@REASON@", escapeAllForJSON("café 😀 <ok>"), 1),
		},
		{
			// Same shape as v1_unicode_escapes but the reason bytes spell
			// "café" decomposed (cafe + ◌́) while the stored hash was
			// computed over the precomposed form. Escapes are not
			// interchangeable: different code points canonicalize
			// differently, so the recomputed hash must not match.
			name:   "v1_unicode_escape_differs",
			why:    "escape variant decoding to different code points (decomposed e+\\u0301 vs precomposed é): canonicalizes differently, must fail decision_hash",
			expect: vectorRejectHashMismatch,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-unicode",
				Tool:          "query",
				Action:        "deny",
				Reason:        "café",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: strings.Replace(`{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-unicode",
  "tool": "query",
  "action": "deny",
  "reason": "@REASON@",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED"
}
`, "@REASON@", escapeAllForJSON("cafe"+string(rune(0x0301))), 1),
			// reason bytes spell cafe + combining ◌́ (decomposed); the stored
			// hash covers precomposed é, so the recomputed hash must differ.
		},
		{
			// The committed bytes list the same members in a scrambled order.
			// JCS sorts keys before hashing, so the record verifies.
			name: "v1_field_reordering",
			why:  "object members in non-sorted order: JCS sorts before hashing, must verify",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-reorder",
				Tool:          "query",
				Action:        "warn",
				Reason:        "field order is not canonical input",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    0.75,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "trust_state": "TRUSTED",
  "trust_score": 0.75,
  "decision_hash": "@DECISION_HASH@",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_mode": "deterministic",
  "reason": "field order is not canonical input",
  "action": "warn",
  "tool": "query",
  "agent_id": "agent-reorder",
  "adapter": "mcp",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "record_id": "@RECORD_ID@",
  "event_type": "governance_decision",
  "schema_version": "1"
}
`,
		},
		{
			// A well-formed record followed by bytes that are not whitespace.
			// A reader that stops at the first complete value would accept the
			// prefix and ignore the smuggled tail; all verifiers must reject.
			name:   "v1_trailing_data",
			why:    "non-whitespace bytes after the top-level JSON value: must be rejected",
			expect: vectorRejectTrailingData,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-trailing",
				Tool:          "query",
				Action:        "allow",
				Reason:        "trailing bytes after the record must not be ignored",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-trailing",
  "tool": "query",
  "action": "allow",
  "reason": "trailing bytes after the record must not be ignored",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED"
}
{"smuggled": true}
`,
		},
		{
			// The decision field was flipped allow->deny after the hash was
			// stored; decision_hash still names the pre-tamper record. The
			// recomputed hash must not match, on every verifier.
			name:   "v1_tampered_decision_unchanged_hash",
			why:    "action tampered (allow->deny) while decision_hash kept the pre-tamper value: must fail decision_hash",
			expect: vectorRejectHashMismatch,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-tampered",
				Tool:          "payments.transfer",
				Action:        "allow",
				Reason:        "transfer permitted by policy",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-tampered",
  "tool": "payments.transfer",
  "action": "deny",
  "reason": "transfer permitted by policy",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			// A member the record schema does not define, smuggled beside
			// the covered fields. The stored hash matches the record without
			// the member, so a verifier that drops unknown members would
			// accept forged content; the member set is closed, so every
			// verifier must reject at ingest with unknown-field.
			name:   "v1_unknown_field_toplevel",
			why:    "member name outside the schema's closed set at top level: must be rejected as unknown-field, not dropped",
			expect: vectorRejectUnknownField,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-unknown-top",
				Tool:          "query",
				Action:        "allow",
				Reason:        "extra top-level member must be rejected, not dropped before hashing",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-unknown-top",
  "tool": "query",
  "action": "allow",
  "reason": "extra top-level member must be rejected, not dropped before hashing",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED",
  "extra_field": "smuggled"
}
`,
		},
		{
			// Same closed-set rule where the unknown member's value is a
			// nested object rather than a scalar.
			name:   "v1_unknown_field_nested_object",
			why:    "member name outside the schema's closed set carrying a nested object: must be rejected as unknown-field",
			expect: vectorRejectUnknownField,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-unknown-obj",
				Tool:          "query",
				Action:        "allow",
				Reason:        "extra member holding an object must be rejected, not dropped before hashing",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-unknown-obj",
  "tool": "query",
  "action": "allow",
  "reason": "extra member holding an object must be rejected, not dropped before hashing",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED",
  "extra_obj": {"injected": true, "nested": {"deep": 1}}
}
`,
		},
		{
			// The closed-set rule applies inside execution_claim too: the
			// claim's member names are fixed (upstream_called, executed,
			// source), so a forged member must be rejected at any schema
			// object position.
			name:   "v2_unknown_field_execution_claim",
			why:    "member name outside the closed set inside execution_claim: must be rejected as unknown-field at every schema object position",
			expect: vectorRejectUnknownField,
			record: governance.DecisionRecordV1{
				SchemaVersion:   governance.DecisionRecordSchemaV2,
				EventType:       "governance_decision",
				Timestamp:       ts,
				Adapter:         governance.TransportMCP,
				AgentID:         "agent-unknown-claim",
				Tool:            "github.create_or_update_file",
				Action:          "deny",
				Reason:          "extra member inside execution_claim must be rejected, not dropped before hashing",
				DecisionMode:    governance.DecisionModeDeterministic,
				MatchedRule:     "deny-github-write-after-taint-fixture",
				RequestHash:     "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:      1,
				TrustState:      "TRUSTED",
				AdapterID:       "mcp-primary",
				RouteID:         "route-github-write",
				TopologyProfile: "single-route-forced",
				ExecutionClaim: &governance.ExecutionClaim{
					UpstreamCalled: false,
					Executed:       true,
					Source:         "mcp-adapter",
				},
			},
			raw: `{
  "schema_version": "2",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-unknown-claim",
  "tool": "github.create_or_update_file",
  "action": "deny",
  "reason": "extra member inside execution_claim must be rejected, not dropped before hashing",
  "decision_mode": "deterministic",
  "matched_rule": "deny-github-write-after-taint-fixture",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED",
  "adapter_id": "mcp-primary",
  "route_id": "route-github-write",
  "topology_profile": "single-route-forced",
  "execution_claim": {
    "upstream_called": false,
    "extra_claim_field": "forged",
    "executed": true,
    "source": "mcp-adapter"
  }
}
`,
		},
		{
			// NaN is not JSON (Python's stock decoder would otherwise accept
			// it); every verifier must reject it as a parse error — the
			// stored hash names the pre-corruption record.
			name:   "v1_number_nan",
			why:    "non-finite literal NaN is not JSON: must be rejected as parse-error, not decoded",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-nan",
				Tool:          "query",
				Action:        "warn",
				Reason:        "NaN trust_score is unrepresentable in JCS",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-nan",
  "tool": "query",
  "action": "warn",
  "reason": "NaN trust_score is unrepresentable in JCS",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": NaN,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			name:   "v1_number_infinity",
			why:    "non-finite literal Infinity is not JSON: must be rejected as parse-error, not decoded",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-inf",
				Tool:          "query",
				Action:        "warn",
				Reason:        "Infinity trust_score is unrepresentable in JCS",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-inf",
  "tool": "query",
  "action": "warn",
  "reason": "Infinity trust_score is unrepresentable in JCS",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": Infinity,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			name:   "v1_number_neg_infinity",
			why:    "non-finite literal -Infinity is not JSON: must be rejected as parse-error, not decoded",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-neginf",
				Tool:          "query",
				Action:        "warn",
				Reason:        "-Infinity trust_score is unrepresentable in JCS",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-neginf",
  "tool": "query",
  "action": "warn",
  "reason": "-Infinity trust_score is unrepresentable in JCS",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": -Infinity,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			name: "v2_route_context",
			why:  "schema_version 2: route-context fields populated and covered by decision_hash; includes execution_claim self-report",
			record: governance.DecisionRecordV1{
				SchemaVersion:   governance.DecisionRecordSchemaV2,
				EventType:       "governance_decision",
				Timestamp:       ts,
				Adapter:         governance.TransportMCP,
				AgentID:         "agent-v2",
				Tool:            "github.create_or_update_file",
				Action:          "deny",
				Reason:          "denied on routed path before upstream execution",
				DecisionMode:    governance.DecisionModeDeterministic,
				MatchedRule:     "deny-github-write-after-taint-fixture",
				RequestHash:     "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:      1,
				TrustState:      "TRUSTED",
				AdapterID:       "mcp-primary",
				RouteID:         "route-github-write",
				TopologyProfile: "single-route-forced",
				ExecutionClaim: &governance.ExecutionClaim{
					UpstreamCalled: false,
					Executed:       false,
					Source:         "mcp-adapter",
				},
			},
		},
		{
			// "ACTION" is a case variant of the declared member "action".
			// encoding/json would bind it to the Action field
			// case-insensitively and the record would verify, while the
			// standalone verifiers' exact-case member sets reject it: one
			// byte stream carrying two verdicts. The closed member set is
			// exact-case, so all four must report unknown-field.
			name:   "v1_case_variant_member",
			why:    "member name differing only in case (ACTION for action) at top level: must be rejected as unknown-field, not bound case-insensitively",
			expect: vectorRejectUnknownField,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-case-top",
				Tool:          "query",
				Action:        "allow",
				Reason:        "case-variant member name must not alias the declared field",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-case-top",
  "tool": "query",
  "ACTION": "allow",
  "reason": "case-variant member name must not alias the declared field",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			// "action" and "Action" in one object are one member under case
			// folding: a case-insensitive decoder (encoding/json) would bind
			// both to Action last-wins and verify, while exact-pair parsers
			// report unknown-field. Uniqueness is enforced case-folded on
			// all four verifiers, so the class is duplicate-key.
			name:   "v1_case_variant_duplicate_key",
			why:    "member names differing only in case (action then Action) in one object: must be rejected as duplicate-key, not last-wins",
			expect: vectorRejectDuplicateKey,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-case-dup",
				Tool:          "query",
				Action:        "allow",
				Reason:        "case-folded duplicate member must be rejected, not last-wins",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-case-dup",
  "tool": "query",
  "action": "deny",
  "Action": "allow",
  "reason": "case-folded duplicate member must be rejected, not last-wins",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			// Same case-variant rule inside execution_claim: EXECUTED is not
			// the declared member executed, and no lowercase executed member
			// is present to collide with, so the class is unknown-field.
			name:   "v2_case_variant_execution_claim",
			why:    "member name differing only in case (EXECUTED for executed) inside execution_claim: must be rejected as unknown-field at every schema object position",
			expect: vectorRejectUnknownField,
			record: governance.DecisionRecordV1{
				SchemaVersion:   governance.DecisionRecordSchemaV2,
				EventType:       "governance_decision",
				Timestamp:       ts,
				Adapter:         governance.TransportMCP,
				AgentID:         "agent-case-claim",
				Tool:            "github.create_or_update_file",
				Action:          "deny",
				Reason:          "case-variant member inside execution_claim must be rejected",
				DecisionMode:    governance.DecisionModeDeterministic,
				MatchedRule:     "deny-github-write-after-taint-fixture",
				RequestHash:     "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:      1,
				TrustState:      "TRUSTED",
				AdapterID:       "mcp-primary",
				RouteID:         "route-github-write",
				TopologyProfile: "single-route-forced",
				ExecutionClaim: &governance.ExecutionClaim{
					UpstreamCalled: false,
					Executed:       false,
					Source:         "mcp-adapter",
				},
			},
			raw: `{
  "schema_version": "2",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-case-claim",
  "tool": "github.create_or_update_file",
  "action": "deny",
  "reason": "case-variant member inside execution_claim must be rejected",
  "decision_mode": "deterministic",
  "matched_rule": "deny-github-write-after-taint-fixture",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED",
  "adapter_id": "mcp-primary",
  "route_id": "route-github-write",
  "topology_profile": "single-route-forced",
  "execution_claim": {
    "upstream_called": false,
    "EXECUTED": false,
    "source": "mcp-adapter"
  }
}
`,
		},
		{
			// A UTF-8 BOM before the record is not JSON whitespace: the
			// document must be rejected as parse-error, not treated as a
			// clean record with trailing bytes.
			name:   "v1_bom_prefixed",
			why:    "leading UTF-8 BOM: must be rejected as parse-error, not skipped or misclassified",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-bom",
				Tool:          "query",
				Action:        "allow",
				Reason:        "a byte-order mark is not JSON whitespace",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: "\uFEFF" + `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-bom",
  "tool": "query",
  "action": "allow",
  "reason": "a byte-order mark is not JSON whitespace",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			// 1e999 is legal JSON syntax but overflows float64: Go's decode
			// and Rust's parser reject it at ingest, Python's rfc8785 raises
			// FloatDomainError at canonicalization, and TypeScript's
			// canonicalize refuses the Infinity JSON.parse produced. All
			// four classify it parse-error.
			name:   "v1_number_overflow",
			why:    "number literal overflowing float64 (1e999): parses as non-finite, must be rejected as parse-error",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-numof",
				Tool:          "query",
				Action:        "warn",
				Reason:        "out-of-range number is unrepresentable in JCS",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-numof",
  "tool": "query",
  "action": "warn",
  "reason": "out-of-range number is unrepresentable in JCS",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1e999,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			name:   "v1_number_neg_overflow",
			why:    "negative number literal overflowing float64 (-1e999): must be rejected as parse-error",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-numof-neg",
				Tool:          "query",
				Action:        "warn",
				Reason:        "out-of-range number is unrepresentable in JCS",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-numof-neg",
  "tool": "query",
  "action": "warn",
  "reason": "out-of-range number is unrepresentable in JCS",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": -1e999,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			name:   "v1_number_overflow_upper_exp",
			why:    "number literal overflowing float64 with uppercase exponent (1E400): must be rejected as parse-error",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-numof-exp",
				Tool:          "query",
				Action:        "warn",
				Reason:        "out-of-range number is unrepresentable in JCS",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-numof-exp",
  "tool": "query",
  "action": "warn",
  "reason": "out-of-range number is unrepresentable in JCS",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1E400,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			// A 400-digit integer is legal JSON syntax but overflows float64
			// (10^399 > ~1.8e308): Go's decode, Rust's serde_json, and
			// Python's rfc8785 integer domain all refuse it; TypeScript's
			// JSON.parse yields Infinity and canonicalize refuses that. All
			// four classify it parse-error.
			name:   "v1_number_int_overflow",
			why:    "integer literal with ~400 digits overflowing float64: must be rejected as parse-error",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-numof-int",
				Tool:          "query",
				Action:        "warn",
				Reason:        "integer out of the RFC 8785 number domain",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-numof-int",
  "tool": "query",
  "action": "warn",
  "reason": "integer out of the RFC 8785 number domain",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890,
  "trust_state": "TRUSTED"
}
`,
		},
		{
			// An out-of-range number nested in execution_claim.source hits
			// the same ingest/canonicalization failure on all four
			// verifiers: Go cannot unmarshal a number into the string
			// field, and the standalone verifiers cannot canonicalize the
			// non-finite value.
			name:   "v2_execution_claim_number_overflow",
			why:    "number literal overflowing float64 inside execution_claim.source (1e999): must be rejected as parse-error",
			expect: vectorRejectParse,
			record: governance.DecisionRecordV1{
				SchemaVersion:   governance.DecisionRecordSchemaV2,
				EventType:       "governance_decision",
				Timestamp:       ts,
				Adapter:         governance.TransportMCP,
				AgentID:         "agent-numof-claim",
				Tool:            "github.create_or_update_file",
				Action:          "deny",
				Reason:          "out-of-range number inside execution_claim is unrepresentable in JCS",
				DecisionMode:    governance.DecisionModeDeterministic,
				MatchedRule:     "deny-github-write-after-taint-fixture",
				RequestHash:     "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:      1,
				TrustState:      "TRUSTED",
				AdapterID:       "mcp-primary",
				RouteID:         "route-github-write",
				TopologyProfile: "single-route-forced",
				ExecutionClaim: &governance.ExecutionClaim{
					UpstreamCalled: false,
					Executed:       false,
					Source:         "mcp-adapter",
				},
			},
			raw: `{
  "schema_version": "2",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-numof-claim",
  "tool": "github.create_or_update_file",
  "action": "deny",
  "reason": "out-of-range number inside execution_claim is unrepresentable in JCS",
  "decision_mode": "deterministic",
  "matched_rule": "deny-github-write-after-taint-fixture",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 1,
  "trust_state": "TRUSTED",
  "adapter_id": "mcp-primary",
  "route_id": "route-github-write",
  "topology_profile": "single-route-forced",
  "execution_claim": {
    "upstream_called": false,
    "executed": false,
    "source": 1e999
  }
}
`,
		},
		{
			// request_id is a declared DecisionRecordV1 member (omitempty)
			// that the Go emitter populates on pipeline records and covers
			// with decision_hash. A verifier whose known-field set omitted
			// it rejected every real pipeline record as unknown-field while
			// Go verified them; this vector pins it as covered.
			name: "v1_request_id",
			why:  "schema_version 1; request_id populated (pipeline correlation id) and covered by decision_hash",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-request-id",
				RequestID:     "req-9f31a2",
				Tool:          "query",
				Action:        "allow",
				Reason:        "pipeline-assigned request correlation id is covered by the hash",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    1,
				TrustState:    "TRUSTED",
			},
		},
		{
			// ADR-047: a required check that cannot produce a valid result is
			// recorded as action=check_indeterminate carrying the CheckFailure
			// context object; stage, class, category, and cause are emitted
			// and covered by decision_hash alongside request_id.
			name: "v1_check_indeterminate",
			why:  "schema_version 1; action=check_indeterminate carrying the ADR-047 check object (stage, class, category, cause) and request_id",
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-check",
				RequestID:     "req-77c1e0",
				Tool:          "github.create_or_update_file",
				Action:        governance.ActionCheckIndeterminate,
				Reason:        "required policy evaluation could not produce a valid result",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    0,
				Check: &governance.CheckFailure{
					Stage:    governance.CheckStagePolicyEval,
					Class:    governance.CheckClassPolicy,
					Category: governance.FailureUnavailable,
					Cause:    "policy evaluation failed",
				},
			},
		},
		{
			// "Stage" is a case variant of the declared check member
			// "stage". encoding/json binds it to CheckFailure.Stage
			// case-insensitively, so the hash over the lowercase-bound
			// record would match while the standalone verifiers' exact-case
			// member sets reject it. check is a closed schema object like
			// execution_claim: all four verifiers must report unknown-field.
			name:   "v1_case_variant_check",
			why:    "member name differing only in case (Stage for stage) inside check: must be rejected as unknown-field, not bound case-insensitively",
			expect: vectorRejectUnknownField,
			record: governance.DecisionRecordV1{
				SchemaVersion: governance.DecisionRecordSchemaVersion,
				EventType:     "governance_decision",
				Timestamp:     ts,
				Adapter:       governance.TransportMCP,
				AgentID:       "agent-check",
				RequestID:     "req-77c1e0",
				Tool:          "github.create_or_update_file",
				Action:        governance.ActionCheckIndeterminate,
				Reason:        "required policy evaluation could not produce a valid result",
				DecisionMode:  governance.DecisionModeDeterministic,
				RequestHash:   "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
				TrustScore:    0,
				Check: &governance.CheckFailure{
					Stage:    governance.CheckStagePolicyEval,
					Class:    governance.CheckClassPolicy,
					Category: governance.FailureUnavailable,
					Cause:    "policy evaluation failed",
				},
			},
			raw: `{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "@RECORD_ID@",
  "timestamp": "2026-06-01T04:36:39.787222Z",
  "adapter": "mcp",
  "agent_id": "agent-check",
  "request_id": "req-77c1e0",
  "tool": "github.create_or_update_file",
  "action": "check_indeterminate",
  "reason": "required policy evaluation could not produce a valid result",
  "decision_mode": "deterministic",
  "request_hash": "sha256:9ee20023d2bec36e7443092c34aa8439193f6ad0939187da18ed4cf044391265",
  "decision_hash": "@DECISION_HASH@",
  "trust_score": 0,
  "check": {
    "Stage": "policy_eval",
    "class": "policy",
    "category": "unavailable",
    "cause": "policy evaluation failed"
  }
}
`,
		},
	}

	// Finish each record with the real hashing function so the committed
	// decision_hash is exactly what a verifier must reproduce, and derive a
	// representative record_id from it (record_id is blanked before hashing, so
	// its value does not affect verification; this just mirrors real output).
	for i := range vs {
		vs[i].record.DecisionHash = governance.ComputeDecisionHash(vs[i].record)
		vs[i].record.RecordID = recordIDFromHash(vs[i].record.DecisionHash)
	}
	return vs
}

// TestVerifierVectors is the always-on cross-implementation gate. It
// (re)generates the frozen corpus only when BOUNDARY_WRITE_VECTORS=1, and on
// every run asserts that each committed vector's decision_hash equals what the
// real governance.ComputeDecisionHash recomputes. Drift fails CI.
func TestVerifierVectors(t *testing.T) {
	vs := buildVectors(t)

	if os.Getenv("BOUNDARY_WRITE_VECTORS") == "1" {
		writeCorpus(t, vs)
	}

	// Assert the committed bytes match the recomputed hashes. This reads what is
	// on disk (the committed corpus the Python verifier also reads), not the
	// in-memory build, so a stale or hand-edited corpus file is caught.
	committed := loadCommittedVectorFiles(t)
	if len(committed) != len(vs) {
		t.Fatalf("corpus drift: %d committed files, %d expected vectors; regenerate with BOUNDARY_WRITE_VECTORS=1", len(committed), len(vs))
	}

	for _, vec := range vs {
		t.Run(vec.name, func(t *testing.T) {
			raw, ok := committed[vec.name+".json"]
			if !ok {
				t.Fatalf("committed corpus missing %s.json; regenerate with BOUNDARY_WRITE_VECTORS=1", vec.name)
			}

			expect := vec.expect
			if expect == "" {
				expect = vectorExpectVerify
			}

			// Every committed file stores the decision_hash the in-memory
			// record produced — for reject vectors that is deliberately the
			// lenient interpretation's hash, so a lax verifier would accept.
			stored := storedDecisionHash(t, raw, vec.name)

			switch expect {
			case vectorRejectDuplicateKey, vectorRejectTrailingData,
				vectorRejectUnknownField, vectorRejectParse:
				// Ingest-stage rejections: the Go verifier's strict decode
				// must refuse these bytes with the reason the manifest
				// advertises — before any hash comparison runs.
				if _, err := governance.DecodeDecisionRecord(raw); err == nil {
					t.Fatalf("%s (%s): strict decode unexpectedly accepted the record; want %s", vec.name, vec.why, expect)
				} else {
					reason := governance.RecordRejectReason(err)
					if expect != "reject:"+reason {
						t.Fatalf("%s: strict decode rejected with reason=%s, manifest expects %s", vec.name, reason, expect)
					}
				}
				return
			case vectorRejectHashMismatch:
				// Well-formed record whose stored hash no longer matches the
				// covered fields: must decode, then fail verification.
				rec, err := governance.DecodeDecisionRecord(raw)
				if err != nil {
					t.Fatalf("%s (%s): strict decode rejected a well-formed record: %v", vec.name, vec.why, err)
				}
				if rec.DecisionHash != stored {
					t.Fatalf("%s: decoded decision_hash %s != stored %s", vec.name, rec.DecisionHash, stored)
				}
				if recomputed := governance.ComputeDecisionHash(rec); recomputed == rec.DecisionHash {
					t.Fatalf("%s (%s): recomputed decision_hash still matches a tampered record", vec.name, vec.why)
				}
				if err := governance.VerifyDecisionRecord(rec, nil, "", ""); err == nil {
					t.Fatalf("%s (%s): VerifyDecisionRecord accepted a tampered record", vec.name, vec.why)
				}
				if vec.record.DecisionHash != stored {
					t.Fatalf("%s: in-memory pre-tamper hash %s != stored %s", vec.name, vec.record.DecisionHash, stored)
				}
				return
			}

			rec, err := governance.DecodeDecisionRecord(raw)
			if err != nil {
				t.Fatalf("decode committed %s.json: %v", vec.name, err)
			}
			if rec.DecisionHash == "" {
				t.Fatalf("committed %s.json has empty decision_hash", vec.name)
			}
			recomputed := governance.ComputeDecisionHash(rec)
			if recomputed != rec.DecisionHash {
				t.Fatalf("decision_hash drift for %s (%s)\n committed: %s\nrecomputed: %s\nregenerate with BOUNDARY_WRITE_VECTORS=1 after confirming the change is intended",
					vec.name, vec.why, rec.DecisionHash, recomputed)
			}
			// Sanity: the in-memory build that produced the committed file must
			// agree with the committed file's stored hash too, so the corpus is
			// not silently out of step with buildVectors.
			if vec.record.DecisionHash != rec.DecisionHash {
				t.Fatalf("in-memory vector and committed file disagree for %s\n in-memory: %s\ncommitted: %s\nregenerate with BOUNDARY_WRITE_VECTORS=1",
					vec.name, vec.record.DecisionHash, rec.DecisionHash)
			}
		})
	}

	// A forgery on the committed bytes must change the recomputed hash: flip
	// action allow->deny on a known-allow vector and confirm the digest moves.
	t.Run("forgery_changes_hash", func(t *testing.T) {
		raw, ok := committed["v1_allow.json"]
		if !ok {
			t.Skip("v1_allow.json not present")
		}
		var rec governance.DecisionRecordV1
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatalf("decode v1_allow.json: %v", err)
		}
		original := governance.ComputeDecisionHash(rec)
		rec.Action = "deny"
		forged := governance.ComputeDecisionHash(rec)
		if forged == original {
			t.Fatalf("forgery did not change decision_hash; hash is not covering action")
		}
		if forged == rec.DecisionHash {
			t.Fatalf("forged record's recomputed hash still equals the stored decision_hash; verification would not catch the edit")
		}
	})
}

// TestManifestMatchesCorpus asserts the committed manifest.json enumerates
// exactly the committed vector files, with the stored decision_hash for each.
// The Python verifier uses this manifest to discover vectors, so it must stay in
// lockstep with the corpus.
func TestManifestMatchesCorpus(t *testing.T) {
	manifestPath := filepath.Join(vectorsDir, "manifest.json")
	raw, err := os.ReadFile(manifestPath) // #nosec G304 -- fixed test-data path.
	if err != nil {
		t.Fatalf("read manifest: %v (regenerate with BOUNDARY_WRITE_VECTORS=1)", err)
	}
	var manifest corpusManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	committed := loadCommittedVectorFiles(t)
	if len(manifest.Vectors) != len(committed) {
		t.Fatalf("manifest lists %d vectors, corpus has %d files; regenerate with BOUNDARY_WRITE_VECTORS=1", len(manifest.Vectors), len(committed))
	}
	for _, entry := range manifest.Vectors {
		raw, ok := committed[entry.File]
		if !ok {
			t.Fatalf("manifest references missing file %s", entry.File)
		}
		if entry.Expect == "" {
			t.Fatalf("manifest entry for %s has empty expect", entry.File)
		}
		// The manifest's decision_hash is the hash the file stores, even for
		// reject vectors (where it is the lenient interpretation's hash). Read
		// it from the bytes without decoding so malformed files still check.
		if stored := storedDecisionHash(t, raw, entry.File); entry.DecisionHash != stored {
			t.Fatalf("manifest decision_hash for %s disagrees with file\n manifest: %s\n     file: %s", entry.File, entry.DecisionHash, stored)
		}
	}
}

// corpusManifest is the committed manifest.json structure consumed by the Python
// verifier to enumerate the corpus.
type corpusManifest struct {
	// Description is a human note on what the corpus is for.
	Description string `json:"description"`
	// Vectors lists every committed vector file with its expected decision_hash
	// and a short note on the canonical-form property it pins.
	Vectors []manifestEntry `json:"vectors"`
}

type manifestEntry struct {
	// File is the corpus file name (relative to the corpus directory).
	File string `json:"file"`
	// DecisionHash is the decision_hash the file stores. For reject vectors
	// this is deliberately the hash a lenient interpretation would compute
	// (last-wins duplicate keys, pre-tamper content), so acceptance is silent
	// unless the verifier rejects for the advertised reason.
	DecisionHash string `json:"decision_hash"`
	// Expect is the machine-readable outcome every verifier must produce:
	// "verify" or "reject:<reason>" using the shared reason vocabulary
	// (docs/VERIFIER_PARITY.md).
	Expect string `json:"expect"`
	// Why documents the canonical-form risk the vector exercises.
	Why string `json:"why"`
}

// writeCorpus serializes every vector to its own pretty-printed JSON file under
// vectorsDir and writes manifest.json. It runs only under BOUNDARY_WRITE_VECTORS=1.
func writeCorpus(t *testing.T, vs []vector) {
	t.Helper()
	if err := os.MkdirAll(vectorsDir, 0o755); err != nil {
		t.Fatalf("mkdir corpus dir: %v", err)
	}
	manifest := corpusManifest{
		Description: "Frozen decision-record conformance corpus. Each file's decision_hash is " +
			"reproduced by governance.ComputeDecisionHash (Go) and by a stock RFC 8785 / JCS " +
			"verifier (see verifiers/python). Regenerate with BOUNDARY_WRITE_VECTORS=1.",
	}
	for _, vec := range vs {
		var body []byte
		if vec.raw != "" {
			// Literal bytes: substitute the placeholders with the hash/id the
			// in-memory record produced so the stored decision_hash is exactly
			// what a lenient verifier would compute.
			body = []byte(vec.raw)
			body = bytes.ReplaceAll(body, []byte("@DECISION_HASH@"), []byte(vec.record.DecisionHash))
			body = bytes.ReplaceAll(body, []byte("@RECORD_ID@"), []byte(vec.record.RecordID))
		} else {
			var err error
			body, err = json.MarshalIndent(vec.record, "", "  ")
			if err != nil {
				t.Fatalf("marshal vector %s: %v", vec.name, err)
			}
			body = append(body, '\n')
		}
		path := filepath.Join(vectorsDir, vec.name+".json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatalf("write vector %s: %v", vec.name, err)
		}
		expect := vec.expect
		if expect == "" {
			expect = vectorExpectVerify
		}
		manifest.Vectors = append(manifest.Vectors, manifestEntry{
			File:         vec.name + ".json",
			DecisionHash: vec.record.DecisionHash,
			Expect:       expect,
			Why:          vec.why,
		})
	}
	sort.Slice(manifest.Vectors, func(i, j int) bool {
		return manifest.Vectors[i].File < manifest.Vectors[j].File
	})
	mb, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	mb = append(mb, '\n')
	if err := os.WriteFile(filepath.Join(vectorsDir, "manifest.json"), mb, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Logf("wrote %d vectors + manifest.json to %s", len(vs), vectorsDir)
}

// loadCommittedVectorFiles reads every *.json vector file under vectorsDir
// (excluding manifest.json) and returns them keyed by file name.
func loadCommittedVectorFiles(t *testing.T) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(vectorsDir)
	if err != nil {
		t.Fatalf("read corpus dir %s: %v (regenerate with BOUNDARY_WRITE_VECTORS=1)", vectorsDir, err)
	}
	out := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || name == "manifest.json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(vectorsDir, name)) // #nosec G304 -- fixed test-data path.
		if err != nil {
			t.Fatalf("read corpus file %s: %v", name, err)
		}
		out[name] = raw
	}
	return out
}
