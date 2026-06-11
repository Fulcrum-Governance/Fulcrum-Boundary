package governance

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// ReceiptSigner produces a detached signature over a decision record. It is an
// OPT-IN integrity-for-key-holders mechanism: when a signer is configured on the
// pipeline, every emitted decision record carries a signature plus the signing
// key's identifier. A valid signature proves the record was signed by the holder
// of that key — it does NOT prove the verdict was correct, that the governed
// action executed or was prevented, and it does not solve key custody. The
// signature is computed over the record's decision_hash (the unkeyed SHA-256
// integrity digest), so it covers exactly what decision_hash covers and adds
// authorship for holders who manage keys. Signing is off by default; see
// docs/SIGNING.md.
type ReceiptSigner interface {
	// KeyID returns the identifier recorded in signature_key_id. It lets a
	// verifier select the public key to check a signature against; it is not a
	// secret and is not itself authenticated by the signature.
	KeyID() string
	// Sign returns the detached signature string ("ed25519:" + base64 over the
	// record's decision_hash). It must not mutate the record.
	Sign(record DecisionRecordV1) (string, error)
}

// Ed25519ReceiptSigner signs a decision record's decision_hash with an Ed25519
// private key. The zero value is unusable; construct it with a populated
// PrivateKey or via NewEd25519SignerFromSeed / NewEd25519SignerFromSeedFile.
type Ed25519ReceiptSigner struct {
	// ID is the value recorded in signature_key_id. When empty, the constructors
	// derive it as the public-key fingerprint (see KeyIDFromPublicKey).
	ID string
	// PrivateKey is the Ed25519 private key used to sign. It is sized
	// ed25519.PrivateKeySize; a wrong-sized key makes Sign return an error.
	PrivateKey ed25519.PrivateKey
}

// KeyID returns the configured signature_key_id for this signer.
func (s Ed25519ReceiptSigner) KeyID() string {
	return s.ID
}

// Sign returns "ed25519:" + base64(signature) over []byte(ComputeDecisionHash(record)).
// It signs the record's stable decision_hash string, so the signature is bound
// to the same canonical content decision_hash covers and is independent of the
// signature/record_id fields (which ComputeDecisionHash blanks). It returns an
// error only when the private key is not a valid Ed25519 key. It does not mutate
// record.
func (s Ed25519ReceiptSigner) Sign(record DecisionRecordV1) (string, error) {
	if len(s.PrivateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid ed25519 private key")
	}
	hash := ComputeDecisionHash(record)
	signature := ed25519.Sign(s.PrivateKey, []byte(hash))
	return "ed25519:" + base64.StdEncoding.EncodeToString(signature), nil
}

// KeyIDFromPublicKey derives a stable, non-secret key identifier from an Ed25519
// public key: "ed25519:" followed by the first 16 lowercase-hex characters of
// SHA-256(pub). It is a fingerprint prefix used as the default signature_key_id
// when an explicit id is not given, so a verifier can match a record to the key
// that should check it. It is not a secret and is not itself authenticated.
func KeyIDFromPublicKey(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "ed25519:" + hex.EncodeToString(sum[:])[:16]
}

