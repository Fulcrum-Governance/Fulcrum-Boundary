package verifier

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInclusionProofStrictDecoderRejectsMalformedShapes(t *testing.T) {
	valid := readTestFile(t, corpusPath(t, "bundles", "valid", proofFile))
	tests := []struct {
		name string
		data func() []byte
	}{
		{
			name: "unknown nested field",
			data: func() []byte {
				object := decodeTestObject(t, valid)
				object["invariant_citation"].(map[string]any)["unknown"] = true
				return compactTestObject(t, object)
			},
		},
		{
			name: "duplicate member",
			data: func() []byte {
				return append(append([]byte(nil), valid[:len(valid)-2]...), []byte(`,"log_id":"duplicate"}`+"\n")...)
			},
		},
		{
			name: "trailing value",
			data: func() []byte { return append(append([]byte(nil), valid...), []byte("{}\n")...) },
		},
		{
			name: "malformed",
			data: func() []byte { return append([]byte(nil), valid[:len(valid)-3]...) },
		},
		{
			name: "missing required number",
			data: func() []byte {
				object := decodeTestObject(t, valid)
				delete(object, "tree_size")
				return compactTestObject(t, object)
			},
		},
		{
			name: "null required array",
			data: func() []byte {
				object := decodeTestObject(t, valid)
				object["audit_path"] = nil
				return compactTestObject(t, object)
			},
		},
		{
			name: "uppercase hash",
			data: func() []byte {
				object := decodeTestObject(t, valid)
				object["source_hash"] = strings.ToUpper(object["source_hash"].(string))
				return compactTestObject(t, object)
			},
		},
		{
			name: "insignificant whitespace",
			data: func() []byte { return bytes.Replace(valid, []byte(":"), []byte(": "), 1) },
		},
		{
			name: "CRLF",
			data: func() []byte { return append(append([]byte(nil), valid[:len(valid)-1]...), '\r', '\n') },
		},
		{
			name: "lone Unicode surrogate",
			data: func() []byte {
				return bytes.Replace(valid, []byte(`"log_id":"`), []byte(`"log_id":"\ud800`), 1)
			},
		},
		{
			name: "implemented high-risk invariant",
			data: func() []byte {
				object := decodeTestObject(t, valid)
				citation := object["invariant_citation"].(map[string]any)
				citation["theorem"] = "high_risk_execution_requires_stronger_trust"
				citation["runtime_status"] = "Implemented"
				return compactTestObject(t, object)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeInclusionProof(test.data()); err == nil {
				t.Fatal("decodeInclusionProof() accepted invalid input")
			}
		})
	}
}

func TestInclusionProofStrictDecoderAllowsConjecturedHighRiskInvariant(t *testing.T) {
	valid := readTestFile(t, corpusPath(t, "bundles", "valid", proofFile))
	object := decodeTestObject(t, valid)
	citation := object["invariant_citation"].(map[string]any)
	citation["theorem"] = "high_risk_execution_kernel_guarantee"
	citation["runtime_status"] = "Conjectured"

	if _, err := decodeInclusionProof(compactTestObject(t, object)); err != nil {
		t.Fatalf("decodeInclusionProof() rejected a conjectured high-risk invariant: %v", err)
	}
}

func TestStrictDecoderRequiresNestedZeroValueMembers(t *testing.T) {
	manifestData := readTestFile(t, corpusPath(t, "bundles", "valid", manifestFilename))
	manifestObject := decodeTestObject(t, manifestData)
	files := manifestObject["files"].([]any)
	delete(files[0].(map[string]any), "lines")
	if _, err := decodeManifest(mustMarshalTest(t, manifestObject)); err == nil {
		t.Fatal("decodeManifest() accepted a missing required zero-valued number")
	}

	aggregateData := readTestFile(t, corpusPath(t, "bundles", "valid", cosignaturesFile))
	aggregateObject := decodeTestObject(t, aggregateData)
	configured := aggregateObject["configured_witnesses"].([]any)
	delete(configured[0].(map[string]any), "witness_key_id")
	if _, err := decodeCosignatureAggregate(compactTestObject(t, aggregateObject)); err == nil {
		t.Fatal("decodeCosignatureAggregate() accepted a missing nested key ID")
	}

	decisionData := readTestFile(t, corpusPath(t, "bundles", "valid", decisionsFile))
	decisionObject := decodeTestObject(t, decisionData)
	delete(decisionObject, "trust_score")
	if _, err := decodeDecisionRecord(mustMarshalTest(t, decisionObject)); err == nil {
		t.Fatal("decodeDecisionRecord() accepted a missing required trust_score")
	}
}

func TestSignatureAndKeyShapesAreStrict(t *testing.T) {
	headData := readTestFile(t, corpusPath(t, "bundles", "valid", treeHeadFile))
	headObject := decodeTestObject(t, headData)
	tests := []struct {
		name      string
		signature string
	}{
		{name: "missing prefix", signature: strings.TrimPrefix(headObject["signature"].(string), "ed25519:")},
		{name: "bad base64", signature: "ed25519:not-base64"},
		{name: "wrong length", signature: "ed25519:AA=="},
		{name: "noncanonical base64", signature: strings.Replace(headObject["signature"].(string), "ed25519:", "ed25519:\n", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			object := decodeTestObject(t, headData)
			object["signature"] = test.signature
			if _, err := decodeTreeHead(compactTestObject(t, object)); err == nil {
				t.Fatal("decodeTreeHead() accepted an invalid signature")
			}
		})
	}

	object := decodeTestObject(t, headData)
	object["signature_key_id"] = "rsa:not-ed25519"
	if _, err := decodeTreeHead(compactTestObject(t, object)); err == nil {
		t.Fatal("decodeTreeHead() accepted an invalid key ID")
	}
}

func TestExactJSONLinesRejectsInvalidFraming(t *testing.T) {
	tests := map[string][]byte{
		"missing LF": []byte(`{}`),
		"empty line": []byte("{}\n\n"),
		"CRLF":       []byte("{}\r\n"),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := splitExactJSONLines(data); err == nil {
				t.Fatal("splitExactJSONLines() accepted invalid framing")
			}
		})
	}
	lines, err := splitExactJSONLines(nil)
	if err != nil || len(lines) != 0 {
		t.Fatalf("empty JSONL = (%v, %v), want empty success", lines, err)
	}
}

func TestReadRegularFileRejectsNonRegularPaths(t *testing.T) {
	dir := t.TempDir()
	if _, err := readRegularFile(dir); err == nil {
		t.Fatal("readRegularFile() accepted a directory")
	}
	target := filepath.Join(dir, "target")
	writeTestFile(t, target, []byte("data"))
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if _, err := readRegularFile(link); err == nil {
		t.Fatal("readRegularFile() followed a symlink")
	}
}

func decodeTestObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("decode test object: %v", err)
	}
	return object
}

func compactTestObject(t *testing.T, object map[string]any) []byte {
	t.Helper()
	return append(mustMarshalTest(t, object), '\n')
}

func mustMarshalTest(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test JSON: %v", err)
	}
	return data
}
