# Secure GitHub Live Denied Write Conformance

The denied-write conformance check first reads a real GitHub issue through the
GitHub App installation, marks the session tainted, and then attempts a
protected private-repo write through the Secure GitHub adapter. Boundary must
deny before the GitHub mutation client is called.

## Command

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

## Required Result

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
github_mutation_called=false
raw_content_included=false
credential_data_included=false
```

`upstream_called=false` means the Secure GitHub adapter did not forward the
denied write to its upstream.

`github_mutation_called=false` means the instrumented GitHub mutation client was
not reached.

## What It Proves

- A live GitHub read can establish taint evidence.
- The protected write-after-taint policy denies before mutation forwarding.
- The conformance transcript records no raw issue body or credential data.

## What It Does Not Prove

- It does not mutate the repository by default.
- It does not prove that the deployment has removed all direct GitHub bypass
  paths.
- It does not make Secure GitHub production.

