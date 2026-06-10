# Govern Your MCP Server

This is the canonical walkthrough for the #1 Boundary journey: **put Boundary
in front of an MCP server so an agent's tool calls are decided before they
reach the upstream**. It is the live-forwarding companion to the
[MCP adapter reference](adapters/MCP.md) (the production gateway path) and the
[MCP firewall install + descriptor lock](firewall/INSTALL_LOCK.md) reference
(the reversible config-rewrite path).

The MCP adapter is Boundary's **production** route. Everything below that is
labeled preview is called out inline; do not read a preview step as production
enforcement.

## Read this first: routed-only is the whole game

Boundary governs an MCP server **only when the client's traffic is forced
through Boundary**. If the agent can still reach the upstream MCP server
directly — a second config entry, a cached endpoint, a network path that skips
the gateway — that direct path is a **bypass** and is not governed. Installing a
route or starting the gateway does not, by itself, remove the direct path.
Closing the bypass is a deployment-topology responsibility (network policy,
service mesh, private networking, or equivalent), not something a CLI flag
proves. This single constraint is the source of nearly every caveat on this
page. See [`LIMITATIONS.md`](../LIMITATIONS.md).

The five steps below are: **discover** (read-only), **install a route** *or*
**run the gateway**, **trigger a denial and read the record**, and
**uninstall**. Run discovery first; it never writes to your MCP configs.

---

## 1. Discover — read-only

Nothing here touches your MCP client config or contacts a live MCP server. It is
local file inspection only.

```bash
boundary init --dry-run      # print the initialization plan, write nothing
boundary inventory --format markdown
```

`boundary init` (without `--dry-run`) creates a Boundary-owned workspace under
`.boundary/firewall/` and records a discovery summary; it reports
`mcp config mutation: none` because it never rewrites an MCP config. `boundary
inventory` renders the MCP configs, servers, and tools Boundary can route, so
you can decide which routed tools should pass through Boundary before you change
anything. (`docs/firewall/DISCOVERY_INVENTORY.md` covers the inventory formats.)

---

## 2. Install a route — reversible config rewrite (preview)

Use this when an MCP **client** (Claude Desktop, Cursor, VS Code, or a repo-local
config) holds the server entry. `boundary install` rewrites the selected entry
so the client launches `boundary mcp proxy` instead of talking to the upstream
directly, preserving a byte-for-byte backup and writing an install receipt.

Preview first, always:

```bash
boundary install --client claude --dry-run
```

Then install (pick the client that owns the config):

```bash
boundary install --client claude     # or: cursor | vscode | repo | custom
boundary install --client repo --root .
boundary install --config ./mcp.json --server github --out .boundary/firewall
```

This writes, under `.boundary/firewall/`:

- a **backup** of the original config bytes (treat as local secret material — the
  default `.boundary/` workspace is gitignored for this reason),
- an **install receipt** recording the config hash before/after, the backup
  path, the routed server names, and the route descriptor hash (receipts redact
  secret-like args, URL credentials, and env values), and
- a rewritten server entry that invokes `boundary mcp proxy`.

> **Preview caveat — the installed entrypoint is fail-closed, not live by
> itself.** `boundary mcp proxy` is a fail-closed entrypoint in this preview. It
> exists so an installed config routes *to* Boundary rather than silently
> bypassing it, but **live forwarding requires a configured Secure MCP profile
> or another governed runtime path** (see step 2b). Do not describe a bare
> install as live runtime protection. Full reference:
> [`docs/firewall/INSTALL_LOCK.md`](firewall/INSTALL_LOCK.md).

Optionally lock the tool surface so descriptor drift is detectable:

```bash
boundary lock --config ./mcp.json --out .boundary/firewall/locks/descriptor-lock.json
boundary verify-lock --lock .boundary/firewall/locks/descriptor-lock.json   # defaults to deny on drift
```

Descriptor lock detects changes to the tool surface Boundary can observe from
local config; it does not prove the upstream tool implementation is safe and
does not replace policy evaluation.

### 2b. Run the gateway — the production live-forwarding path

When you can put Boundary **in the network path** to the upstream MCP server,
run it as an HTTP MCP gateway. This is the production MCP route: Boundary accepts
JSON-RPC MCP requests, evaluates each action, returns a protocol-shaped denial
(JSON-RPC error `-32001`) before a blocked request reaches the upstream, and
forwards allowed requests to the configured upstream.

```bash
boundary serve \
  --listen :8080 \
  --policies ./policies \
  --upstream http://127.0.0.1:9000/mcp
```

An **HTTP/HTTPS** `--upstream` selects the production MCP proxy. (A Postgres DSN
`--upstream` selects the legacy Postgres demo path that powers `make demo`, not
the MCP proxy.) Point the client at the Boundary listen address instead of the
upstream, and enforce at the network layer that the agent cannot reach the
upstream directly — otherwise the direct path is an ungoverned bypass.

