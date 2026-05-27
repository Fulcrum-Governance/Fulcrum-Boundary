# Claims

Boundary keeps public claims synchronized across the claims ledger, readiness
matrix, release truth reports, README, and tests.

Current claim posture:

- MCP is production.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, and Secure GitHub are
  preview adapter/profile surfaces.
- Boundary is not a SQL firewall.
- Generated policies are starter policies for operator review.
- Secure GitHub remains preview until live conformance and deployment bypass
  evidence are recorded.

Authoritative files in the repository:

- `claims/boundary_claims.yaml`
- `docs/CLAIMS_LEDGER.md`
- `docs/ADAPTER_READINESS_MATRIX.md`
- `docs/RELEASE_TRUTH_PUBLIC.md`
