# MCP Firewall Risk Graph And Policy Generation

Boundary can turn local MCP inventory into a risk graph and a set of starter
policy templates. This surface is intentionally pre-install and operator-led:
it identifies risk paths and writes starter policies, but it does not route,
protect, lock, or rewrite MCP client configs by itself.

## Commands

Render inventory-derived risk paths:

```bash
boundary graph --format json
boundary graph --format mermaid --out boundary-risk-graph.mmd
```

Generate balanced starter policies:

```bash
boundary policy generate --mode balanced --out boundary-firewall-policies
boundary verify --policies boundary-firewall-policies
```

`balanced` is the only generated mode in this release. It is conservative around
W1/W2 tools and still expects operator review before deployment.

## Risk Graph

`boundary graph` reuses the same read-only discovery inputs as
`boundary inventory`: `--root`, `--home`, `--config`, `--include-defaults`, and
`--out`. The graph output is derived from the inventory model, not from live MCP
traffic.

JSON output includes:

- inventory provenance and discovery warnings
- deterministic nodes and edges
- source, sink, server, client, config path, tool, risk class, reason, and
  mitigation for each path
- summary counts for high-risk, descriptor-change, repo-write, and external
  sink paths

Mermaid output renders the same model as a `flowchart LR` graph.

Risk categories currently include:

| Category | Meaning |
|---|---|
| `untrusted_input_to_private_data` | Untrusted repository or collaborator content can enter agent context before private tools run. |
| `untrusted_input_to_private_repo_mutation` | A GitHub MCP server exposes both untrusted read paths and private-repo mutation paths. |
| `external_sink` | Agent-controlled output can be published outside the workspace. |
| `privileged_mutation` | W2 tools can perform critical mutations. |
| `descriptor_change` | MCP descriptors affect policy projection and should be locked by the later install/lock step. |
| `destructive_db_action` | Database query tools need statement-class constraints. |
| `filesystem_exfil` | Local file reads can move private local data into agent context. |
| `repo_write_path` | Repository write tools can mutate private source state. |
| `review_required` | Boundary could not classify the server or tool from available config data. |

## Starter Policies

`boundary policy generate` writes schema v1 YAML templates for:

- filesystem
- GitHub
- Postgres/database
- Slack/messaging
- shell
- descriptor integrity

The generated files are starter policies. They are deliberately small,
verifiable, and easy to edit. They include rule metadata that names the template
and graph-path category that motivated the rule.

The command refuses to overwrite existing files unless `--force` is provided.
It does not read or modify MCP client config files. It writes only to the
operator-selected policy output directory.

## Claim Boundary

This release proves inventory-derived risk graph rendering and starter policy
generation. It does not prove production policy completeness, runtime
write-after-taint denial, descriptor hash locking, live GitHub App conformance,
or universal MCP exploit detection. Protection begins only when operators route
tool calls through Boundary with reviewed policies.
