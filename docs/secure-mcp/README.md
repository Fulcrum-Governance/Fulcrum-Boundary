# Secure MCP

Secure MCP is the Boundary pattern for governed MCP server profiles. A profile
classifies MCP tools, evaluates policy before forwarding, emits decision
records, and documents the bypass paths that remain outside Boundary.

This directory is the home for Secure MCP profile docs. The contract docs live
one level up:

- [`docs/SECURE_MCP_CONTRACT.md`](../SECURE_MCP_CONTRACT.md)
- [`docs/SECURE_MCP_SERVER_TEMPLATE.md`](../SECURE_MCP_SERVER_TEMPLATE.md)
- [`docs/SECURE_MCP_TOOL_TAXONOMY.md`](../SECURE_MCP_TOOL_TAXONOMY.md)

## Current Status

Secure MCP is a pattern and release-train contract. It does not imply every
profile exists or is production-ready.

The first flagship profile is Secure GitHub MCP. Its preview proof is
fixture-based and implemented in `adapters/securegithub`:

1. Untrusted GitHub content enters a governed envelope.
2. The agent attempts a protected private-repo mutation.
3. Boundary denies before GitHub is touched.
4. A decision record captures the verdict, tool, resource, taint source, and
   reason.

Secure GitHub MCP now includes an opt-in live conformance preview harness, but
remains preview until deployment bypass proof exists.

Profile docs:

- [`GITHUB.md`](./GITHUB.md)
- [`GITHUB_REDTEAM.md`](./GITHUB_REDTEAM.md)
- [`GITHUB_APP_AUTH.md`](./GITHUB_APP_AUTH.md)
- [`GITHUB_APP_PERMISSIONS.md`](./GITHUB_APP_PERMISSIONS.md)
- [`GITHUB_LIVE_CONFORMANCE.md`](./GITHUB_LIVE_CONFORMANCE.md)
- [`GITHUB_LIVE_EVIDENCE.md`](./GITHUB_LIVE_EVIDENCE.md)
- [`GITHUB_LIVE_BYPASS_MODEL.md`](./GITHUB_LIVE_BYPASS_MODEL.md)
- [`docs/deployment/secure-github-bypass-proofing.md`](../deployment/secure-github-bypass-proofing.md)

## Profile Checklist

Each profile should add:

- profile overview
- supported tool set
- tool taxonomy
- descriptor-lock behavior
- taint model
- policy projection
- denial shape
- decision-record fields
- bypass model
- tests and evidence
- claims and readiness updates

Unsupported tools must fail closed or return a clear unsupported error.
