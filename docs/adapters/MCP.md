# MCP Adapter

The MCP adapter is Boundary's flagship governed proxy path. It accepts HTTP
JSON-RPC MCP requests, evaluates each action through `governance.Pipeline`,
forwards allowed requests to an upstream MCP server, and returns protocol-shaped
denials before a blocked request reaches the upstream.

## Lifecycle

1. Parse JSON-RPC `tools/call` or `tools/list`.
2. Extract identity from governance headers or MCP-specific identity headers.
3. Evaluate the request through the Boundary pipeline.
4. Return JSON-RPC error `-32001` on deny without forwarding.
5. Forward allowed requests to the configured upstream MCP HTTP endpoint.
6. Inspect upstream JSON-RPC responses for malformed bodies and error objects.
7. Attach governance metadata under `result._meta.governance` when the response
   shape permits it, and always emit governance HTTP headers.
8. Emit structured decision records through the configured `AuditPublisher`.
9. Treat bypass proof as a deployment responsibility: the agent must be unable
   to reach the upstream MCP server directly.
10. Fail closed for MCP pipeline evaluation errors.

## CLI Usage

Use an HTTP or HTTPS `--upstream` to run `boundary serve` as a general MCP proxy:

```bash
boundary serve \
  --listen :8080 \
  --policies ./policies \
  --upstream http://127.0.0.1:9000/mcp
```

The legacy Postgres demo path is still available when `--upstream` is a Postgres
DSN. That path powers `make demo`; the production MCP proxy path is selected by
an HTTP(S) upstream URL.

## Policy Shape

Static policy rules match the MCP tool name. A denied `tools/call` returns a
JSON-RPC error and is not forwarded. For `tools/list`, Boundary forwards the
request and then removes tools that would be denied for the current identity.

```yaml
rules:
  - name: hide-danger
    tool: danger
    action: deny
    reason: blocked for this tenant
```

## Bypass Condition

Boundary only protects MCP tools when the agent's route to the upstream MCP
server passes through Boundary. Production deployments must enforce that with
network policy, service mesh policy, private networking, or equivalent controls.
