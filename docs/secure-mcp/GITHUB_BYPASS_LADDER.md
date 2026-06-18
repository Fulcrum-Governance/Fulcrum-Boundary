# Secure GitHub Bypass-Proof Ladder

Secure GitHub governs a GitHub action only when the route is forced through the
Boundary profile. Direct access to the same repository — a personal token, an
SSH key, the upstream GitHub MCP server, an unmanaged `gh`/`git`, or any
ambient-credential path — is a bypass unless deployment topology removes it.
This page is the operator-facing form of the bypass-proof ladder; the
machine-readable level lives in the bypass-proof packet
(`boundary.secure_github.bypass_proof_packet.v1`).

This ladder records what evidence a deployment has earned. It **does not prove**
that no other path to the same repository exists; that is a property of your
topology, not of this ladder. Secure GitHub remains **preview** at every level.

## Levels

| Level | Meaning |
|---|---|
| L0 | Fixture/demo denies before upstream; no credentials; no live mutation. |
| L1 | Operator-owned live conformance with a controlled GitHub App; no-mutation proof for the denied write-after-taint path (`github_mutation_called=false`). |
| L2 | Managed deployment topology attests every direct path is denied: no direct GitHub API token, no upstream GitHub MCP, no SSH/git-write, no unmanaged `gh`, and egress policy enforced. |
| L3 | Third-party / enterprise deployment attestation plus network-policy evidence, reviewed outside Boundary. Boundary code never asserts L3. |

## What L2 is — and is not

L2 is the internal-only **production-candidate** gate. Reaching L2 does not make
Secure GitHub production and does not change its public maturity: it stays
preview until Boundary release truth changes. "Production-candidate" is an
internal planning word and must not appear in public copy. Calling Secure GitHub
production, or claiming live conformance proves deployment bypass resistance,
remains forbidden.

## How the level is computed

The L1 facts are derived from the routed live-evidence index — the sanitized
denied-write transcript proving the protected mutation was denied before the
GitHub mutation client was reached. The L2 facts are **operator-attested**
deployment-topology denials recorded in the bypass-proof packet; Boundary
records and classifies them but does not verify your deployment. The classifier
fails closed: any unattested denial caps the level at L1, and no live evidence
caps it at L0.

See [GITHUB_LIVE_BYPASS_MODEL.md](GITHUB_LIVE_BYPASS_MODEL.md) for the in-scope
vs bypass-path list, and
[../deployment/secure-github-bypass-proof-packet.md](../deployment/secure-github-bypass-proof-packet.md)
for the operator packet template.
