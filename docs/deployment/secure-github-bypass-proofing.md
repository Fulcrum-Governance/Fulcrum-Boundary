# Secure GitHub Bypass Proofing

Secure GitHub only governs GitHub actions that route through the Boundary
profile. Direct access to GitHub's API, a direct upstream GitHub MCP server, or
another credentialed wrapper is outside this profile unless deployment topology
removes that path.

## Preview Fixture Boundary

The current profile proves the local decision path:

- GitHub MCP tool call enters `boundary secure github`.
- The adapter classifies the tool into `R0`, `W0`, `W1`, or `W2`.
- External GitHub content marks the session tainted.
- A later `W1` or `W2` private-repo mutation is denied before upstream.
- Fixture upstream records that the denied write was not called.

No live GitHub credential is used in the fixture proof.

## Live Conformance Boundary

Secure GitHub also has an opt-in live conformance harness. It can:

- use operator-owned GitHub App credentials;
- read a configured live GitHub issue;
- record sanitized taint evidence;
- deny a protected write-after-taint action;
- prove `upstream_called=false` and `github_mutation_called=false` for that
  denied write.

This proves the governed route for the configured test repository. It does not
prove that the deployment has removed all bypass paths.

## Production Bypass Controls

A production Secure GitHub deployment would need documented evidence that:

- the agent has no direct GitHub token or SSH key;
- the GitHub App credential is held only by the governed profile runtime;
- direct access to the upstream GitHub MCP server is unavailable to the agent;
- egress or network policy prevents bypassing Boundary for GitHub API writes;
- the one-repo-per-session policy is backed by durable session state when
  required by the deployment;
- live conformance transcripts are sanitized before storage.

## Evidence Field Operator-Trust Boundary

The credential scrubber rejects raw secrets, bearer tokens, PAT/SSH key
material, and raw private-key bodies in attestation Evidence fields. It does
NOT detect non-credential sensitive content such as PR bodies, internal
repository paths, or owner/repo names pasted into an Evidence string. Operators
are responsible for supplying short deployment-control references (e.g. "token
sealed to runtime vault; direct GitHub path denied by egress policy") rather
than raw repository or PR content. This is a preview-stage operator-trust
boundary: Boundary records what the operator attests, not what it has
independently verified.

## Release Gate

Do not mark Secure GitHub production until deployment bypass evidence is
recorded for the protected environment. Until then, public language must say
preview and distinguish fixture proof from opt-in live conformance proof.
