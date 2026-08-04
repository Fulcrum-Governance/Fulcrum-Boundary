package verifier

import (
	"crypto/ed25519"
	"encoding/json"
)

type canonicalTreeHead struct {
	SchemaVersion  string `json:"schema_version"`
	LogID          string `json:"log_id"`
	TenantID       string `json:"tenant_id"`
	TreeSize       int64  `json:"tree_size"`
	RootHash       string `json:"root_hash"`
	IssuedAt       string `json:"issued_at"`
	PrevTreeSize   int64  `json:"prev_tree_size"`
	PrevRootHash   string `json:"prev_root_hash"`
	SignatureKeyID string `json:"signature_key_id"`
	Signature      string `json:"signature"`
}

type canonicalWitnessCosignature struct {
	SchemaVersion string `json:"schema_version"`
	WitnessID     string `json:"witness_id"`
	LogID         string `json:"log_id"`
	TreeSize      int64  `json:"tree_size"`
	RootHash      string `json:"root_hash"`
	FulcrumKeyID  string `json:"fulcrum_key_id"`
	CosignedAt    string `json:"cosigned_at"`
	Cosignature   string `json:"cosignature"`
}

func verifyTreeHeadSignature(head treeHead, keys *KeySet) bool {
	key, ok := keys.lookup(head.SignatureKeyID, RoleFulcrumSTH)
	if !ok {
		return false
	}
	signature, err := parseEd25519Value(head.Signature, ed25519.SignatureSize, "tree head signature")
	if err != nil {
		return false
	}
	message, err := json.Marshal(canonicalTreeHead{
		SchemaVersion:  head.SchemaVersion,
		LogID:          head.LogID,
		TenantID:       head.TenantID,
		TreeSize:       head.TreeSize,
		RootHash:       head.RootHash,
		IssuedAt:       head.IssuedAt,
		PrevTreeSize:   head.PrevTreeSize,
		PrevRootHash:   head.PrevRootHash,
		SignatureKeyID: head.SignatureKeyID,
		Signature:      "",
	})
	return err == nil && ed25519.Verify(key, message, signature)
}

func verifyWitnessSignature(item witnessCosignature, key ed25519.PublicKey) bool {
	signature, err := parseEd25519Value(item.Cosignature, ed25519.SignatureSize, "witness cosignature")
	if err != nil {
		return false
	}
	message, err := json.Marshal(canonicalWitnessCosignature{
		SchemaVersion: item.SchemaVersion,
		WitnessID:     item.WitnessID,
		LogID:         item.LogID,
		TreeSize:      item.TreeSize,
		RootHash:      item.RootHash,
		FulcrumKeyID:  item.FulcrumKeyID,
		CosignedAt:    item.CosignedAt,
		Cosignature:   "",
	})
	return err == nil && ed25519.Verify(key, message, signature)
}
