package verifier

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/mod/sumdb/tlog"
)

// TestWitnessedBundleWithEveryRecordMember is the end-to-end regression for the
// v0.14.0 defect: a complete witnessed-log bundle (decisions, empty declines,
// inclusion proof, signed tree head, witness cosignatures, manifest) whose
// decision record is one of Boundary's own conformance records must pass every
// verifier check. The records carry request_id, a check object, schema_version
// "2" route context, and signature members — the members verify-witnessed
// v0.14.0 rejected, which failed decision_record_integrity and
// manifest:decisions.jsonl for the whole bundle.
//
// The bundle is assembled here from the wire contract (leaf input, tree-head
// and cosignature signing messages, manifest registration) with fresh
// deterministic keys; the decision records are the corpus bytes produced by
// governance.ComputeDecisionHash. The production exporter lives outside this
// repository.
func TestWitnessedBundleWithEveryRecordMember(t *testing.T) {
	for _, vector := range []string{
		"v1_request_id",
		"v1_check_indeterminate",
		"v2_route_context",
		"v1_signed_record",
		"v2_all_members",
		"v1_mode_classified",
	} {
		t.Run(vector, func(t *testing.T) {
			dir, keys := buildWitnessedBundle(t, vector)
			results, err := VerifyDir(dir, keys)
			if err != nil {
				t.Fatalf("VerifyDir: %v", err)
			}
			if len(results.Checks) != 12 {
				t.Fatalf("got %d checks, want 12 (1 record, 5 manifest, leaf, inclusion, tenant, sth, 2 witnesses): %+v", len(results.Checks), results.Checks)
			}
			for _, check := range results.Checks {
				if check.Status != StatusPass {
					t.Errorf("check %s = %s, want pass (all checks must pass for a bundle holding %s)", check.ID, check.Status, vector)
				}
			}
		})
	}
}

func e2eKey(label string) (ed25519.PrivateKey, string) {
	seed := sha256.Sum256([]byte("verify-witnessed-e2e:" + label))
	private := ed25519.NewKeyFromSeed(seed[:])
	return private, "ed25519:" + base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
}

func e2eSign(private ed25519.PrivateKey, message []byte) string {
	return "ed25519:" + base64.StdEncoding.EncodeToString(ed25519.Sign(private, message))
}

func e2eSidecar(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}
	return append(encoded, '\n')
}

