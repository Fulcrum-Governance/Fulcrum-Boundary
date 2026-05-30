# Test Fixtures (synthetic, no real secrets)

Everything under `fixtures/` is **intentional, synthetic test data** used by
Boundary's unit, integration, and red-team suites. None of it contains a real
secret, credential, token, or live endpoint. Nothing here is read, executed, or
sent anywhere at runtime — the red-team tests assert that the proposed actions
are classified and **denied before execution** (`Executed == false`).

If a secret scanner flags this directory on clone, it is a **false positive**:
the payloads are designed to *look* like the dangerous patterns Boundary
detects so the tests can prove Boundary denies them. An allowlist is provided
(`.gitleaks.toml`, `.trivyignore`) so common scanners stay clean.

## What's here

### `fixtures/editboundary/` — Edit Boundary diff fixtures

Proposed file mutations fed to the Edit Boundary classifier. The red-team
diffs under `redteam/` model dangerous edits; each is denied or held for
approval and never applied.

| Fixture | Models | Synthetic marker (not a real secret) |
|---|---|---|
| `secret.diff` | Writing a `.env` with a credential | `API_KEY=example-secret` |
| `redteam/secret-exfil.diff` | Creating a `.env` to stage exfiltration | `BOUNDARY_FIXTURE_VALUE=redacted_fixture_value`, `…=synthetic` |
| `redteam/curl-pipe-script.diff` | `curl … | sh` bootstrap script | target host `example.invalid` |
| `redteam/destructive-delete.diff` | Destructive filesystem delete | — |
| `redteam/ci-deploy.diff` | Unreviewed CI/deploy change | — |
| `redteam/dockerfile.diff` | Risky Dockerfile mutation | — |
| `redteam/terraform.diff` | Risky infrastructure mutation | — |
| `redteam/cross-scope.diff` | Edit crossing an allowed scope | — |
| `redteam/package-script.diff`, `package-scripts.diff` | Package lifecycle script injection | — |
| `docs.diff` | A benign docs-only edit (allowed) | — |

### `fixtures/external-inventory/` — external MCP inventory fixtures

NDJSON inventories fed to `--source external-mcp`. Boundary-owned mapping
format only; not an official third-party integration or compatibility claim.
No secrets, no live endpoints.

## Why the values look "real"

`example-secret`, `redacted_fixture_value`, `synthetic`, and `example.invalid`
are deliberately non-functional placeholders. `example.invalid` is a reserved
non-resolvable domain (RFC 2606 / RFC 6761). The fixtures exist so the test
suite can demonstrate detection and denial — proving the boundary fires, not
shipping a secret.
