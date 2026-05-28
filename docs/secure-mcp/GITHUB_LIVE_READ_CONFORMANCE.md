# Secure GitHub Live Read Conformance

The read conformance check verifies that Boundary can use GitHub App
installation auth to fetch a real issue from the configured repository and turn
that external issue content into sanitized taint evidence.

## Command

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

## What It Proves

- GitHub App JWT generation and installation-token exchange completed.
- The configured issue was read through the live GitHub client.
- The issue body/title were hashed for evidence.
- The governed session recorded `github.issue_body` taint.
- No raw issue body, JWT, installation token, private key, or private key path
  appears in the transcript.

## What It Does Not Prove

- It does not prove GitHub write denial.
- It does not prove deployment bypass resistance.
- It does not prove full GitHub MCP catalog coverage.

Run the denied-write conformance check for the no-mutation proof.

