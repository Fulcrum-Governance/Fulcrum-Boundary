# Secure GitHub Live Conformance

Secure GitHub live conformance is an opt-in preview harness for operator-owned
GitHub App credentials.

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

The denied-write check must report:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
github_mutation_called=false
```

This proves the routed live conformance path for the configured test
repository. It does not prove production GitHub security, full GitHub MCP
catalog coverage, or deployment bypass resistance.

Canonical repository docs:

- [Live conformance](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/secure-mcp/GITHUB_LIVE_CONFORMANCE.md)
- [Evidence handling](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/secure-mcp/GITHUB_LIVE_EVIDENCE.md)
- [Bypass model](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/secure-mcp/GITHUB_LIVE_BYPASS_MODEL.md)

