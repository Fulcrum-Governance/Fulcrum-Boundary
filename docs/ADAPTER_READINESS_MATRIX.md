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
| cli | preview | preview | implemented | implemented | delegated | delegated | delegated | implemented | implemented | delegated | stub | delegated | BND-CLI-001: owned shell execution wrapper and bypass proof. |
| codeexec | preview | production | implemented | implemented | delegated | delegated | delegated | implemented | implemented | delegated | stub | delegated | BND-CODE-001: sandbox lifecycle integration tests and bypass proof. |
| grpc | preview | production | implemented | implemented | implemented | implemented | delegated | stub | delegated | delegated | stub | delegated | BND-GRPC-001: metadata/trailer conformance, response policy, and bypass proof. |
| webhook | preview | preview | implemented | implemented | implemented | implemented | delegated | implemented | delegated | delegated | stub | delegated | BND-WEB-001: explicit mode split between informational and execution forwarding. |
| a2a | experimental | preview | implemented | implemented | delegated | stub | stub | stub | stub | delegated | stub | stub | BND-A2A-001: real A2A protocol control instead of no-op lifecycle methods. |

MCP is the first production adapter. Other adapters remain below production
until an adapter-specific spec proves their full lifecycle.
