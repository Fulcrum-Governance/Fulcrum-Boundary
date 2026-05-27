# Launch Truth Freeze

This file records the release-facing truth for Fulcrum Boundary v0.2.0. It is a claims boundary for the OSS release surface, not a competitor benchmark or marketing claim registry.

## Product Identity

| Surface | Release value |
|---|---|
| OSS project name | Fulcrum Boundary |
| GitHub repository | `Fulcrum-Governance/Boundary` |
| Go module path | `github.com/fulcrum-governance/boundary` |
| CLI binary | `boundary` |
| First release campaign | MCP Safety Gateway |
| Primary release claim | Boundary evaluates an agent action before it reaches the privileged tool when the deployment routes that action through Boundary. |

Historical names and repository redirects are intentionally omitted from release-facing docs. Public setup instructions should point to the current module path and repository only.

## What v0.2.0 Proves

The MCP Safety Gateway demo proves the release spine:

- a safe `SELECT` request is allowed through Boundary
- a destructive demo `DROP TABLE` request is denied before execution
- a direct bypass attempt from the demo agent fails by network topology
- every verdict emits a structured decision record
- the Postgres path uses an AST guard for statement classification

The bypass claim is scoped to the Docker demo topology. Production deployments must enforce the same sole-route constraint with their own infrastructure controls.

## What v0.2.0 Does Not Claim

Fulcrum Boundary v0.2.0 does not claim:

- general SQL firewall coverage
- universal SQL injection prevention
- receipt verification
- trust-based adaptive termination
- multi-agent coordination governance
- benchmark superiority
- compliance certification

The release uses the term **decision records**. Do not call them receipts until verification mechanics such as policy hashes, request hashes, and record verification are implemented.

## Verified Release Surface

| Surface | Status |
|---|---|
| `cmd/boundary/` CLI | Present |
| `examples/mcp-postgres-gateway/` Docker demo | Present |
| YAML policy loading | Present |
| Structured decision records | Present |
| `docs/DECISION_RECORDS.md` | Present |
| `docs/LIMITATIONS.md` | Present |
| `docs/BOUNDARY_CONDITIONS.md` | Present |
| `docs/THREAT_MODEL.md` | Present |
| `SECURITY.md` | Present |
| `CONTRIBUTING.md` | Present |
| `CHANGELOG.md` v0.2.0 section | Present |

## Language Lock

Use:

- Fulcrum Boundary
- Boundary
- `boundary` CLI
- MCP Safety Gateway
- action boundary
- pre-execution control
- decision record

Do not use as public release names:

- Zero-Trust MCP Router
- MCP gateway as the whole project identity
- governance platform as the lead phrase
- receipts for v0.2.0 decision records

Adapters change. The boundary does not.
