package verifier

import (
	"crypto/sha256"

	"golang.org/x/mod/sumdb/tlog"
)

func leafInput(sourceField, sourceHash string) []byte {
	input := make([]byte, 0, len("fulcrum:witnessed-log:leaf:v1")+len(sourceField)+len(sourceHash)+2)
	input = append(input, "fulcrum:witnessed-log:leaf:v1"...)
	input = append(input, 0)
	input = append(input, sourceField...)
	input = append(input, 0)
	input = append(input, sourceHash...)
	return input
}

func verifySourceToLeaf(proof inclusionProof) bool {
	leaf, err := parseHash(proof.MerkleLeafHash)
	if err != nil {
		return false
	}
	return tlog.RecordHash(leafInput(proof.SourceField, proof.SourceHash)) == leaf
}

func verifyInclusion(proof inclusionProof, head treeHead) bool {
	if proof.LogID != head.LogID || proof.TreeHeadID != head.TreeHeadID || proof.TreeSize != head.TreeSize {
		return false
	}
	leaf, err := parseHash(proof.MerkleLeafHash)
	if err != nil {
		return false
	}
	root, err := parseHash(head.RootHash)
	if err != nil {
		return false
	}
	path := make(tlog.RecordProof, 0, len(proof.AuditPath))
	for _, encoded := range proof.AuditPath {
		hash, err := parseHash(encoded)
		if err != nil {
			return false
		}
		path = append(path, hash)
	}
	return tlog.CheckRecord(path, proof.TreeSize, root, proof.LeafIndex, leaf) == nil
}

func exactLineSHA256(line []byte) string {
	digest := sha256.Sum256(line)
	const hexDigits = "0123456789abcdef"
	encoded := make([]byte, len(digest)*2)
	for i, value := range digest {
		encoded[i*2] = hexDigits[value>>4]
		encoded[i*2+1] = hexDigits[value&0x0f]
	}
	return string(encoded)
}
