package governance

// CanonicalJSONBytes returns the RFC 8785 (JCS) canonical encoding of value,
// the identical canonicalization behind every stable hash in this package
// (see mustCanonicalJSON). It is exported so sibling packages (e.g.
// governance/proofreceipt) can hash witness inputs to a digest the stock JCS
// verifiers in verifiers/ reproduce, without duplicating canonicalization. It
// panics only on inputs json.Marshal cannot encode, matching mustCanonicalJSON.
func CanonicalJSONBytes(value any) []byte {
	return mustCanonicalJSON(value)
}
