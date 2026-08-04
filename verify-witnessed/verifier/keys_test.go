package verifier

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeySetMergesIdenticalAndRejectsConflicts(t *testing.T) {
	loaded := corpusKeySet(t)
	entry := loaded.keys["ed25519:fulcrum-sth-golden-v1"]
	encoded := "ed25519:" + base64.StdEncoding.EncodeToString(entry.key)
	spec := "ed25519:fulcrum-sth-golden-v1=" + encoded

	merged := NewKeySet()
	if err := merged.Merge(loaded); err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	if err := merged.AddSpec(spec, RoleFulcrumSTH); err != nil {
		t.Fatalf("identical AddSpec() error = %v", err)
	}
	if got, want := merged.Len(), loaded.Len(); got != want {
		t.Fatalf("merged Len() = %d, want %d", got, want)
	}

	otherKey := "ed25519:" + base64.StdEncoding.EncodeToString(make([]byte, 32))
	if err := merged.AddEncoded("ed25519:fulcrum-sth-golden-v1", otherKey, RoleFulcrumSTH); err == nil {
		t.Fatal("AddEncoded() accepted conflicting bytes")
	}
	if err := merged.AddEncoded("ed25519:fulcrum-sth-golden-v1", encoded, RoleWitness); err == nil {
		t.Fatal("AddEncoded() accepted conflicting role")
	}
}

func TestPublicKeySpecsRejectMalformedValues(t *testing.T) {
	validBytes := base64.StdEncoding.EncodeToString(make([]byte, 32))
	tests := []struct {
		name string
		spec string
		role KeyRole
	}{
		{name: "missing equals", spec: "ed25519:key", role: RoleWitness},
		{name: "empty ID", spec: "=ed25519:" + validBytes, role: RoleWitness},
		{name: "invalid ID prefix", spec: "key=ed25519:" + validBytes, role: RoleWitness},
		{name: "invalid ID character", spec: "ed25519:key=id=ed25519:" + validBytes, role: RoleWitness},
		{name: "invalid key prefix", spec: "ed25519:key=rsa:" + validBytes, role: RoleWitness},
		{name: "invalid base64", spec: "ed25519:key=ed25519:not-base64", role: RoleWitness},
		{name: "wrong length", spec: "ed25519:key=ed25519:AA==", role: RoleWitness},
		{name: "unsupported role", spec: "ed25519:key=ed25519:" + validBytes, role: "other"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := NewKeySet().AddSpec(test.spec, test.role); err == nil {
				t.Fatal("AddSpec() accepted malformed input")
			}
		})
	}
	if err := (*KeySet)(nil).AddEncoded("ed25519:key", "ed25519:"+validBytes, RoleWitness); err == nil {
		t.Fatal("nil KeySet accepted AddEncoded")
	}
}

func TestLoadKeyringIsStrict(t *testing.T) {
	valid := readTestFile(t, corpusPath(t, "public-keys-v1.json"))
	tests := map[string][]byte{
		"unknown field":       bytesBeforeFinalObject(t, valid, `,"unknown":true}`),
		"duplicate field":     []byte(strings.Replace(string(valid), `"schema_version":`, `"schema_version":"witnessed-public-key-map-v1","schema_version":`, 1)),
		"wrong schema":        []byte(strings.Replace(string(valid), "witnessed-public-key-map-v1", "witnessed-public-key-map-v2", 1)),
		"null keys":           []byte(`{"schema_version":"witnessed-public-key-map-v1","keys":null}`),
		"missing nested role": []byte(fmt.Sprintf(`{"schema_version":"witnessed-public-key-map-v1","keys":{"ed25519:key":{"public_key":"ed25519:%s"}}}`, base64.StdEncoding.EncodeToString(make([]byte, 32)))),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "keys.json")
			writeTestFile(t, path, data)
			if _, err := LoadKeyring(path); err == nil {
				t.Fatal("LoadKeyring() accepted malformed keyring")
			}
		})
	}

	path := filepath.Join(t.TempDir(), "keys.json")
	if err := os.Symlink(corpusPath(t, "public-keys-v1.json"), path); err != nil {
		t.Fatalf("create keyring symlink: %v", err)
	}
	if _, err := LoadKeyring(path); err == nil {
		t.Fatal("LoadKeyring() followed a symlink")
	}
}

func bytesBeforeFinalObject(t *testing.T, data []byte, suffix string) []byte {
	t.Helper()
	trimmed := strings.TrimSpace(string(data))
	if !strings.HasSuffix(trimmed, "}") {
		t.Fatal("test JSON is not an object")
	}
	return []byte(strings.TrimSuffix(trimmed, "}") + suffix)
}
