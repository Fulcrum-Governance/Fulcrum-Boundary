# Risk Graph

Risk graphs connect discovered sources, MCP servers, tools, and privileged
sinks into reviewable paths.

```bash
boundary graph --format mermaid
```

```mermaid
flowchart LR
  A[Untrusted issue text] --> B[Agent context]
  B --> C[GitHub MCP server]
  C --> D[Private repo write tool]
```

Risk graphs are visibility and policy-starting artifacts. They do not govern a
tool until traffic routes through Boundary.
