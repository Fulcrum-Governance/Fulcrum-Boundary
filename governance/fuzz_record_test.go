package governance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// recordFuzzSeedDirs are repository directories whose .json files seed
// FuzzDecisionRecordRoundTrip. The conformance vectors are the frozen, spec-grade
// records whose decision_hash a stock RFC 8785 verifier must reproduce; the
// docs/examples records are the human-facing samples. Seeding with both means the
// fuzzer starts from every record shape the project promises to hash stably and
// mutates outward. Paths are relative to this package directory (go test's
// working directory); a missing directory is skipped, not fatal.
var recordFuzzSeedDirs = []string{
	"../tests/conformance/testdata/verifier-vectors",
	"../docs/examples",
}

// recordFuzzSeeds gathers seed JSON bodies from recordFuzzSeedDirs plus a few
// inline degenerate inputs (empty object, a minimal record, HTML-bearing reason,
// a non-trivial float trust_score). Only files that actually unmarshal into a
// DecisionRecordV1 are kept, so non-record JSON in docs/examples (e.g. MCP config
// fixtures) does not pollute the corpus. Directory walks are best-effort.
func recordFuzzSeeds(t *testing.T) [][]byte {
	t.Helper()
	seeds := [][]byte{
		[]byte(`{}`),
		[]byte(`{"schema_version":"1","action":"deny","trust_score":0}`),
		[]byte(`{"schema_version":"2","action":"allow","trust_score":0.3333333333333333,"reason":"DROP TABLE & SELECT < 1 > 0"}`),
		[]byte(`{"schema_version":"1","action":"allow","trust_score":1,"execution_claim":{"upstream_called":false,"executed":false}}`),
	}

	for _, dir := range recordFuzzSeedDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Logf("record seed dir %s unavailable (%v); skipping", dir, err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			body, err := os.ReadFile(path)
			if err != nil {
				t.Logf("record seed %s unreadable (%v); skipping", path, err)
				continue
			}
			// Keep only bytes that decode into a record, so the corpus is
			// records, not arbitrary JSON files that happen to sit alongside.
			var rec DecisionRecordV1
			if json.Unmarshal(body, &rec) != nil {
				continue
			}
			seeds = append(seeds, body)
		}
	}
	return seeds
}

// FuzzDecisionRecordRoundTrip fuzzes arbitrary JSON bytes against the decision
// record hashing path and pins canonicalization stability — the property the
// cross-language verifier story depends on.
//
// For any input that successfully unmarshals into a DecisionRecordV1:
//
//  1. ComputeDecisionHash must not panic. It blanks the four self-excluding
//     fields and routes through RFC 8785 canonical JSON; no record value
//     (NaN/Inf excepted, which cannot arrive via encoding/json) may crash it.
//  2. The hash is canonical/idempotent: re-marshaling the decoded record and
//     decoding it again must yield a record whose ComputeDecisionHash is byte-for-
//     byte identical to the first. This is the load-bearing guarantee — that the
//     digest is a function of record CONTENT, invariant under JSON
//     re-serialization (key order, whitespace, HTML-escape differences). If a
//     marshal→unmarshal cycle could shift the hash, an independent verifier could
//     not reproduce it.
//  3. The hash is well-formed: "sha256:" + 64 lowercase hex chars.
//
// Inputs that do not unmarshal are simply skipped — this target fuzzes the
// hashing/canonicalization contract, not the JSON decoder. Verification binding
// (request_hash/policy_bundle_hash/signature) is out of scope here; this asserts
// only that the integrity digest is stable, which is what makes it verifiable.
func FuzzDecisionRecordRoundTrip(f *testing.F) {
	for _, seed := range recordFuzzSeeds(&testing.T{}) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var rec DecisionRecordV1
		if err := json.Unmarshal(data, &rec); err != nil {
			// Not a record; nothing to assert about hashing. The JSON decoder is
			// not under test here.
			return
		}

		// Contract 1: hashing arbitrary decoded records must not panic.
		first := ComputeDecisionHash(rec)

		// Contract 3: well-formed digest. "sha256:" + 64 lowercase hex.
		if !isSHA256Hex(first) {
			t.Fatalf("ComputeDecisionHash returned malformed digest %q", first)
		}

		// Contract 2: canonicalization stability across a marshal/unmarshal cycle.
		// Re-encoding then decoding must not change the computed hash.
		reEncoded, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("re-marshal of a decoded record failed: %v", err)
		}
		var roundTripped DecisionRecordV1
		if err := json.Unmarshal(reEncoded, &roundTripped); err != nil {
			t.Fatalf("re-unmarshal of a re-marshaled record failed: %v (bytes=%s)", err, reEncoded)
		}
		second := ComputeDecisionHash(roundTripped)
		if first != second {
			t.Fatalf("decision_hash not stable under marshal/unmarshal: first=%s second=%s\noriginal=%s\nreencoded=%s", first, second, data, reEncoded)
		}
	})
}

// isSHA256Hex reports whether s is a "sha256:"-prefixed lowercase-hex SHA-256
// digest (the exact shape ComputeDecisionHash documents it returns).
func isSHA256Hex(s string) bool {
	const prefix = "sha256:"
	if len(s) != len(prefix)+64 {
		return false
	}
	if s[:len(prefix)] != prefix {
		return false
	}
	for _, c := range s[len(prefix):] {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
