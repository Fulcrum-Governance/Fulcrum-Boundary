# MCP Firewall

The MCP Firewall surface helps operators see and route MCP tool paths before
they become runtime surprises.

Core commands:

```bash
boundary inventory --format markdown
boundary graph --format mermaid
boundary policy generate --out boundary-firewall-policies
boundary dashboard --format html --out .boundary/firewall/dashboard.html
```

The dashboard is local-only artifact visibility. It is not hosted monitoring
and does not protect MCP servers by itself.
