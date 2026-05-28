# Secure GitHub Live Conformance Harness

This package verifies sanitized evidence from an operator-owned Secure GitHub
live conformance run. It skips by default so ordinary CI and local development
do not call GitHub or require credentials.

## Default Run

```bash
go test ./tests/conformance/secure_github/ -v -timeout 5m
```

Expected result: all tests are skipped with exit code 0 because
`BOUNDARY_GITHUB_CONFORMANCE` is not set.

## Live Evidence Run

Run the denied write-after-taint conformance command, then point this harness at
the sanitized transcript:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true \
BOUNDARY_GITHUB_APP_ID=... \
BOUNDARY_GITHUB_INSTALLATION_ID=... \
BOUNDARY_GITHUB_PRIVATE_KEY_PATH=/absolute/path/to/app.pem \
BOUNDARY_GITHUB_OWNER=... \
BOUNDARY_GITHUB_REPO=... \
BOUNDARY_GITHUB_ISSUE_NUMBER=... \
boundary secure github conformance denied-write --out /tmp/boundary-secure-github

BOUNDARY_GITHUB_CONFORMANCE=true \
BOUNDARY_GITHUB_TRANSCRIPT=/tmp/boundary-secure-github/denied-write-after-taint.sanitized.json \
go test ./tests/conformance/secure_github/ -v -timeout 5m
```

The harness verifies:

- real GitHub issue read evidence was captured through GitHub App auth;
- the read marked the session tainted from `github.issue_body`;
- the protected write after taint was denied;
- `upstream_called=false`;
- `github_mutation_called=false`;
- decision record evidence is present;
- transcript evidence is sanitized.

## Transcript Safety

NEVER commit raw transcripts. The conformance command writes sanitized
transcripts containing hashes and booleans only. Raw GitHub issue bodies,
installation tokens, bearer tokens, private keys, private key paths, and
operator credentials must not appear in transcript files.

Ignored raw transcript patterns are declared in the repository `.gitignore`.