// NewEd25519SignerFromSeed builds an Ed25519ReceiptSigner from a 32-byte Ed25519
// seed (ed25519.SeedSize). When id is empty, KeyID is derived from the public
// key via KeyIDFromPublicKey; otherwise id is used verbatim. It returns an error
// if the seed is not exactly ed25519.SeedSize bytes. The seed is secret key
// material: callers are responsible for its custody.
func NewEd25519SignerFromSeed(seed []byte, id string) (Ed25519ReceiptSigner, error) {
	if len(seed) != ed25519.SeedSize {
		return Ed25519ReceiptSigner{}, fmt.Errorf("ed25519 seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	keyID := id
	if keyID == "" {
		pub, ok := priv.Public().(ed25519.PublicKey)
		if !ok {
			return Ed25519ReceiptSigner{}, fmt.Errorf("ed25519 public key derivation failed")
		}
		keyID = KeyIDFromPublicKey(pub)
	}
	return Ed25519ReceiptSigner{ID: keyID, PrivateKey: priv}, nil
}

// NewEd25519SignerFromSeedFile reads a 64-hex-character Ed25519 seed from path
// and builds an Ed25519ReceiptSigner. The file must contain exactly 64 hex
// characters (32 bytes; surrounding whitespace is trimmed) and nothing else.
// When id is empty, KeyID is derived from the public key. It returns an error if
// the file cannot be read, is not 64 hex characters, or is not valid hex. The
// file is secret key material; 0600 permissions are recommended (see
// docs/SIGNING.md) but not enforced here.
func NewEd25519SignerFromSeedFile(path, id string) (Ed25519ReceiptSigner, error) {
	// #nosec G304 -- path is an operator-supplied seed file location; the file is
	// read, never executed, and the caller owns its custody.
	raw, err := os.ReadFile(path)
	if err != nil {
		return Ed25519ReceiptSigner{}, fmt.Errorf("read seed file %s: %w", path, err)
	}
	hexSeed := strings.TrimSpace(string(raw))
	if len(hexSeed) != ed25519.SeedSize*2 {
		return Ed25519ReceiptSigner{}, fmt.Errorf("seed file %s must contain %d hex characters, got %d", path, ed25519.SeedSize*2, len(hexSeed))
	}
	seed, err := hex.DecodeString(hexSeed)
	if err != nil {
		return Ed25519ReceiptSigner{}, fmt.Errorf("seed file %s is not valid hex: %w", path, err)
	}
	return NewEd25519SignerFromSeed(seed, id)
}

// ParseEd25519PublicKey accepts a public key supplied as either a 64-hex-character
// string (32 raw bytes) or a path to a file containing one, and returns the
// decoded ed25519.PublicKey. It first treats input as a literal hex key; if that
// is not 64 hex characters, it tries to read input as a file whose trimmed
// contents are 64 hex characters. It returns an error when neither form yields a
// 32-byte key. The public key is not secret.
func ParseEd25519PublicKey(input string) (ed25519.PublicKey, error) {
	if key, ok := decodeHexPublicKey(strings.TrimSpace(input)); ok {
		return key, nil
	}
	// #nosec G304 -- input is an operator-supplied public-key location; the file
	// is read for non-secret key material, never executed.
	raw, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("public key %q is not 64 hex chars and is not a readable file: %w", input, err)
	}
	if key, ok := decodeHexPublicKey(strings.TrimSpace(string(raw))); ok {
		return key, nil
	}
	return nil, fmt.Errorf("public key file %s must contain %d hex characters", input, ed25519.PublicKeySize*2)
}

// decodeHexPublicKey returns the decoded key and true when s is exactly
// PublicKeySize*2 valid hex characters, otherwise nil and false.
func decodeHexPublicKey(s string) (ed25519.PublicKey, bool) {
	if len(s) != ed25519.PublicKeySize*2 {
		return nil, false
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return nil, false
	}
	return ed25519.PublicKey(decoded), true
}

// VerifyReceiptSignature checks the record's detached signature over its
// recomputed decision_hash against pub. It fails closed: a missing or empty
// signature, a signature without the "ed25519:" prefix, malformed base64, a
// wrong-length signature, or a public key that is not ed25519.PublicKeySize all
// return an error, as does a cryptographically invalid signature. On success it
// returns nil, which proves the record was signed by the holder of pub over the
// content decision_hash covers — NOT that the verdict was correct or that the
// action executed or was prevented, and not that key custody is sound. It
// recomputes decision_hash from the record, so an altered record (which yields a
// new decision_hash) fails verification. It does not check decision_hash against
// the record's stored value; callers wanting both integrity and signature run
// VerifyDecisionRecord as well.
func VerifyReceiptSignature(record DecisionRecordV1, pub ed25519.PublicKey) error {
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("ed25519 public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}
	if record.Signature == "" {
		return fmt.Errorf("record carries no signature")
	}
	encoded, ok := strings.CutPrefix(record.Signature, "ed25519:")
	if !ok {
		return fmt.Errorf("signature is not an ed25519 signature: %q", record.Signature)
	}
	sig, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("signature is not valid base64: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("signature must be %d bytes, got %d", ed25519.SignatureSize, len(sig))
	}
	hash := ComputeDecisionHash(record)
	if !ed25519.Verify(pub, []byte(hash), sig) {
		return fmt.Errorf("signature does not verify against decision_hash %s for key", hash)
	}
	return nil
}
