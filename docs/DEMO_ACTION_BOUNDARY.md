# Action Boundary Demo

`boundary demo action-boundary` runs a local fixture demo across three routed
surfaces:

- MCP / Secure GitHub
- Command Boundary
- Edit Boundary

The demo is fixture-only. It uses no credentials, does not call the network, and
does not mutate live systems.

## Commands

Run the text report:

```bash
boundary demo action-boundary
```

Run the machine-readable report:

```bash
boundary demo action-boundary --json
```

Write a Markdown report:

```bash
boundary demo action-boundary --markdown --out demo.md
```

Write local-only dashboard artifacts:

```bash
boundary demo action-boundary --dashboard --out .boundary/action-boundary-demo
```

## What Runs

The command composes existing fixture paths. It does not create a fourth action
surface or a separate governance pipeline.

| Surface | Fixture | Expected result |
| --- | --- | --- |
| MCP / Secure GitHub | poisoned issue context followed by private-repository write | `DENY`, `reason: lethal_trifecta_detected`, `upstream_called=false` |
| Command Boundary | `git push origin main` | `require_approval`, `class: C3`, `risk: HIGH`, `executed=false` |
| Edit Boundary | `.env` secret-bearing patch | `DENY`, `class: E4`, `risk: CRITICAL`, `applied=false` |

## What This Proves

- Boundary can deny the fixture Secure GitHub private-repository write before
  upstream execution.
- Boundary can classify a fixture repository-mutation command and avoid
  executing it.
- Boundary can classify a fixture secret-bearing edit and avoid applying it.
- The three routed surfaces share the same action-boundary product model:
  classify, evaluate before execution, and record the decision.

## What This Does Not Prove

- It does not prove live GitHub App conformance.
- It does not prove global shell control.
- It does not prove direct file-edit interception.
- It does not prove production deployment bypass resistance.
- It does not prove universal coding-agent safety.

## Claim Boundary

Use this wording:

> Boundary can demonstrate fixture-backed pre-execution control across MCP /
> Secure GitHub, Command Boundary, and Edit Boundary routed surfaces.

Do not say Boundary controls all shell commands, governs direct editor writes,
prevents every overeager agent action, or provides production readiness for
preview surfaces from this fixture alone.
