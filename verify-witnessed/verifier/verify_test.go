package verifier

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestFrozenCorpus(t *testing.T) {
	keys := corpusKeySet(t)
	tests := []struct {
		name        string
		wantFailure bool
	}{
		{name: "valid"},
		{name: "leaf-tamper", wantFailure: true},
		{name: "cosignature-tamper", wantFailure: true},
		{name: "partial-1-of-2"},
		{name: "sth-signature-tamper", wantFailure: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := VerifyDir(corpusPath(t, "bundles", test.name), keys)
			if err != nil {
				t.Fatalf("VerifyDir() error = %v", err)
			}
			var want Results
			expected := readTestFile(t, corpusPath(t, "expected", test.name+".json"))
			if err := json.Unmarshal(expected, &want); err != nil {
				t.Fatalf("decode expected results: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				gotJSON, _ := json.Marshal(got)
				t.Fatalf("results mismatch\n got: %s\nwant: %s", gotJSON, expected)
			}
			if got.HasFailures() != test.wantFailure {
				t.Fatalf("HasFailures() = %v, want %v", got.HasFailures(), test.wantFailure)
			}
		})
	}
}

func TestIndependentByteMutations(t *testing.T) {
	keys := corpusKeySet(t)
	tests := []struct {
		name       string
		mutate     func(*testing.T, string)
		wantStatus map[string]Status
	}{
		{
			name: "selected decision record byte",
			mutate: func(t *testing.T, bundle string) {
				path := filepath.Join(bundle, decisionsFile)
				data := readTestFile(t, path)
				changed := bytes.Replace(data, []byte(`"reason":"permitted`), []byte(`"reason":"Permitted`), 1)
				if bytes.Equal(data, changed) {
					t.Fatal("decision mutation did not change a byte")
				}
				writeTestFile(t, path, changed)
				refreshManifestRegistration(t, bundle, decisionsFile)
			},
			wantStatus: map[string]Status{
				"decision_record_integrity:sha256:04f26fb888aad12b244ed3350a0e6e5cd0d139e438a401f42d6c88ca230ddfc7": StatusFail,
				"manifest:decisions.jsonl": StatusPass,
				"source_hash_to_leaf":      StatusPass,
			},
		},
		{
			name: "proof source hash",
			mutate: func(t *testing.T, bundle string) {
				rewriteProof(t, bundle, func(proof *inclusionProof) { proof.SourceHash = mutateHash(proof.SourceHash) })
			},
			wantStatus: map[string]Status{
				"manifest:inclusion-proof-v1.json": StatusPass,
				"source_hash_to_leaf":              StatusFail,
				"inclusion_proof":                  StatusPass,
			},
		},
		{
			name: "proof leaf",
			mutate: func(t *testing.T, bundle string) {
				rewriteProof(t, bundle, func(proof *inclusionProof) { proof.MerkleLeafHash = mutateHash(proof.MerkleLeafHash) })
			},
			wantStatus: map[string]Status{
				"manifest:inclusion-proof-v1.json": StatusPass,
				"source_hash_to_leaf":              StatusFail,
				"inclusion_proof":                  StatusFail,
			},
		},
		{
			name: "proof audit path",
			mutate: func(t *testing.T, bundle string) {
				rewriteProof(t, bundle, func(proof *inclusionProof) { proof.AuditPath[0] = mutateHash(proof.AuditPath[0]) })
			},
			wantStatus: map[string]Status{
				"manifest:inclusion-proof-v1.json": StatusPass,
				"source_hash_to_leaf":              StatusPass,
				"inclusion_proof":                  StatusFail,
			},
		},
		{
			name: "manifest tenant label",
			mutate: func(t *testing.T, bundle string) {
				path := filepath.Join(bundle, manifestFilename)
				bundleManifest, err := decodeManifest(readTestFile(t, path))
				if err != nil {
					t.Fatalf("decode manifest before mutation: %v", err)
				}
				bundleManifest.TenantID = "00000000-0000-4000-8000-000000000099"
				data, err := json.Marshal(bundleManifest)
				if err != nil {
					t.Fatalf("marshal manifest mutation: %v", err)
				}
				writeTestFile(t, path, data)
			},
			wantStatus: map[string]Status{
				"bundle_tenant_binding": StatusFail,
			},
		},
		{
			name: "STH root",
			mutate: func(t *testing.T, bundle string) {
				rewriteTreeHead(t, bundle, func(head *treeHead) { head.RootHash = mutateHash(head.RootHash) })
			},
			wantStatus: map[string]Status{
				"manifest:tree-head-v1.json":        StatusPass,
				"inclusion_proof":                   StatusFail,
				"sth_signature":                     StatusFail,
				"witness_cosignature:witness-alpha": StatusFail,
				"witness_cosignature:witness-beta":  StatusFail,
			},
		},
		{
			name: "STH key",
			mutate: func(t *testing.T, bundle string) {
				rewriteTreeHead(t, bundle, func(head *treeHead) { head.SignatureKeyID = "ed25519:unknown-sth-v1" })
			},
			wantStatus: map[string]Status{
				"manifest:tree-head-v1.json":        StatusPass,
				"sth_signature":                     StatusFail,
				"witness_cosignature:witness-alpha": StatusFail,
				"witness_cosignature:witness-beta":  StatusFail,
			},
		},
		{
			name: "STH signature",
			mutate: func(t *testing.T, bundle string) {
				rewriteTreeHead(t, bundle, func(head *treeHead) { head.Signature = mutateEd25519(t, head.Signature) })
			},
			wantStatus: map[string]Status{
				"manifest:tree-head-v1.json": StatusPass,
				"sth_signature":              StatusFail,
			},
		},
		{
			name: "witness key",
			mutate: func(t *testing.T, bundle string) {
				rewriteAggregate(t, bundle, func(aggregate *cosignatureAggregate) {
					aggregate.ConfiguredWitnesses[0].WitnessKeyID = "ed25519:unknown-witness-v1"
				})
			},
			wantStatus: map[string]Status{
				"manifest:witness-cosignatures-v1.json": StatusPass,
				"witness_cosignature:witness-alpha":     StatusFail,
				"witness_cosignature:witness-beta":      StatusPass,
			},
		},
		{
			name: "witness signature",
			mutate: func(t *testing.T, bundle string) {
				rewriteAggregate(t, bundle, func(aggregate *cosignatureAggregate) {
					aggregate.Cosignatures[0].Cosignature = mutateEd25519(t, aggregate.Cosignatures[0].Cosignature)
				})
			},
			wantStatus: map[string]Status{
				"manifest:witness-cosignatures-v1.json": StatusPass,
				"witness_cosignature:witness-alpha":     StatusFail,
				"witness_cosignature:witness-beta":      StatusPass,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle := copyValidBundle(t)
			test.mutate(t, bundle)
			results, err := VerifyDir(bundle, keys)
			if err != nil {
				t.Fatalf("VerifyDir() error = %v", err)
			}
			if !results.HasFailures() {
				t.Fatal("mutated supplied artifact did not make verification fail")
			}
			for id, want := range test.wantStatus {
				if got := statusByID(t, results, id); got != want {
					t.Errorf("check %q status = %q, want %q", id, got, want)
				}
			}
		})
	}
}

