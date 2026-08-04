package verifier

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// KeyRole constrains a key to the artifact class it may verify.
type KeyRole string

const (
	RoleFulcrumSTH KeyRole = "fulcrum_sth"
	RoleWitness    KeyRole = "witness"
)

type keyEntry struct {
	role KeyRole
	key  ed25519.PublicKey
}

// KeySet is a role-aware set of trusted Ed25519 public keys.
type KeySet struct {
	keys map[string]keyEntry
}

type keyringDocument struct {
	SchemaVersion string                  `json:"schema_version"`
	Keys          map[string]keyringEntry `json:"keys"`
}

type keyringEntry struct {
	Role      KeyRole `json:"role"`
	PublicKey string  `json:"public_key"`
}

// NewKeySet returns an empty role-aware key set.
func NewKeySet() *KeySet {
	return &KeySet{keys: make(map[string]keyEntry)}
}

// Len returns the number of unique key IDs in the set.
func (k *KeySet) Len() int {
	if k == nil {
		return 0
	}
	return len(k.keys)
}

// LoadKeyring strictly decodes a witnessed-public-key-map-v1 file.
func LoadKeyring(path string) (*KeySet, error) {
	data, err := readRegularFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read keyring: %w", err)
	}
	var document keyringDocument
	if err := decodeStrictRequired(data, &document, "schema_version", "keys"); err != nil {
		return nil, fmt.Errorf("decode keyring: %w", err)
	}
	if document.SchemaVersion != "witnessed-public-key-map-v1" {
		return nil, fmt.Errorf("keyring schema_version = %q, want %q", document.SchemaVersion, "witnessed-public-key-map-v1")
	}
	if document.Keys == nil {
		return nil, fmt.Errorf("keyring keys must be an object")
	}

	set := NewKeySet()
	ids := make([]string, 0, len(document.Keys))
	for id := range document.Keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		entry := document.Keys[id]
		if err := set.AddEncoded(id, entry.PublicKey, entry.Role); err != nil {
			return nil, fmt.Errorf("keyring key %q: %w", id, err)
		}
	}
	return set, nil
}

// AddSpec parses and adds KEY_ID=ed25519:BASE64 using the supplied role.
func (k *KeySet) AddSpec(spec string, role KeyRole) error {
	id, encoded, ok := strings.Cut(spec, "=")
	if !ok || id == "" || encoded == "" {
		return fmt.Errorf("public key must use KEY_ID=ed25519:BASE64")
	}
	return k.AddEncoded(id, encoded, role)
}

// AddEncoded validates and adds one encoded public key. An identical repeated
// mapping is accepted; a role or byte conflict for the same ID is rejected.
func (k *KeySet) AddEncoded(id, encoded string, role KeyRole) error {
	if k == nil {
		return fmt.Errorf("key set is nil")
	}
	if k.keys == nil {
		k.keys = make(map[string]keyEntry)
	}
	if err := validateKeyID(id); err != nil {
		return err
	}
	if role != RoleFulcrumSTH && role != RoleWitness {
		return fmt.Errorf("unsupported key role %q", role)
	}
	key, err := parseEd25519Value(encoded, ed25519.PublicKeySize, "public key")
	if err != nil {
		return err
	}
	entry := keyEntry{role: role, key: ed25519.PublicKey(key)}
	if existing, ok := k.keys[id]; ok {
		if existing.role != entry.role || !bytes.Equal(existing.key, entry.key) {
			return fmt.Errorf("conflicting duplicate key ID %q", id)
		}
		return nil
	}
	k.keys[id] = entry
	return nil
}

// Merge adds every key from another set with duplicate-conflict protection.
func (k *KeySet) Merge(other *KeySet) error {
	if other == nil {
		return nil
	}
	ids := make([]string, 0, len(other.keys))
	for id := range other.keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		entry := other.keys[id]
		encoded := "ed25519:" + base64.StdEncoding.EncodeToString(entry.key)
		if err := k.AddEncoded(id, encoded, entry.role); err != nil {
			return err
		}
	}
	return nil
}

func (k *KeySet) lookup(id string, role KeyRole) (ed25519.PublicKey, bool) {
	if k == nil {
		return nil, false
	}
	entry, ok := k.keys[id]
	if !ok || entry.role != role {
		return nil, false
	}
	return entry.key, true
}

func validateKeyID(id string) error {
	const prefix = "ed25519:"
	if !strings.HasPrefix(id, prefix) || len(id) == len(prefix) {
		return fmt.Errorf("key ID %q must use ed25519:<key-id>", id)
	}
	for _, r := range id[len(prefix):] {
		if r < 0x21 || r > 0x7e || r == '=' {
			return fmt.Errorf("key ID %q contains an invalid character", id)
		}
	}
	return nil
}

func parseEd25519Value(value string, size int, label string) ([]byte, error) {
	const prefix = "ed25519:"
	if !strings.HasPrefix(value, prefix) {
		return nil, fmt.Errorf("%s must use ed25519:BASE64", label)
	}
	encoded := strings.TrimPrefix(value, prefix)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	if len(decoded) != size {
		return nil, fmt.Errorf("%s decodes to %d bytes, want %d", label, len(decoded), size)
	}
	if base64.StdEncoding.EncodeToString(decoded) != encoded {
		return nil, fmt.Errorf("%s is not canonical base64", label)
	}
	return decoded, nil
}
