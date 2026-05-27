# Managed Agents Governed Session Example

This example is credential-free. It models the session events Boundary expects
from a Managed Agents stream and shows how the adapter auto-resolves tool
confirmation pauses by policy.

```go
pipeline := governance.NewPipeline(governance.PipelineConfig{
    StaticPolicies: []governance.StaticPolicyRule{
        {Name: "block-prod-delete", Tool: "delete_production_issue", Action: "deny"},
    },
}, nil, nil, auditor)

forwarder := &managedagents.MemoryForwarder{}
tracker := managedagents.NewThreadTracker("sess-1", 1.00)
adapter := managedagents.NewProxyAdapter("tenant-a", forwarder)
resolver := &managedagents.ToolResolver{
    Adapter: adapter,
    Pipeline: pipeline,
    Tracker: tracker,
    Forwarder: forwarder,
}
proxy := managedagents.NewSessionProxy(resolver, tracker)
```

Event sequence:

1. `agent.tool_use` for `read_issue` receives an allow confirmation.
2. `agent.mcp_tool_use` for `delete_production_issue` receives a deny
   confirmation with a policy reason.
3. `session.thread_created` registers a child thread with a budget allocation.
4. Further tool events reserve from the child and root budgets before
   confirmation is sent.

Run the integration tests for the executable example:

```bash
env -u GOROOT go test ./tests/integration -run ManagedAgents
```