func TestVerifyDirRejectsPayloadSHAReceiptProof(t *testing.T) {
	bundle := copyValidBundle(t)
	rewriteProof(t, bundle, func(proof *inclusionProof) {
		proof.SourceField = "payload_sha"
	})
	if _, err := decodeInclusionProof(readTestFile(t, filepath.Join(bundle, proofFile))); err == nil {
		t.Fatal("payload_sha proof mutation was not rejected by the wire decoder")
	}

	results, err := VerifyDir(bundle, corpusKeySet(t))
	if err != nil {
		t.Fatalf("VerifyDir() error = %v", err)
	}
	if !results.HasFailures() {
		t.Fatal("VerifyDir() accepted a payload_sha per-decision receipt proof")
	}
	if got := statusByID(t, results, "source_hash_to_leaf"); got != StatusFail {
		t.Fatalf("source_hash_to_leaf status = %q, want fail", got)
	}
}

func TestManifestUsesExactLFIncludingLineHash(t *testing.T) {
	bundle := copyValidBundle(t)
	path := filepath.Join(bundle, proofFile)
	data := readTestFile(t, path)
	data[len(data)-2] ^= 1
	writeTestFile(t, path, data)

	results, err := VerifyDir(bundle, corpusKeySet(t))
	if err != nil {
		t.Fatalf("VerifyDir() error = %v", err)
	}
	if got := statusByID(t, results, "manifest:"+proofFile); got != StatusFail {
		t.Fatalf("manifest proof status = %q, want fail", got)
	}
}

func TestMissingWitnessIsNonfatal(t *testing.T) {
	results, err := VerifyDir(corpusPath(t, "bundles", "partial-1-of-2"), corpusKeySet(t))
	if err != nil {
		t.Fatalf("VerifyDir() error = %v", err)
	}
	if results.HasFailures() {
		t.Fatal("missing configured witness must not be an aggregate failure")
	}
	if got := statusByID(t, results, "witness_cosignature:witness-beta"); got != StatusMissing {
		t.Fatalf("beta status = %q, want missing", got)
	}
}