Full lifecycle and policy shape: [`docs/adapters/MCP.md`](adapters/MCP.md). The
governed runtime profile that an installed `boundary mcp proxy` route binds to
is the [Secure MCP profile](secure-mcp/README.md) (preview).

---

## 3. Trigger a denial and read the result

Add a deny rule for a tool and confirm the call is blocked before upstream. With
the gateway from step 2b, a static policy rule matches the MCP tool name:

```yaml
# ./policies/deny-danger.yaml
rules:
  - name: hide-danger
    tool: danger
    action: deny
    reason: blocked for this tenant
```

A denied `tools/call` returns a JSON-RPC error and is **not** forwarded
upstream. For `tools/list`, Boundary forwards the request and then removes tools
that would be denied for the current identity.

The success signal you are looking for on a denial is the adapter reporting that
the upstream was never called. For the fixture-only MCP proof lane you can run
this with no gateway, credentials, or network:

```bash
boundary demo github-lethal-trifecta --json --out demo.json
```

Expected denial signal:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

> **Honesty about `upstream_called` / `execution_claim`.** `upstream_called=false`
> (and the `schema_version "2"` record field `execution_claim`) is an **adapter
> self-report**, not a field of the hash-verified decision record and not
> independent attestation that no bytes moved. A `deny` record is evidence of the
> *decision*, not proof of *enforcement*, and it cannot see a direct-access
> bypass. See [`LIMITATIONS.md`](../LIMITATIONS.md).

### Read and verify the decision record

When a demo or evidence step writes a record, it prints a
`decision record path:` line. Verify that record's integrity locally:

```bash
boundary verify-record demo-artifacts/decision-record.json
```

`boundary verify-record` recomputes the record's stable SHA-256 hashes (request,
policy-bundle, and decision) and confirms the record is internally consistent
and unmodified since emission. These are **unkeyed integrity hashes, not
authenticity or proof**: a pass does not verify a signature, does not prove the
verdict was globally correct, and does not prove the action was enforced. To
render a record without verifying it, use `boundary explain`; to re-evaluate the
recorded request against the recorded policy bundle, use `boundary replay`.
References: [`docs/CLI_REFERENCE.md`](CLI_REFERENCE.md),
[`docs/RECEIPTS.md`](RECEIPTS.md),
[`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md).

---

## 4. Uninstall — reverse the route, keep the receipt

Installs are reversible. Restore the original config byte-for-byte from the
backup recorded in the install receipt:

```bash
boundary uninstall --receipt .boundary/firewall/install-receipts/<receipt>.json --dry-run
boundary uninstall --receipt .boundary/firewall/install-receipts/<receipt>.json
```

Uninstall verifies that the backup bytes match the pre-install hash in the
receipt **and** that the current config still matches the post-install hash,
then restores from that backup. If the config changed after install, uninstall
refuses to clobber those edits; use `--force` only after you have preserved or
intentionally discarded the post-install changes. Full reference:
[`docs/firewall/INSTALL_LOCK.md`](firewall/INSTALL_LOCK.md).

The gateway path (step 2b) has nothing to uninstall: stop the `boundary serve`
process and repoint the client back at the upstream.

---

## What this does and does not prove

- **Does**: decide allow/deny/warn/escalate/require-approval before a routed MCP
  tool call reaches the upstream; return a protocol-shaped denial without
  forwarding; emit a hash-verifiable decision record.
- **Does not**: govern any path that is not forced through Boundary; prove —
  from `upstream_called=false`, a `deny` record, or a `verify-record` pass —
  that no upstream bytes moved, that the verdict was globally correct, or that
  every deployment route is protected. Closing the direct-access bypass is a
  deployment-topology responsibility.

### Static (CGO_ENABLED=0) builds and SQL

If you route a Postgres-style `query` tool, note that static / no-cgo builds
(the `_static-nocgo` archives, the Homebrew formula, and the container image) do
not carry the AST SQL classifier: routed SQL classifies as `UNKNOWN` and the
Postgres guard denies it fail-closed. Use a `_cgo` release archive or a cgo
source build for class-based SQL policy. See [`docs/INSTALL.md`](INSTALL.md).

## See also

- [MCP adapter reference](adapters/MCP.md) — the production gateway lifecycle,
  policy shape, and bypass condition.
- [MCP firewall install + descriptor lock](firewall/INSTALL_LOCK.md) — the
  reversible config-rewrite path and descriptor drift detection.
- [Secure MCP profile](secure-mcp/README.md) — the governed runtime profile an
  installed route binds to (preview).
- [CLI reference](CLI_REFERENCE.md) — every command and its maturity label.
- [Limitations](../LIMITATIONS.md) — the routed-only constraint in full.
