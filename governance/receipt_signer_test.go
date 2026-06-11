package governance

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedSeed is a deterministic 32-byte Ed25519 seed used across the signer
// tests so KeyIDs and signatures are reproducible.
var fixedSeed = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
}

func testSigner(t *testing.T) (Ed25519ReceiptSigner, ed25519.PublicKey) {
	t.Helper()
	signer, err := NewEd25519SignerFromSeed(fixedSeed, "")
	if err != nil {
		t.Fatalf("NewEd25519SignerFromSeed: %v", err)
	}
	pub := ed25519.NewKeyFromSeed(fixedSeed).Public().(ed25519.PublicKey)
	return signer, pub
}

// sampleRecord builds a fixed, deterministic decision record. The Timestamp is
// pinned so repeated calls produce a byte-identical record and a stable
// decision_hash (BuildDecisionRecord defaults an unset timestamp to time.Now,
// which would otherwise vary per call).
func sampleRecord() DecisionRecordV1 {
	return BuildDecisionRecord(AuditEvent{
		Transport:        TransportMCP,
		ToolName:         "query",
		Action:           "deny",
		Reason:           "blocked: DROP TABLE",
		PolicyBundleHash: "sha256:abc",
		RequestHash:      "sha256:def",
		TrustScore:       1,
		TrustState:       TrustStateTrusted.String(),
		Timestamp:        time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	})
}

func TestEd25519Signer_RoundTrip(t *testing.T) {
	signer, pub := testSigner(t)
	record := sampleRecord()

	sig, err := signer.Sign(record)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !strings.HasPrefix(sig, "ed25519:") {
		t.Fatalf("signature missing ed25519 prefix: %q", sig)
	}
	record.Signature = sig
	record.SignatureKeyID = signer.KeyID()

	if err := VerifyReceiptSignature(record, pub); err != nil {
		t.Fatalf("VerifyReceiptSignature should pass for a freshly signed record: %v", err)
	}
}

func TestEd25519Signer_TamperFails(t *testing.T) {
	signer, pub := testSigner(t)
	record := sampleRecord()

	sig, err := signer.Sign(record)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	record.Signature = sig

	// Mutate a covered field after signing. decision_hash recomputes to a new
	// value, so the signature (over the original decision_hash) must fail.
	record.Action = "allow"
	if err := VerifyReceiptSignature(record, pub); err == nil {
		t.Fatal("VerifyReceiptSignature must fail after a covered field is altered")
	}
}

func TestEd25519Signer_WrongKeyFails(t *testing.T) {
	signer, _ := testSigner(t)
	record := sampleRecord()

	sig, err := signer.Sign(record)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	record.Signature = sig

	otherSeed := make([]byte, ed25519.SeedSize)
	for i := range otherSeed {
		otherSeed[i] = byte(0xff - i)
	}
	otherPub := ed25519.NewKeyFromSeed(otherSeed).Public().(ed25519.PublicKey)
	if err := VerifyReceiptSignature(record, otherPub); err == nil {
		t.Fatal("VerifyReceiptSignature must fail against a different public key")
	}
}

