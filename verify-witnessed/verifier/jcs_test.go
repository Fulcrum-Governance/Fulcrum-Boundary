package verifier

import (
	"encoding/json"
	"testing"
)

func TestPinnedBoundaryDecisionHash(t *testing.T) {
	lines, err := splitExactJSONLines(readTestFile(t, corpusPath(t, "bundles", "valid", decisionsFile)))
	if err != nil {
		t.Fatalf("split decision corpus: %v", err)
	}
	record, err := decodeDecisionRecord(lines[0].body)
	if err != nil {
		t.Fatalf("decode decision record: %v", err)
	}
	got, err := computeDecisionHash(record)
	if err != nil {
		t.Fatalf("computeDecisionHash() error = %v", err)
	}
	const want = "sha256:04f26fb888aad12b244ed3350a0e6e5cd0d139e438a401f42d6c88ca230ddfc7"
	if got != want {
		t.Fatalf("decision hash = %q, want %q", got, want)
	}
}

func TestCanonicalJSONUsesJCSOrderingAndEscapes(t *testing.T) {
	value := map[string]any{
		"\ue000":     json.Number("2"),
		"\U00010000": json.Number("1"),
		"text":       "<>&\n\b",
	}
	got, err := canonicalJSON(value)
	if err != nil {
		t.Fatalf("canonicalJSON() error = %v", err)
	}
	const want = `{"text":"<>&\n\b","𐀀":1,"":2}`
	if string(got) != want {
		t.Fatalf("canonical JSON = %s, want %s", got, want)
	}
}

func TestCanonicalNumberMatchesECMAScriptThresholds(t *testing.T) {
	tests := map[string]string{
		"-0":                    "0",
		"1e-7":                  "1e-7",
		"1e-6":                  "0.000001",
		"1e20":                  "100000000000000000000",
		"1e21":                  "1e+21",
		"333333333.33333329":    "333333333.3333333",
		"-12345678901234567890": "-12345678901234567000",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := canonicalNumber(json.Number(input))
			if err != nil {
				t.Fatalf("canonicalNumber() error = %v", err)
			}
			if got != want {
				t.Fatalf("canonicalNumber(%q) = %q, want %q", input, got, want)
			}
		})
	}
	if _, err := canonicalNumber(json.Number("NaN")); err == nil {
		t.Fatal("canonicalNumber() accepted NaN")
	}
}
