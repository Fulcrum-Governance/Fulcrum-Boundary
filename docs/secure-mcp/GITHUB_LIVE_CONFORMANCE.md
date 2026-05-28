# Secure GitHub Live Conformance

Secure GitHub live conformance is an opt-in preview harness. It uses
operator-owned GitHub App credentials to read real GitHub context, mark the
session tainted, and deny a protected write-after-taint action before any
upstream GitHub mutation client call executes.

This does not make Secure GitHub production. It proves the live-read and
no-mutation conformance path for the configured test repository.

## Claim Boundary

Allowed public copy:

> Secure GitHub can use real GitHub App credentials to read real GitHub
> context, mark the session tainted, and deny a protected write-after-taint
> action before any upstream GitHub mutation client call executes.

Do not claim:

- Do not claim Secure GitHub is production.
- Do not claim Boundary fully secures GitHub.
- Do not claim Boundary prevents every malicious issue.
- Do not claim Boundary prevents universal prompt injection.
- Do not claim live conformance proves deployment bypass resistance.
- Do not claim conformance mutates live repositories by default.

## Commands

Read conformance:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true \
BOUNDARY_GITHUB_APP_ID=... \
BOUNDARY_GITHUB_INSTALLATION_ID=... \
BOUNDARY_GITHUB_PRIVATE_KEY_PATH=/absolute/path/to/app.pem \
BOUNDARY_GITHUB_OWNER=... \
BOUNDARY_GITHUB_REPO=... \
BOUNDARY_GITHUB_ISSUE_NUMBER=... \
boundary secure github conformance read --out /tmp/boundary-secure-github
```

Denied write-after-taint conformance:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true \
BOUNDARY_GITHUB_APP_ID=... \
BOUNDARY_GITHUB_INSTALLATION_ID=... \
BOUNDARY_GITHUB_PRIVATE_KEY_PATH=/absolute/path/to/app.pem \
BOUNDARY_GITHUB_OWNER=... \
BOUNDARY_GITHUB_REPO=... \
BOUNDARY_GITHUB_ISSUE_NUMBER=... \
boundary secure github conformance denied-write --out /tmp/boundary-secure-github
```

Both checks:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true \
BOUNDARY_GITHUB_APP_ID=... \
BOUNDARY_GITHUB_INSTALLATION_ID=... \
BOUNDARY_GITHUB_PRIVATE_KEY_PATH=/absolute/path/to/app.pem \
BOUNDARY_GITHUB_OWNER=... \
BOUNDARY_GITHUB_REPO=... \
BOUNDARY_GITHUB_ISSUE_NUMBER=... \
boundary secure github conformance all --out /tmp/boundary-secure-github
```

Without `BOUNDARY_GITHUB_CONFORMANCE=true`, the command skips without network
calls.

## Expected Denied-Write Output

The denied-write run must report:

```text
profile: secure-github
status: preview
mode: denied-write-after-taint
expected action: DENY
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
github_mutation_called=false
raw_content_included=false
credential_data_included=false
```

## Evidence Files

Sanitized transcripts are written to:

```text
.boundary/conformance/secure-github/
```

or the directory passed with `--out`.

See [GITHUB_LIVE_EVIDENCE.md](./GITHUB_LIVE_EVIDENCE.md) before preserving
transcripts.
