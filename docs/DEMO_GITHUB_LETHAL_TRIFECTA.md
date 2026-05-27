# GitHub Lethal-Trifecta Demo

`boundary demo github-lethal-trifecta` runs a local fixture that shows the Secure GitHub preview profile denying a private-repository write after untrusted GitHub context has entered the session.

The demo is fixture-only. It does not require credentials, does not call GitHub, and does not mutate a real repository.

## Commands

Run the text report:

```bash
boundary demo github-lethal-trifecta
```

Run the machine-readable report:

```bash
boundary demo github-lethal-trifecta --json
```

Write a Markdown report and local dashboard artifact:

```bash
boundary demo github-lethal-trifecta --markdown --out demo-report.md --dashboard
```

When `--out` is omitted, Boundary creates an isolated temporary workspace for fixture artifacts. When `--out` is provided, Boundary writes supporting artifacts next to the report under `github-lethal-trifecta-artifacts/`.

## What Runs

The command performs the release demo path end to end:

1. Writes a fixture MCP config for a GitHub MCP server.
2. Builds MCP Firewall inventory from that config.
3. Builds the inventory-derived risk graph.
4. Generates starter policies and verifies them with the Boundary policy loader.
5. Writes Secure GitHub fixture profile and policy artifacts.
6. Runs the `github-lethal-trifecta` redteam pack.
7. Runs the Secure GitHub fixture adapter through a read step followed by a protected write step.
8. Emits decision records for the denial path.
9. Optionally renders a local-only HTML dashboard over the fixture artifacts.

## Expected Result

The report must show:

```text
expected action: DENY
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

The read step reaches the fixture upstream to establish taint. The later private-repository write is denied before upstream execution.

## What This Proves

- Boundary can discover the fixture GitHub MCP surface and identify private-repository mutation tools.
- Boundary can render the risk path from untrusted GitHub context to private-repository mutation.
- Boundary starter policies are generated and parse successfully.
- The Secure GitHub preview fixture can deny write-after-taint private-repository mutations before upstream execution.
- The denial path emits receipt-grade decision-record evidence.

## What This Does Not Prove

- It does not prove live GitHub App conformance.
- It does not prove production deployment bypass resistance.
- It does not mutate GitHub, call the network, or validate real credential handling.
- It does not replace the Managed Agents or live Secure GitHub conformance gates.

## Claim Boundary

This demo supports the preview Secure GitHub claim only. Public language should say that Boundary has a fixture-backed Secure GitHub preview path for write-after-taint denial before upstream execution. Do not claim universal GitHub safety, production Secure GitHub readiness, or live GitHub conformance from this fixture.
