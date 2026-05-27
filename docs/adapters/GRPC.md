# gRPC Adapter

Status: preview.

The gRPC adapter governs unary RPCs through a server interceptor in the separate `adapters/grpc` module. The interceptor parses the full method and incoming metadata into a `GovernanceRequest`, evaluates the shared `governance.Pipeline`, denies before the handler on policy failure, and emits governance verdict metadata in gRPC response trailers.

## Trailer Metadata

Unary responses and denials use these trailer keys:

| Trailer | Meaning |
|---|---|
| `governance-action` | Governance verdict action. |
| `governance-rule` | Matched rule when available. |
| `governance-mode` | Decision mode when available. |
| `governance-trust` | Trust score attached to the decision. |
| `governance-request-id` | Boundary request ID. |
| `governance-envelope-id` | Boundary envelope ID. |
| `governance-response-safe` | Best-effort unary response inspection result. |
| `governance-response-concerns` | Policy-relevant response markers when detected. |

Trailer emission is best effort when the interceptor runs inside a real gRPC server context. Non-server embeddings can use `EmitGovernanceMetadata` to attach the same keys to a `governance.ToolResponse`.

## Unary Lifecycle

1. Parse the full method and incoming metadata.
2. Identify agent, tenant, and trace from metadata when present.
3. Evaluate the shared governance pipeline.
4. Return `PermissionDenied` before the handler on deny.
5. Forward allowed unary requests by invoking the downstream handler.
6. Inspect string or byte-like unary responses for policy-relevant markers.
7. Attach governance and inspection metadata as trailers.
8. Record decisions through the shared pipeline auditor.
9. Treat the gRPC interceptor as the governed path; direct service endpoints bypass Boundary.
10. Fail closed on missing pipeline, parse errors, and fail-closed policy evaluation outcomes.

## Streaming Limitation

The adapter governs the initial unary request path only. Streaming RPC messages are not individually governed by this adapter. A deployment can claim production readiness for unary RPCs only if the interceptor is the sole path to the protected service and the deployment provides bypass evidence. Streaming workloads remain preview unless every message is individually routed through a governed interceptor with lifecycle tests.

## Bypass Model

Governance applies when clients reach the service through the Boundary gRPC interceptor. Direct access to the underlying gRPC service, a second unguarded listener, sidecar escape path, or service mesh route that skips the interceptor bypasses Boundary.
