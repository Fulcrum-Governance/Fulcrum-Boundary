# Release Utilities

The v0.6.x utility train adds local commands for proving, diagnosing, and
packaging the current Boundary surface.

```bash
boundary version
boundary doctor --json
boundary demo action-boundary
boundary evidence bundle --include-demo --out boundary-evidence
boundary evidence verify boundary-evidence
```

These commands are local-first. They do not require credentials, do not make
live GitHub calls by default, and do not mutate real systems by default.

## What They Prove

| Command | Proof |
| --- | --- |
| `boundary version` | The binary can report local build metadata, module path, Go runtime, and schema-versioned JSON. |
| `boundary doctor` | Local routed-surface diagnostics and bypass caveats can be rendered without network calls. |
| `boundary demo action-boundary` | Fixture-only MCP / Secure GitHub, Command Boundary, and Edit Boundary paths can be shown together. |
| `boundary evidence bundle` | Local release artifacts can be packaged with a manifest and SHA-256 hashes. |
| `boundary evidence verify` | A local evidence bundle can be checked for manifest shape, artifact existence, hash integrity, and fixture-safe summary references. |

## What They Do Not Prove

- Production route enforcement.
- Deployment bypass resistance.
- Production Secure GitHub, Command Boundary, or Edit Boundary maturity.
- Universal attack prevention.
- Cryptographic release provenance.

MCP remains the production adapter path. Secure GitHub, Command Boundary, and
Edit Boundary remain preview surfaces.
