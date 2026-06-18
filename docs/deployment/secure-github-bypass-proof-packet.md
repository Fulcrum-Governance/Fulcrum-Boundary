# Secure GitHub Deployment Bypass-Proof Packet

This packet is **operator-authored** and **deployment-specific**. Boundary
provides the packet shape (`boundary.secure_github.bypass_proof_packet.v1`) and a
fail-closed classifier; the operator supplies the attestations. The packet
records and classifies what you attest. It **does not prove** that your
deployment is bypass-proof, and it does not upgrade Secure GitHub out of preview.

## What you bind together

1. **Live evidence (L1).** The sanitized live-evidence index from
   `boundary secure github conformance denied-write` — the denied write-after-taint
   transcript proving `upstream_called=false` and `github_mutation_called=false`,
   with a decision record hash. Never commit raw transcripts; commit only
   sanitized `.sanitized.json` hashes (see
   [../secure-mcp/GITHUB_LIVE_EVIDENCE.md](../secure-mcp/GITHUB_LIVE_EVIDENCE.md)).

2. **Deployment-topology attestation (L2).** For each direct path, attest it is
   denied and reference the evidence by a non-secret identifier (a manifest name,
   a NetworkPolicy id, a runbook link) — never a credential:

   | Denial | What you attest | Example evidence reference |
   |---|---|---|
   | `agent_has_no_direct_token` | The agent runtime holds no GitHub PAT/SSH key. | deploy manifest name |
   | `app_credential_runtime_only` | The GitHub App private key is sealed to the governed runtime only. | secret store policy id |
   | `upstream_mcp_unavailable` | The upstream GitHub MCP server is unreachable by the agent. | network namespace / egress rule id |
   | `no_unmanaged_git_or_gh` | No unmanaged `gh`/`git`/SSH write path exists in the agent image. | image SBOM / allowlist id |
   | `egress_policy_enforced` | Egress policy denies `api.github.com` except from the Boundary route. | NetworkPolicy name |

## Fail-closed rules

- Any denial left unattested caps the packet at **L1**.
- No recorded live evidence caps the packet at **L0** regardless of attestation.
- Evidence references that look secret-like (e.g. a bearer token, a private key)
  are rejected: reference the control, not the credential.

## What a passing L2 packet means

It means this deployment attested every direct path is denied and recorded the
routed no-mutation evidence. It is the internal production-candidate gate. It is
**not** a public production claim, and it does not prove bypass resistance for
paths outside what you attested. Third-party attestation (L3) is reviewed outside
Boundary.