func rewriteProof(t *testing.T, bundle string, mutate func(*inclusionProof)) {
	t.Helper()
	path := filepath.Join(bundle, proofFile)
	proof, err := decodeInclusionProof(readTestFile(t, path))
	if err != nil {
		t.Fatalf("decode proof before mutation: %v", err)
	}
	mutate(&proof)
	writeCompactSidecar(t, path, proof)
	refreshManifestRegistration(t, bundle, proofFile)
}

func rewriteTreeHead(t *testing.T, bundle string, mutate func(*treeHead)) {
	t.Helper()
	path := filepath.Join(bundle, treeHeadFile)
	head, err := decodeTreeHead(readTestFile(t, path))
	if err != nil {
		t.Fatalf("decode tree head before mutation: %v", err)
	}
	mutate(&head)
	writeCompactSidecar(t, path, head)
	refreshManifestRegistration(t, bundle, treeHeadFile)
}

func rewriteAggregate(t *testing.T, bundle string, mutate func(*cosignatureAggregate)) {
	t.Helper()
	path := filepath.Join(bundle, cosignaturesFile)
	aggregate, err := decodeCosignatureAggregate(readTestFile(t, path))
	if err != nil {
		t.Fatalf("decode aggregate before mutation: %v", err)
	}
	mutate(&aggregate)
	writeCompactSidecar(t, path, aggregate)
	refreshManifestRegistration(t, bundle, cosignaturesFile)
}

func writeCompactSidecar(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	writeTestFile(t, path, append(data, '\n'))
}

func refreshManifestRegistration(t *testing.T, bundle, name string) {
	t.Helper()
	path := filepath.Join(bundle, manifestFilename)
	bundleManifest, err := decodeManifest(readTestFile(t, path))
	if err != nil {
		t.Fatalf("decode manifest before refresh: %v", err)
	}
	lines, err := splitExactJSONLines(readTestFile(t, filepath.Join(bundle, name)))
	if err != nil {
		t.Fatalf("split mutated %s: %v", name, err)
	}
	for i := range bundleManifest.Files {
		if bundleManifest.Files[i].Name != name {
			continue
		}
		bundleManifest.Files[i].Lines = len(lines)
		bundleManifest.Files[i].LineSHA256 = make([]manifestLineHash, len(lines))
		for lineIndex, line := range lines {
			bundleManifest.Files[i].LineSHA256[lineIndex] = manifestLineHash{
				Line: lineIndex + 1, SHA256: exactLineSHA256(line.complete),
			}
		}
		data, marshalErr := json.Marshal(bundleManifest)
		if marshalErr != nil {
			t.Fatalf("marshal refreshed manifest: %v", marshalErr)
		}
		writeTestFile(t, path, data)
		return
	}
	t.Fatalf("manifest registration %q not found", name)
}

func mutateHash(value string) string {
	replacement := byte('0')
	if value[len(value)-1] == replacement {
		replacement = '1'
	}
	return value[:len(value)-1] + string(replacement)
}

func mutateEd25519(t *testing.T, value string) string {
	t.Helper()
	encoded := strings.TrimPrefix(value, "ed25519:")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode Ed25519 value: %v", err)
	}
	decoded[0] ^= 1
	return "ed25519:" + base64.StdEncoding.EncodeToString(decoded)
}

func copyValidBundle(t *testing.T) string {
	t.Helper()
	destination := t.TempDir()
	entries, err := os.ReadDir(corpusPath(t, "bundles", "valid"))
	if err != nil {
		t.Fatalf("read valid bundle: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected directory in valid bundle: %s", entry.Name())
		}
		writeTestFile(t, filepath.Join(destination, entry.Name()), readTestFile(t, corpusPath(t, "bundles", "valid", entry.Name())))
	}
	return destination
}

func corpusKeySet(t *testing.T) *KeySet {
	t.Helper()
	keys, err := LoadKeyring(corpusPath(t, "public-keys-v1.json"))
	if err != nil {
		t.Fatalf("LoadKeyring() error = %v", err)
	}
	return keys
}

func corpusPath(t *testing.T, elements ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	base := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "testdata", "witnessed-log-v1"))
	return filepath.Join(append([]string{base}, elements...)...)
}

func statusByID(t *testing.T, results Results, id string) Status {
	t.Helper()
	for _, check := range results.Checks {
		if check.ID == id {
			return check.Status
		}
	}
	t.Fatalf("check %q not found", id)
	return ""
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
