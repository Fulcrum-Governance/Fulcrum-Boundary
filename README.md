# Fulcrum Boundary

> Pre-execution control for agent actions across transports. Fulcrum Boundary sits between agent intent and privileged tools, decides before execution, and emits an inspectable decision record.

[![Go Reference](https://pkg.go.dev/badge/github.com/fulcrum-governance/fulcrum-boundary.svg)](https://pkg.go.dev/github.com/fulcrum-governance/fulcrum-boundary)
[![CI](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fulcrum-governance/fulcrum-boundary)](https://goreportcard.com/report/github.com/fulcrum-governance/fulcrum-boundary)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)

## What is Fulcrum Boundary?

Fulcrum Boundary is the out-of-process action boundary for production AI agents. As a Go library and gateway binary, it evaluates tool calls against trust state, static policies, domain interceptors, and a portable policy engine before those calls reach the underlying tool.

The first packaged release is the **MCP Safety Gateway**: route a Postgres tool call through Boundary, allow a safe `SELECT`, block a destructive `DROP TABLE`, demonstrate that the demo agent cannot bypass the gateway network path, and inspect the structured decision record.

Boundary includes a production MCP adapter plus CLI, CodeExec, gRPC, Managed Agents, Webhook, and A2A adapter packages with maturity tracked per adapter. Direct tool calls are governed only when routed through Boundary and when the deployment topology prevents the agent from reaching the privileged tool directly.

## MCP Safety Gateway Quick Start

Run the launch demo from a clean clone:

```bash
make demo
```

The demo starts three containers:

- `demo-agent`: frontend network only
- `gateway`: frontend + backend networks
- `postgres`: backend-only internal network

Expected spine:

```text
1. Safe SELECT through Boundary
ALLOW status=200 ...

2. Destructive DROP TABLE through Boundary
DENY status=403 ... "matched_rule":"block-drop-table"

3. Direct bypass attempt to Postgres
BYPASS BLOCKED ...
```

For a local binary:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest
boundary --help
boundary verify --policies examples/mcp-postgres-gateway/policies
```

## Library Quick Start

```go
package main

import (
	"context"
	"fmt"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func main() {
	cfg := governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{
			{Name: "block-rm", Tool: "rm", Action: "deny", Reason: "destructive"},
		},
	}
	pipeline := governance.NewPipeline(cfg, nil, nil, nil)

	req := &governance.GovernanceRequest{
		ToolName:  "rm",
		Transport: governance.TransportCLI,
		TenantID:  "tenant-1",
	}
	decision, err := pipeline.Evaluate(context.Background(), req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s — %s\n", decision.Action, decision.Reason)
}
```

```
$ go run main.go
deny — destructive
```

## Architecture

```
Agent Request
     │
     ▼
┌─────────────────┐   Stage 1:  Trust / circuit-breaker check (optional)
│ TrustChecker    │            Isolated or Terminated → deny
└────────┬────────┘
         │
         ▼
┌─────────────────┐   Stage 2:  Static allow/deny rules on tool name
│ Static Policies │            Fastest path; no I/O
└────────┬────────┘
         │
         ▼
┌─────────────────┐   Stage 3:  Domain-specific interceptors by tool name
│  Interceptors   │            (e.g. SQL guard, filesystem whitelist)
└────────┬────────┘
         │
         ▼
┌─────────────────┐   Stage 4:  Portable PolicyEval engine
│   PolicyEval    │            Declarative rules with conditions
└────────┬────────┘
         │
         ▼
  GovernanceDecision  (allow | deny | warn | escalate | require_approval)
         │
         ▼
  AuditPublisher     Emitted on every evaluation, allow or deny
```

Every stage returns early on a terminal decision. Audit events fire regardless
of outcome.

## Transport Adapters

Boundary tracks adapter maturity explicitly. See
[`docs/ADAPTER_READINESS_MATRIX.md`](./docs/ADAPTER_READINESS_MATRIX.md) and the
per-adapter `readiness.yaml` files for the ten-step lifecycle behind each row.

### Production

| Adapter | Package | Handles |
|---|---|---|
| MCP | `adapters/mcp` | HTTP JSON-RPC MCP proxying for `tools/call` and `tools/list`; allowed requests forward to an upstream MCP server, denied requests never reach upstream, and responses carry governance metadata |

### Preview

| Adapter | Package | Handles |
|---|---|---|
| CLI | `adapters/cli` | Shell commands including pipe chains, with a risk classifier; execution control is delegated to the host command wrapper |
| Code exec | `adapters/codeexec` | Python and JavaScript source submitted to a sandbox, with obfuscation analysis; execution is delegated to the sandbox runtime |
| gRPC | `adapters/grpc` | gRPC unary calls via a server interceptor in a separate module |
| Managed Agents | `adapters/managedagents` | Managed Agents session streams in preview proxy mode, with policy-driven tool confirmations, thread budget tracking, and a documented credential-bound bypass model; production status requires a live upstream conformance run |
| Webhook | `adapters/webhook` | HTTP webhook tool-call payloads, with handler-owned allow/deny response shaping |
| A2A | `adapters/a2a` | Agent-to-agent task/message envelopes in preview mode, with a documented protocol snapshot, governed forwarding, denial shaping, response inspection, governance metadata, and fail-closed handling for malformed or unsupported mandatory fields |

Each adapter implements the `governance.TransportAdapter` interface. Adding a
new transport is a matter of satisfying that interface and declaring lifecycle
readiness — see
[`docs/ADAPTER_CONTRACT.md`](./docs/ADAPTER_CONTRACT.md) and
[ARCHITECTURE.md](./ARCHITECTURE.md#adding-a-new-transport-adapter).

The gRPC adapter lives in its own Go module under `adapters/grpc/` so that
`google.golang.org/grpc` does not propagate into the root dependency tree.
The other adapters use only stdlib and sibling packages.

## HTTP Middleware

Boundary ships an HTTP middleware for reverse-proxy deployments. Wrap any
downstream handler and every request is evaluated through the pipeline
before it is forwarded.

```go
middleware := governance.NewMiddleware(pipeline, downstream, governance.MiddlewareConfig{})
http.ListenAndServe(":8080", middleware)
```

Denied requests return HTTP 403 with a JSON body containing `action`, `reason`,
`decision_mode`, `matched_rule`, `policy_file`, `gateway_version`, and
`request_id`. Every response — allow or deny — carries `X-Governance-Action`,
`X-Governance-Reason`, `X-Governance-Matched-Rule`, and
`X-Governance-Envelope-ID` headers so clients can read the verdict without parsing the body. See
[`examples/http-middleware`](./examples/http-middleware).

By default, the middleware reads identity from `X-Governance-Agent-ID` and
`X-Governance-Tenant-ID`. For compatibility it also accepts legacy
`X-Agent-ID` / `X-Tenant-ID` inputs and normalizes forwarded requests so the
downstream handler sees the governance-prefixed headers.

## Logging

Boundary ships a `SlogAuditPublisher` that writes every governance decision as a
structured record. Allow and warn decisions log at `INFO`; deny, escalate,
and require-approval log at `WARN`.

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
auditor := governance.NewSlogAuditPublisher(logger)
pipeline := governance.NewPipeline(cfg, nil, nil, auditor)
```

All standard fields are attached as `slog.Attr` values (`request_id`,
`transport`, `tool_name`, `action`, `reason`, `decision_mode`, `matched_rule`,
`policy_file`, `gateway_version`, `trace_id`, `agent_id`, `tenant_id`,
`trust_score`, `envelope_id`, `timestamp`) so they index cleanly in any
structured sink. See [docs/DECISION_RECORDS.md](./docs/DECISION_RECORDS.md).

## Dry-Run Mode

Roll out governance in audit-only mode before enforcing. With `DryRun: true`,
the pipeline evaluates every stage normally but converts any terminal deny
into an allow. The decision carries `DryRun: true` and a reason prefixed
with `DRY-RUN would deny:` so the audit trail still reflects what would
have been blocked.

```go
cfg := governance.PipelineConfig{
    DryRun:         true,
    StaticPolicies: rules,
}
pipeline := governance.NewPipeline(cfg, nil, nil, auditor)
```

The HTTP middleware also emits an `X-Governance-Dry-Run: true` header on
any response that was converted from deny to allow.

## Rate Limiting

The `interceptors` package ships a token-bucket rate limiter with three
keying strategies (by agent, by tool, or by the `agent:tool` combination).

```go
rl := interceptors.NewRateLimiter(interceptors.RateLimitConfig{
    MaxRequests: 100,
    Window:      time.Minute,
})
pipeline.RegisterInterceptor("search", rl.ForAgent())
```

The limiter has zero external dependencies and is safe for concurrent use.
See [`examples/rate-limit`](./examples/rate-limit).

## Static Policy Glob Patterns

Static policy rules match the `Tool` field against `GovernanceRequest.ToolName`
using `path.Match` semantics. Exact names match, `*` and the empty string
match everything, and the `*` / `?` / `[abc]` glob operators are supported.
Malformed patterns are treated as non-matching rather than crashing the
pipeline.

```go
{Name: "deny-all-db-writes", Tool: "database_*", Action: "deny", Reason: "writes routed through approval"}
```

## Policy Schema And SQL Guard

`boundary verify --policies ./policies` validates both legacy v0.2.0 static
YAML and schema v1 policy files. Schema v1 adds an explicit
`schema_version: "1"` envelope, condition validation, tenant and agent scopes,
and richer request projection into PolicyEval. See
[`docs/POLICY_SCHEMA.md`](./docs/POLICY_SCHEMA.md).

The Postgres interceptor classifies SQL with the PostgreSQL parser AST and
annotates requests with `sql_class` before PolicyEval. Unknown or unparsable
SQL fails closed; destructive SQL is denied; administrative SQL escalates. See
[`docs/policies/POSTGRES.md`](./docs/policies/POSTGRES.md).

## Examples

| Directory | What it shows |
|---|---|
| [`examples/simple`](./examples/simple) | Minimal pipeline with two static rules |
| [`examples/mcp-proxy`](./examples/mcp-proxy) | MCP adapter parsing a JSON-RPC payload |
| [`examples/mcp-postgres-gateway`](./examples/mcp-postgres-gateway) | Dockerized MCP Safety Gateway demo with Postgres network isolation |
| [`examples/custom-interceptor`](./examples/custom-interceptor) | Domain interceptor composed with a static policy |
| [`examples/redis-trust`](./examples/redis-trust) | Redis-backed `TrustChecker` implementation |
| [`examples/http-middleware`](./examples/http-middleware) | HTTP reverse-proxy middleware with structured audit logging |
| [`examples/rate-limit`](./examples/rate-limit) | Token-bucket rate limiter wired as an interceptor |

Each example is a standalone Go module with its own `go.mod`. Run any of them
with `go run main.go` from its directory.

## Why Out-of-Process?

Boundary is narrower on purpose: it is the part of the Fulcrum stack that must
run outside the agent to be trustworthy. When the agent's route to a dangerous
tool passes through Boundary, the decision happens before mutation, outside the
agent process, and leaves behind a structured record of the verdict.

The router is a deployment pattern. The boundary is the product.

## Interfaces

The governance package exports the core interfaces that define Boundary's
extension points:

- **`TrustChecker`** — returns the current trust state for an agent. Implement
  this to wire Boundary to your circuit-breaker or reputation system. `nil` is
  accepted; Stage 1 is skipped.
- **`TransportAdapter`** — the contract each transport satisfies. `ParseRequest`
  converts a protocol-specific payload into a `GovernanceRequest`,
  `ForwardGoverned` relays an allowed request, `InspectResponse` examines
  tool output, `EmitGovernanceMetadata` attaches headers to the response.
  Per-method requirements, no-op semantics, and integration patterns for
  cross-repo consumers (fulcrum-io MCP/CLI/code-exec proxies, fulcrum-trust
  LangGraph adapter) are documented in
  [docs/ADAPTER_CONTRACT.md](./docs/ADAPTER_CONTRACT.md).
- **`Interceptor`** — `func(ctx, *GovernanceRequest) (*InterceptorResult, error)`.
  Register one per tool name via `Pipeline.RegisterInterceptor`. Return `nil`
  to decline and continue the pipeline.
- **`AuditPublisher`** — `Publish(ctx, AuditEvent)`. Boundary calls this after every
  evaluation. The default is a no-op; a production deployment typically wires
  this to NATS, Kafka, or a log sink.

Full signatures live in [`governance/`](./governance/).
Standalone and kernel integration seams are documented in
[docs/INTEGRATION.md](./docs/INTEGRATION.md) and
[docs/STANDALONE_VS_KERNEL.md](./docs/STANDALONE_VS_KERNEL.md).

## Part of the Fulcrum Architecture

Fulcrum is built as four coordinated repositories. This repo provides the
out-of-process enforcement boundary; the core runtime owns multi-tenant
orchestration and operator surfaces; the trust engine tracks agent-pair
reputation; and the formal core publishes machine-checkable proof artifacts.

| Repo | Role | License |
|------|------|---------|
| [`fulcrum-io`](https://github.com/Fulcrum-Governance/fulcrum-io) | Runtime control plane: policy engine, envelope, Foundry, MCP proxy, dashboard | BSL 1.1 |
| **`Boundary`** (this repo) | Out-of-process action boundary: transport adapters, 4-stage pipeline, MCP Safety Gateway | Apache 2.0 |
| [`fulcrum-trust`](https://github.com/Fulcrum-Governance/fulcrum-trust) | Trust engine: Beta(α,β) evaluator, circuit breaker, LangGraph adapter | Apache 2.0 |
| [`Fulcrum-Proofs`](https://github.com/Fulcrum-Governance/Fulcrum-Proofs) | Formal core: Lean 4 proofs, claim ledger, theorem inventory | MIT |

Project docs: [Contributing](./CONTRIBUTING.md) · [Security](./SECURITY.md) · [Changelog](./CHANGELOG.md) · [Code of Conduct](./CODE_OF_CONDUCT.md) · [Citation](./CITATION.cff)

Boundary is the open-source enforcement layer. The full kernel pairs it with upstream Lean 4 proofs of bounded policy invariants in `Fulcrum-Proofs`; Boundary consumes those proof-backed contracts through documented correspondence and decision-mode boundaries rather than emitting `proved` decisions itself. See [docs/PROOF_BOUNDARY.md](./docs/PROOF_BOUNDARY.md) for the correspondence map. The full kernel also adds Bayesian trust scoring with Beta distributions, per-tenant cost modelling, multi-agent workflow orchestration, and managed multi-tenant infrastructure.

- Website: [fulcrumlayer.io](https://fulcrumlayer.io)
- Companion paper: tracked separately from this repository; cite Boundary as software until a public paper citation is issued

## License

Apache 2.0 — see [LICENSE](./LICENSE).

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). For security issues, see
[SECURITY.md](./SECURITY.md).
