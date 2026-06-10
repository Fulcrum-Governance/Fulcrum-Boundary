# Govern Your MCP Server

Put Boundary in front of an MCP server so an agent's tool calls are decided
before they reach the upstream. Boundary governs that server **only when the
client's traffic is forced through Boundary** — a direct path to the same
upstream is a bypass and is not governed. Closing that bypass is a deployment
responsibility, not a CLI flag.

The MCP adapter is Boundary's production route. The reversible config-rewrite
install (`boundary install` / `boundary mcp proxy`) is preview, and the
installed entrypoint is fail-closed until it binds to a governed runtime path.

The five-step journey:

```bash
# 1. Discover (read-only — never writes your MCP config)
boundary init --dry-run
boundary inventory --format markdown

# 2a. Install a reversible route into an MCP client config (preview)
boundary install --client claude --dry-run
boundary install --client claude          # or: cursor | vscode | repo | custom

# 2b. Or run the production MCP gateway (HTTP/HTTPS upstream = live proxy)
boundary serve --listen :8080 --policies ./policies --upstream http://127.0.0.1:9000/mcp

# 3. Trigger a denial and read the record
boundary demo github-lethal-trifecta --json --out demo.json
boundary verify-record demo-artifacts/decision-record.json

# 4. Uninstall (reverse the route, verify against the receipt)
boundary uninstall --receipt .boundary/firewall/install-receipts/<receipt>.json
```

Expected denial signal (fixture MCP lane):

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

`upstream_called=false` is an adapter self-report, not a field of the
hash-verified decision record and not proof that no bytes moved. `boundary
verify-record` recomputes the record's stable SHA-256 hashes — integrity, not
authenticity, and not proof the verdict was correct or enforced.

Canonical walkthrough:
[docs/GOVERN_MCP_SERVER.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/GOVERN_MCP_SERVER.md)

Related references:

- [docs/adapters/MCP.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/adapters/MCP.md)
  — the production gateway lifecycle, policy shape, and bypass condition.
- [docs/firewall/INSTALL_LOCK.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/firewall/INSTALL_LOCK.md)
  — the reversible config-rewrite path and descriptor drift detection.
- [docs/LIMITATIONS.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/LIMITATIONS.md)
  — the routed-only constraint in full.
