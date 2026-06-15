# Per-Host Setup: Route an MCP Client Through Boundary

Short, per-host walkthroughs for putting an MCP client's tool calls on a
Boundary-governed route. Each host follows the same shape — **find the config →
install the route → check readiness** — and carries the same honest caveats:

> **Routed-only.** Boundary governs a tool call **only** when that call is forced
> through it. After install, the client's routed MCP servers go through Boundary;
> any path that still reaches a tool directly (a server you did not route, a shell
> or editor that calls the tool outside the rewritten config) is a bypass.
>
> **Fail-closed preview.** The generic installed route runs `boundary mcp proxy`,
> which is **fail-closed in this preview**: a real routed call does **not** forward
> or emit a decision record until you configure a
> [Secure MCP profile](../secure-mcp/README.md). Install proves the route is *in
> place and reversible*; `boundary doctor` checks local readiness; the full check
> that a real call is governed is the
> [Route Conformance Checklist](../ROUTE_CONFORMANCE_CHECKLIST.md).

Install is conservative: it rewrites a config **only** when you run `boundary
install` explicitly, and it writes a byte-for-byte backup first — restore it with
`boundary uninstall --receipt <path>`, using the receipt path the install command
prints. `boundary inventory`, `graph`, and policy generation are read-only. Full mechanics: [INSTALL_LOCK.md](./INSTALL_LOCK.md),
[DISCOVERY_INVENTORY.md](./DISCOVERY_INVENTORY.md).

Always preview first with `--dry-run` — it prints the exact rewrite for *your*
config without touching a file:

```bash
boundary install --client <claude|cursor|vscode|repo> --dry-run
```

---

## Claude Desktop

**Config location** (`--client claude`):

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |
| Windows | `%AppData%\Claude\claude_desktop_config.json` |

```bash
boundary install --client claude --dry-run   # preview the rewrite + backup path
boundary install --client claude             # write the route (backs up first)
```

Each routed `mcpServers` entry is rewritten so the server launches **through** a
Boundary-owned route instead of directly; your original file is backed up
byte-for-byte under `.boundary/firewall/backups/`. Restart Claude Desktop so it
re-reads the config. **Check readiness:** `boundary doctor` (or `--json`) lists
the local routed surface and the bypass caveats — it does not by itself prove
this server is live-governed (see the fail-closed note above). Servers you did
not route, or tools reached outside this config, stay a bypass.

---

## Claude Code

Claude Code reads project MCP servers from a repo-local **`.mcp.json`**. Boundary
has no dedicated Claude Code selector — route it as the **repo-local** client:

```bash
boundary install --config ./.mcp.json --dry-run   # preview first (no mutation)
boundary install --client repo --root .            # routes .mcp.json / mcp.json in the repo
```

| Scope | Path |
|---|---|
| Project | `.mcp.json` (or `mcp.json`) at the repo root |

The routed entry wraps the server command so calls pass through Boundary first.
**Check readiness:** run `boundary doctor` from the repo. Servers Claude Code
loads from your *user* config (outside the repo) are not covered by a repo-local
install — that path is a bypass until you route those servers explicitly with
`--config`.

---

## Cursor

**Config location** (`--client cursor`):

| Scope / OS | Path |
|---|---|
| Project | `.cursor/mcp.json` |
| User (macOS) | `~/Library/Application Support/Cursor/User/mcp.json` or `~/.cursor/mcp.json` |
| User (Linux) | `~/.config/Cursor/User/mcp.json` or `~/.cursor/mcp.json` |

```bash
boundary install --client cursor --dry-run
boundary install --client cursor
```

Reload Cursor after install. **Check readiness:** `boundary doctor` shows the
local routed surface and bypass caveats. Cursor servers you did not route, or
tools reached outside the rewritten config, are a bypass.

---

## VS Code

**Config location** (`--client vscode`):

| Scope / OS | Path |
|---|---|
| Project | `.vscode/mcp.json` |
| User (macOS) | `~/Library/Application Support/Code/User/mcp.json` |
| User (Linux) | `~/.config/Code/User/mcp.json` |
| User (Windows) | `%AppData%\Code\User\mcp.json` |

```bash
boundary install --client vscode --dry-run
boundary install --client vscode
```

Reload the VS Code window after install. **Check readiness:** `boundary doctor`
lists the local routed surface and bypass caveats. Anything VS Code reaches
outside the routed config — an un-routed server, or a tool called directly — is a
bypass.

---

## Confirm the route actually passes through Boundary

`boundary doctor` reports the local routed surface and the known bypass paths; it
does not call the network and does not prove remote deployment safety. The
generic installed route is **fail-closed** until you configure a
[Secure MCP profile](../secure-mcp/README.md) — it holds the route open in your
config but does not forward live calls or emit decision records on its own. For
the full check — that a real call is intercepted, denied where it should be, and
that no un-routed path reaches the tool — work through the
[Route Conformance Checklist](../ROUTE_CONFORMANCE_CHECKLIST.md). Until a route
passes that checklist, treat the host as only partially covered.
