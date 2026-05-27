# Receipt-Grade Decision Records

Fulcrum Boundary decision records are receipt-grade when they include stable
hashes for the governed request, the policy bundle, and the emitted decision.
The signature fields are schema-supported but optional; unsigned records can
still be checked for tampering against their hashes.

## Verification

Use `boundary verify-record` against a JSON decision record:

```bash
boundary verify-record \
  --request request.json \
  --policies examples/mcp-postgres-gateway/policies \
  --binary-digest sha256:<build-digest> \
  record.json
```

The command verifies:

- `decision_hash` by recomputing the canonical record hash.
- `request_hash` from the canonical JSON request body when `--request` is set.
- `policy_bundle_hash` from canonical YAML policy content when `--policies` is set.
- `boundary_build_digest` when `--binary-digest` is set.

## Hash Inputs

Policy hashes are computed from YAML content after YAML-to-JSON normalization.
File modification time, directory order, and file metadata are not part of the
hash. Request hashes are computed from canonical JSON so key ordering does not
change the digest.

Malformed requests that cannot enter pipeline evaluation emit
`event_type=parse_rejected` records with `raw_shape_hash`. These records prove
that Boundary observed and rejected an input shape even though no governed tool
request was built.

## Signature Fields

The v1 schema includes `signature` and `signature_key_id` fields for operators
that attach their own signing layer. Boundary's local verification command does
not require signatures; it validates the stable hashes that make tampering
detectable.
