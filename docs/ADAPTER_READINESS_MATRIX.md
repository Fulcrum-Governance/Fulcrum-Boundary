# Adapter Readiness Matrix

Boundary adapters are not all at the same maturity level. This matrix records
the current lifecycle truth for each adapter so public docs can talk about
transport coverage without implying production parity.

The machine-readable declarations live next to each adapter as
`adapters/<adapter>/readiness.yaml`. The reusable gate in
[`tests/adapter_conformance`](../tests/adapter_conformance) fails when an
adapter is missing a declaration, omits one of the ten lifecycle steps, or is
listed as production without conformance evidence.

## Lifecycle Steps

| Step | Meaning |
|---|---|
| `parse` | Convert transport payload into `GovernanceRequest`. |
| `identify` | Populate agent, tenant, and trace identity from transport context. |
| `evaluate` | Pass the request through `governance.Pipeline`. |
| `deny` | Return a transport-shaped denial without forwarding to the tool. |
| `forward` | Send allowed requests to the tool through the governed path only. |
| `inspect` | Examine tool responses where the protocol allows it. |
| `metadata` | Attach governance verdict metadata to the response. |
| `record` | Emit a structured decision record. |
| `bypass_proof` | Demonstrate the deployment has no direct tool path around Boundary. |
| `fail_closed` | Deny rather than pass through on governance errors. |

Step states are `implemented`, `delegated`, `not_applicable`, or `stub`.

## Maturity Taxonomy

| Level | Name | Requirement |
|---|---|---|
| `experimental` | Concept exists | `parse` is implemented. Other lifecycle steps may be stubbed. |
| `preview` | Core lifecycle works | Parse, evaluate, deny, and record are implemented or explicitly delegated. Forwarding may be host-delegated. |
| `production` | Full lifecycle proven | All ten steps are implemented or formally delegated, with integration tests, bypass proof, and fail-mode tests. |

## Readiness Matrix

| Adapter | Status | Target | parse | identify | evaluate | deny | forward | inspect | metadata | record | bypass_proof | fail_closed | Key gap |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| mcp | production | production | implemented | implemented | implemented | implemented | implemented | implemented | implemented | delegated | delegated | implemented | Production proxy path ships with lifecycle tests; bypass proof remains a deployment topology contract. |
| cli | preview | preview | implemented | implemented | delegated | implemented | implemented | implemented | implemented | delegated | delegated | implemented | BND-CLI-002: production requires deployment evidence that the Boundary wrapper is the sole command path. |
| codeexec | preview | preview | implemented | implemented | delegated | implemented | implemented | implemented | implemented | delegated | delegated | implemented | BND-CODE-001: production requires a real named sandbox boundary with integration tests and bypass proof. |
| grpc | preview | preview | implemented | implemented | implemented | implemented | delegated | implemented | implemented | delegated | delegated | implemented | BND-GRPC-001: production requires deployment bypass evidence; streaming workloads require per-message governance lifecycle tests. |
| managedagents | preview | production | implemented | implemented | implemented | implemented | implemented | implemented | implemented | delegated | delegated | implemented | BND-MAPROD-001: live upstream Managed Agents conformance run with operator-owned credentials. |
| webhook | preview | preview | implemented | implemented | implemented | implemented | delegated | implemented | delegated | delegated | stub | delegated | BND-WEB-001: explicit mode split between informational and execution forwarding. |
| a2a | preview | preview | implemented | implemented | delegated | implemented | implemented | implemented | implemented | delegated | delegated | implemented | BND-A2A-002: live protocol conformance and deployment bypass evidence before production. |

MCP is the first production adapter. Managed Agents now has a preview proxy
path. A2A now has a preview governed lifecycle against a documented protocol
snapshot. CodeExec now has a preview governed lifecycle, but remains below
production until a real named sandbox boundary is tested and documented. gRPC
now has a preview unary lifecycle with governance trailers and response
inspection; streaming workloads remain below production until per-message
governance is implemented and tested. Other adapters remain below production
until an adapter-specific spec proves their full lifecycle.
