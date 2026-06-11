package governance

import (
	"os"
	"path/filepath"
	"testing"
)

// policyFuzzSeedDirs are repository directories whose .yaml/.yml files seed
// FuzzPolicyParse. Using the project's own real policies (the v1-schema starter
// and the policy-as-code corpus, including the intentionally malformed case)
// gives the fuzzer structurally valid starting points to mutate, so it explores
// realistic policy shapes rather than random YAML noise. Paths are relative to
// this package directory (go test's working directory); a missing directory is
// skipped, not fatal.
var policyFuzzSeedDirs = []string{
	"../schemas",
	"../policies",
	"../tests/fixtures/policy-test/policies",
	"../tests/fixtures/policy-test/cases",
}

// policyFuzzSeeds gathers seed YAML bodies from policyFuzzSeedDirs plus a small
// fixed set of degenerate inputs (empty, a bare v1 envelope, a syntactically
// broken document). Directory walks are best-effort: unreadable trees are
// skipped so the inline seeds always run.
func policyFuzzSeeds(t *testing.T) [][]byte {
	t.Helper()
	seeds := [][]byte{
		[]byte(""),
		[]byte("   \n"),
		[]byte("schema_version: \"1\"\npolicy:\n  name: x\n  version: \"1\"\n  rules:\n    - name: r\n      tool: \"*\"\n      action: deny\n"),
		[]byte("name: legacy\nrules:\n  - name: r\n    tool: \"*\"\n    action: allow\n"),
		[]byte("name: broken\nrules:\n  - name: [unterminated\n"),
		[]byte("schema_version: \"1\"\npolicy: {name: p, version: \"1\", rules: [{name: r, tool: q, action: deny, conditions: [{type: regex, field: code, regex: \"([\"}]}]}}\n"),
	}

	for _, dir := range policyFuzzSeedDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Logf("policy seed dir %s unavailable (%v); skipping", dir, err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := filepath.Ext(entry.Name())
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			body, err := os.ReadFile(path)
			if err != nil {
				t.Logf("policy seed %s unreadable (%v); skipping", path, err)
				continue
			}
			seeds = append(seeds, body)
		}
	}
	return seeds
}

// FuzzPolicyParse fuzzes arbitrary bytes through ParseStaticPolicyDocument, the
// narrowest exported bytes-level entry to the static-policy load path. It routes
// fuzzed input through both branches the loader dispatches on: the policy-v1
// schema validator (policyeval.ValidatePolicyV1YAML) when the bytes look like a
// v1 envelope, and the legacy StaticPolicyDocument YAML unmarshal otherwise.
//
// The contract under test is robustness, not acceptance: for ANY byte slice the
// parser must terminate by returning a (document, error) pair and must never
// panic. A rejection must surface as a non-nil error — never a crash and never a
// nil document with a nil error. When parsing succeeds, the returned document
// must be non-nil. The fuzzer makes no claim about WHICH inputs are valid; it
// only pins that malformed policy bytes degrade to an error rather than taking
// down the loader. (The on-disk loader, LoadStaticPolicyFiles, layers warnings
// on top of this; this target fuzzes the byte-level parse it is built on.)
func FuzzPolicyParse(f *testing.F) {
	for _, seed := range policyFuzzSeeds(&testing.T{}) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		// Must never panic for any input. The path argument is a fixed label;
		// only body is fuzzed.
		doc, err := ParseStaticPolicyDocument("fuzz.yaml", body)

		// A rejection is an error, not a crash, and not a silent nil/nil. When
		// there is no error, a usable document must be returned.
		if err == nil && doc == nil {
			t.Fatalf("ParseStaticPolicyDocument returned nil document and nil error for %q", body)
		}
	})
}