func TestEd25519Signer_NilSignerRecordUnchangedBytes(t *testing.T) {
	// The nil-signer path must leave the emitted record byte-identical to the
	// unsigned record and the decision_hash unchanged. Signing must not perturb
	// decision_hash either, because ComputeDecisionHash blanks the signature.
	unsigned := sampleRecord()

	signer, _ := testSigner(t)
	sig, err := signer.Sign(unsigned)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	signed := sampleRecord()
	signed.Signature = sig
	signed.SignatureKeyID = signer.KeyID()

	if ComputeDecisionHash(unsigned) != ComputeDecisionHash(signed) {
		t.Fatal("signing must not change decision_hash")
	}
	if signed.DecisionHash != unsigned.DecisionHash {
		t.Fatalf("stored decision_hash diverged: signed=%s unsigned=%s", signed.DecisionHash, unsigned.DecisionHash)
	}

	// A record with no signature fields set marshals byte-for-byte the same as a
	// record built with no signer (signature/signature_key_id are omitempty).
	unsignedBytes, err := json.Marshal(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := json.Marshal(sampleRecord())
	if err != nil {
		t.Fatal(err)
	}
	if string(unsignedBytes) != string(rebuilt) {
		t.Fatalf("unsigned record bytes are not stable:\n%s\n%s", unsignedBytes, rebuilt)
	}
	if strings.Contains(string(unsignedBytes), "signature") {
		t.Fatalf("unsigned record must not emit signature keys: %s", unsignedBytes)
	}
}

func TestEd25519Signer_InvalidPrivateKey(t *testing.T) {
	signer := Ed25519ReceiptSigner{ID: "k", PrivateKey: ed25519.PrivateKey{0x00}}
	if _, err := signer.Sign(sampleRecord()); err == nil {
		t.Fatal("Sign must error on a wrong-sized private key")
	}
}

func TestNewEd25519SignerFromSeed_KeyIDDerivation(t *testing.T) {
	signer, err := NewEd25519SignerFromSeed(fixedSeed, "")
	if err != nil {
		t.Fatalf("NewEd25519SignerFromSeed: %v", err)
	}
	pub := ed25519.NewKeyFromSeed(fixedSeed).Public().(ed25519.PublicKey)
	want := KeyIDFromPublicKey(pub)
	if signer.KeyID() != want {
		t.Fatalf("derived KeyID = %q, want %q", signer.KeyID(), want)
	}
	if !strings.HasPrefix(signer.KeyID(), "ed25519:") || len(signer.KeyID()) != len("ed25519:")+16 {
		t.Fatalf("KeyID fingerprint shape unexpected: %q", signer.KeyID())
	}

	explicit, err := NewEd25519SignerFromSeed(fixedSeed, "my-key")
	if err != nil {
		t.Fatalf("NewEd25519SignerFromSeed explicit: %v", err)
	}
	if explicit.KeyID() != "my-key" {
		t.Fatalf("explicit KeyID = %q, want my-key", explicit.KeyID())
	}
}

func TestNewEd25519SignerFromSeed_WrongLength(t *testing.T) {
	if _, err := NewEd25519SignerFromSeed([]byte{0x00, 0x01}, ""); err == nil {
		t.Fatal("NewEd25519SignerFromSeed must reject a short seed")
	}
}

func TestNewEd25519SignerFromSeedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.hex")
	// Trailing newline must be tolerated.
	if err := os.WriteFile(path, []byte(hex.EncodeToString(fixedSeed)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	signer, err := NewEd25519SignerFromSeedFile(path, "")
	if err != nil {
		t.Fatalf("NewEd25519SignerFromSeedFile: %v", err)
	}
	// Equivalent to loading the seed directly.
	direct, err := NewEd25519SignerFromSeed(fixedSeed, "")
	if err != nil {
		t.Fatal(err)
	}
	if signer.KeyID() != direct.KeyID() {
		t.Fatalf("seed-file KeyID %q != direct KeyID %q", signer.KeyID(), direct.KeyID())
	}
}

func TestNewEd25519SignerFromSeedFile_Errors(t *testing.T) {
	dir := t.TempDir()

	if _, err := NewEd25519SignerFromSeedFile(filepath.Join(dir, "absent"), ""); err == nil {
		t.Fatal("missing seed file must error")
	}

	short := filepath.Join(dir, "short.hex")
	if err := os.WriteFile(short, []byte("deadbeef"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEd25519SignerFromSeedFile(short, ""); err == nil {
		t.Fatal("short seed file must error")
	}

	nothex := filepath.Join(dir, "nothex.hex")
	if err := os.WriteFile(nothex, []byte(strings.Repeat("z", 64)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEd25519SignerFromSeedFile(nothex, ""); err == nil {
		t.Fatal("non-hex seed file must error")
	}
}

func TestParseEd25519PublicKey(t *testing.T) {
	pub := ed25519.NewKeyFromSeed(fixedSeed).Public().(ed25519.PublicKey)
	hexKey := hex.EncodeToString(pub)

	// Literal hex.
	got, err := ParseEd25519PublicKey(hexKey)
	if err != nil {
		t.Fatalf("ParseEd25519PublicKey hex: %v", err)
	}
	if !got.Equal(pub) {
		t.Fatal("literal hex public key did not round-trip")
	}

	// File form.
	dir := t.TempDir()
	path := filepath.Join(dir, "key.pub")
	if err := os.WriteFile(path, []byte(hexKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = ParseEd25519PublicKey(path)
	if err != nil {
		t.Fatalf("ParseEd25519PublicKey file: %v", err)
	}
	if !got.Equal(pub) {
		t.Fatal("file public key did not round-trip")
	}

	if _, err := ParseEd25519PublicKey("not-a-key"); err == nil {
		t.Fatal("garbage public key must error")
	}
}

func TestVerifyReceiptSignature_FailClosedShapes(t *testing.T) {
	signer, pub := testSigner(t)
	record := sampleRecord()
	sig, err := signer.Sign(record)
	if err != nil {
		t.Fatal(err)
	}

	// Missing signature.
	if err := VerifyReceiptSignature(record, pub); err == nil {
		t.Fatal("missing signature must fail closed")
	}

	// Wrong public-key length.
	signed := record
	signed.Signature = sig
	if err := VerifyReceiptSignature(signed, ed25519.PublicKey{0x00}); err == nil {
		t.Fatal("wrong-length public key must fail closed")
	}

	// Not an ed25519-prefixed signature.
	noPrefix := record
	noPrefix.Signature = strings.TrimPrefix(sig, "ed25519:")
	if err := VerifyReceiptSignature(noPrefix, pub); err == nil {
		t.Fatal("non-ed25519 signature must fail closed")
	}

	// Malformed base64.
	badB64 := record
	badB64.Signature = "ed25519:!!!notbase64!!!"
	if err := VerifyReceiptSignature(badB64, pub); err == nil {
		t.Fatal("malformed base64 signature must fail closed")
	}

	// Right encoding, wrong length payload.
	shortSig := record
	shortSig.Signature = "ed25519:AAAA"
	if err := VerifyReceiptSignature(shortSig, pub); err == nil {
		t.Fatal("wrong-length signature payload must fail closed")
	}
}

func TestPublishParseRejection_SignedWhenSignerConfigured(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	signer, err := NewEd25519SignerFromSeed(seed, "")
	if err != nil {
		t.Fatal(err)
	}
	auditor := &collectingAuditor{}
	pipeline := NewPipeline(PipelineConfig{
		GatewayVersion: "test-version",
		ReceiptSigner:  signer,
	}, nil, nil, auditor)

	pipeline.PublishParseRejection(context.Background(), ParseRejectionEvent{
		Adapter:         TransportMCP,
		RawPayload:      []byte(`{"not":"parseable as a tool call"}`),
		RejectionReason: "malformed envelope",
	})

	events := auditor.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	event := events[0]
	if event.Signature == "" || event.SignatureKeyID == "" {
		t.Fatalf("parse-rejection event must carry a signature when a signer is configured: %+v", event)
	}
	record := BuildDecisionRecord(event)
	pub := signer.PrivateKey.Public().(ed25519.PublicKey)
	if err := VerifyReceiptSignature(record, pub); err != nil {
		t.Fatalf("signed parse-rejection record must verify: %v", err)
	}
}
