package governance

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

type ReceiptSigner interface {
	KeyID() string
	Sign(record DecisionRecordV1) (string, error)
}

type Ed25519ReceiptSigner struct {
	ID         string
	PrivateKey ed25519.PrivateKey
}

func (s Ed25519ReceiptSigner) KeyID() string {
	return s.ID
}

func (s Ed25519ReceiptSigner) Sign(record DecisionRecordV1) (string, error) {
	if len(s.PrivateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid ed25519 private key")
	}
	hash := ComputeDecisionHash(record)
	signature := ed25519.Sign(s.PrivateKey, []byte(hash))
	return "ed25519:" + base64.StdEncoding.EncodeToString(signature), nil
}
