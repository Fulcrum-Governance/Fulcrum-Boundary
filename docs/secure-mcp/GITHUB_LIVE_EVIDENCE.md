# Secure GitHub Live Evidence

Live conformance writes sanitized transcripts for operator review. Transcripts
are designed to prove the conformance result without storing raw GitHub issue
content or credentials.

## Current Release Evidence Status

As of v0.5.0 packaging, the opt-in live conformance harness is implemented and
covered by automated skip, missing-environment fail-closed, transcript
sanitization, and no-mutation tests. An operator-owned live run against a real
GitHub repository has not been recorded in release evidence yet.

To convert that partial evidence into recorded operator evidence, run the live
conformance with operator-owned GitHub App credentials and record the sanitized
transcript hash.

## Transcript Shape

Sanitized transcripts include:

- schema version;
- mode;
- generated timestamp;
- profile id and profile status;
- owner, repository, and issue number;
- taint source type;
- SHA-256 hash of the live issue content used for evidence;
- expected action and actual action;
- matched rule and decision hash;
- `read_upstream_called`;
- `upstream_called`;
- `github_mutation_called`;
- `raw_content_included`;
- `credential_data_included`;
- transcript SHA-256 hash.

## Transcript Safety Rules

Do not commit:

- raw transcripts;
- raw HTTP request or response logs;
- issue bodies;
- JWTs;
- installation tokens;
- private keys;
- private key paths;
- operator emails or personal data.

Commit only sanitized `.sanitized.json` files if a release explicitly needs
evidence, and prefer storing transcript hashes in docs over storing full
payloads.

## Validation Harness

The offline transcript validator skips by default:

```bash
go test ./tests/conformance/secure_github/ -v -timeout 5m
```

After a live denied-write run, point the harness at the sanitized transcript:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true \
BOUNDARY_GITHUB_TRANSCRIPT=/tmp/boundary-secure-github/denied-write-after-taint.sanitized.json \
go test ./tests/conformance/secure_github/ -v -timeout 5m
```

The validator checks read taint evidence, deny/no-mutation evidence, decision
evidence, and sanitized transcript fields.