func buildWitnessedBundle(t *testing.T, vector string) (string, *KeySet) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	vectorPath := filepath.Join(filepath.Dir(thisFile), "..", "testdata", "verifier-vectors", vector+".json")
	raw, err := os.ReadFile(vectorPath) // #nosec G304 -- fixed test-data path.
	if err != nil {
		t.Fatalf("read vector: %v", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		t.Fatalf("compact vector: %v", err)
	}
	decisionLine := append(compact.Bytes(), '\n')

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("decode vector: %v", err)
	}
	str := func(name string) string {
		value, _ := generic[name].(string)
		return value
	}
	decisionHash, action, mode := str("decision_hash"), str("action"), str("decision_mode")
	signatureKeyID := str("signature_key_id")

	const (
		tenant     = "00000000-0000-4000-8000-0000000000e2"
		logID      = "witnessed-log-e2e-tenant"
		treeHeadID = "018f7f8a-6f10-7f40-8000-0000000000e2"
		issuedAt   = "2026-07-13T12:00:00Z"
	)
	sthPrivate, sthPublic := e2eKey("sth")
	alphaPrivate, alphaPublic := e2eKey("witness-alpha")
	betaPrivate, betaPublic := e2eKey("witness-beta")
	const sthKeyID, alphaKeyID, betaKeyID = "ed25519:e2e-sth", "ed25519:e2e-witness-alpha", "ed25519:e2e-witness-beta"
	keys := NewKeySet()
	for _, entry := range []struct {
		id, public string
		role       KeyRole
	}{
		{sthKeyID, sthPublic, RoleFulcrumSTH},
		{alphaKeyID, alphaPublic, RoleWitness},
		{betaKeyID, betaPublic, RoleWitness},
	} {
		if err := keys.AddEncoded(entry.id, entry.public, entry.role); err != nil {
			t.Fatalf("add key %s: %v", entry.id, err)
		}
	}

	// A one-leaf tree: the root is the leaf hash and the audit path is empty.
	leaf := tlog.RecordHash(leafInput("decision_hash", decisionHash))
	hashText := func(h tlog.Hash) string { return "sha256:" + hex.EncodeToString(h[:]) }
	root := hashText(leaf)

	proof := inclusionProof{
		SchemaVersion:  "inclusion-proof-v1",
		LogID:          logID,
		TreeHeadID:     treeHeadID,
		SourceField:    "decision_hash",
		SourceHash:     decisionHash,
		MerkleLeafHash: root,
		LeafIndex:      0,
		TreeSize:       1,
		AuditPath:      []string{},
		DecisionMode:   mode,
		CoverageScope:  []string{"grpc.explicitly_signaled_envelope_outcomes"},
		InvariantCitation: invariantCitation{
			Theorem: "budget_safety_guarantee", Module: "BasicInvariants.lean",
			ProofsTag: "v0.2.0", TheoremStatus: "Proved", RuntimeStatus: "Implemented",
		},
		ProofReference: proofReference{DOI: "10.5281/zenodo.19900714", TheoremInventoryID: "budget_safety_guarantee"},
	}

	head := treeHead{
		SchemaVersion: "tree-head-v1", TreeHeadID: treeHeadID, LogID: logID, TenantID: tenant,
		TreeSize: 1, RootHash: root, IssuedAt: issuedAt, PrevTreeSize: 0, PrevRootHash: "",
		SignatureKeyID: sthKeyID,
	}
	headMessage, err := json.Marshal(canonicalTreeHead{
		SchemaVersion: head.SchemaVersion, LogID: head.LogID, TenantID: head.TenantID, TreeSize: head.TreeSize,
		RootHash: head.RootHash, IssuedAt: head.IssuedAt, PrevTreeSize: head.PrevTreeSize,
		PrevRootHash: head.PrevRootHash, SignatureKeyID: head.SignatureKeyID,
	})
	if err != nil {
		t.Fatal(err)
	}
	head.Signature = e2eSign(sthPrivate, headMessage)

	cosign := func(witnessID string, private ed25519.PrivateKey, cosignedAt string) witnessCosignature {
		item := witnessCosignature{
			SchemaVersion: "witness-cosignature-v1", WitnessID: witnessID, LogID: logID, TreeSize: 1,
			RootHash: root, FulcrumKeyID: sthKeyID, CosignedAt: cosignedAt,
		}
		message, err := json.Marshal(canonicalWitnessCosignature{
			SchemaVersion: item.SchemaVersion, WitnessID: item.WitnessID, LogID: item.LogID, TreeSize: item.TreeSize,
			RootHash: item.RootHash, FulcrumKeyID: item.FulcrumKeyID, CosignedAt: item.CosignedAt,
		})
		if err != nil {
			t.Fatal(err)
		}
		item.Cosignature = e2eSign(private, message)
		return item
	}
	aggregate := cosignatureAggregate{
		SchemaVersion: "witness-cosignatures-v1", TreeHeadID: treeHeadID,
		ConfiguredWitnesses: []configuredWitness{
			{WitnessID: "witness-alpha", WitnessKeyID: alphaKeyID},
			{WitnessID: "witness-beta", WitnessKeyID: betaKeyID},
		},
		PresentCosignatures: 2,
		Cosignatures: []witnessCosignature{
			cosign("witness-alpha", alphaPrivate, "2026-07-13T12:00:01Z"),
			cosign("witness-beta", betaPrivate, "2026-07-13T12:00:02Z"),
		},
	}

	files := map[string][]byte{
		decisionsFile:    decisionLine,
		declinesFile:     {},
		proofFile:        e2eSidecar(t, proof),
		treeHeadFile:     e2eSidecar(t, head),
		cosignaturesFile: e2eSidecar(t, aggregate),
	}
	registrations := make([]manifestFile, 0, len(registeredBundleFiles))
	for _, name := range registeredBundleFiles {
		lines := []manifestLineHash{}
		data := files[name]
		for i, line := range bytes.SplitAfter(data, []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			lines = append(lines, manifestLineHash{Line: i + 1, SHA256: exactLineSHA256(line)})
		}
		registrations = append(registrations, manifestFile{Name: name, Lines: len(lines), LineSHA256: lines})
	}

	signing := manifestSigning{}
	if signatureKeyID != "" {
		signing = manifestSigning{AnySigned: true, SignedCount: 1, KeyIDs: []string{signatureKeyID}}
	}
	bundleManifest := manifest{
		SchemaVersion: "fulcrum.receipt_bundle_manifest.v1",
		TenantID:      tenant,
		Window:        manifestWindow{From: "2026-07-13T12:00:00Z", To: "2026-07-13T12:01:00Z"},
		GeneratedAt:   "2026-07-13T12:01:00Z",
		Exporter: manifestExporter{
			ExporterVersion: "e2e-v1", GoVersion: "fixture", BoundaryWireContractVersion: "pinned-verifier-vector",
		},
		Decisions: manifestCounts{
			Total:                1,
			ByVerdict:            map[string]int{action: 1},
			ByDecisionMode:       map[string]int{mode: 1},
			ByVerificationStatus: map[string]int{"verified": 1},
		},
		DeclinesCount: 0,
		DeclinesNote:  "End-to-end fixture bundle contains no decline record.",
		Signing:       signing,
		Index: []manifestIndex{{
			Line: 1, Source: "boundary_audit", DecisionHash: decisionHash, Action: action, DecisionMode: mode,
			VerificationStatus: "verified", Signed: signatureKeyID != "", SignatureKeyID: signatureKeyID,
		}},
		WitnessedLog: witnessedLogCounts{ConfiguredWitnesses: 2, PresentCosignatures: 2},
		Files:        registrations,
	}
	manifestBytes, err := json.Marshal(bundleManifest)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	files[manifestFilename] = append(manifestBytes, '\n')
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir, keys
}
