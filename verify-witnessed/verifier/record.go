package verifier

import (
	"crypto/sha256"
	"encoding/hex"
)

func computeDecisionHash(record decisionRecordV1) (string, error) {
	record.RecordID = ""
	record.DecisionHash = ""
	record.Signature = ""
	record.SignatureKeyID = ""
	canonical, err := canonicalJSON(record)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
