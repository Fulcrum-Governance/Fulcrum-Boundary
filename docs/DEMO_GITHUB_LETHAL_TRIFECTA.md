# Lane 1: MCP Demo - GitHub Lethal-Trifecta

`boundary demo github-lethal-trifecta` is Lane 1 of Boundary's two-lane proof
spine. It exercises the MCP route, the first production route, through a local
fixture that shows the Secure GitHub preview profile denying a
private-repository write after untrusted GitHub context has entered the session.

The demo is fixture-only. It does not require credentials, does not call GitHub, and does not mutate a real repository.

For the equal-weight Command Boundary proof lane, run
`boundary demo command-secret-exfil` and read
[command-boundary/DEMO.md](./command-boundary/DEMO.md). The two-lane overview is
[DEMOS.md](./DEMOS.md).

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

## Evidence Pack (primary artifact)

The demo's primary deliverable is a fixture-only **evidence pack**, not the
recording. Generate it with:

```bash
boundary demo github-lethal-trifecta --evidence-pack ./github-lethal-trifecta-pack
```

The pack directory contains a hashed manifest and the artifacts a reviewer can
re-verify offline:

| Artifact | What it is |
|---|---|
| `pack.json` | Manifest: status, `secure_github_status: preview`, `bypass_ladder_level: L0`, and a SHA-256 + size for every other artifact. |
| `decision-record.json` | The denial `DecisionRecordV1` (verify with `boundary verify-record`). |
| `proof-receipt.json` | The `proof-receipt-v0.1` sidecar — a checker-validated proof-receipt bound to the record by `decision_hash`, naming the checker build and the budget / static-privilege invariants it validated. It is attached evidence, not a `proved` decision mode. |
| `route-conformance.json` | Route assertions: write denied before upstream, read reached upstream, decision mode is not `proved`, record hash verifies, receipt checker verifies. |
| `tamper-cases.json` | Negative cases: a forged verdict (record hash rejects it) and a broken receipt binding (the checker rejects it). |
| `caveats.md` | What the pack does not prove (L0 / preview / bypass). |
| `route-topology` diagram | `docs/assets/github-lethal-trifecta-route-topology.mmd`: the forced route and the bypass paths a deployment must deny. |

### Offline re-verification

The demo pack uses a demo-specific manifest schema
(`boundary.demo.github_lethal_trifecta.evidence_pack.v1`) and is **not**
consumed by the generic `boundary evidence verify` command (which expects
`manifest.json` with schema `boundary.evidence_bundle.v1`). Do not run
`boundary evidence verify <pack-dir>` against this pack — it will not
recognise the manifest.

To re-verify the pack offline:

1. **Recompute artifact hashes.** For each artifact listed in `pack.json`,
   recompute its SHA-256 and compare against the `sha256` field in the
   manifest's entries.

   ```bash
   sha256sum github-lethal-trifecta-pack/decision-record.json
   sha256sum github-lethal-trifecta-pack/proof-receipt.json
   # etc.
   ```

2. **Verify the decision record.** `boundary verify-record` works against the
   decision record directly:

   ```bash
   boundary verify-record github-lethal-trifecta-pack/decision-record.json
   # record verification: ok
   ```

3. **Confirm receipt binding.** The proof-receipt sidecar is bound to the
   record by `decision_hash`. The `VerifyBinding` check in the sidecar
   confirms the receipt corresponds to the same record that `verify-record`
   verified. The receipt is not hash-chained and not signed by default; it is
   invariant evidence attached to the record, not a `proved` decision mode.

`boundary replay ./github-lethal-trifecta-pack/decision-record.json`
re-evaluates the recorded request and compares the live verdict against the
stored decision. The pack shows the **wired** proof receipt, not a mock: the
receipt is a checker-validated proof-receipt sidecar scoped to the same record the
verifier checks. Secure GitHub remains preview at this fixture L0 level.

## What This Does Not Prove

- It does not prove live GitHub App conformance.
- It does not prove production deployment bypass resistance.
- It does not mutate GitHub, call the network, or validate real credential handling.
- It does not replace the Managed Agents or live Secure GitHub conformance gates.

## Claim Boundary

This demo supports the preview Secure GitHub claim only. Public language should say that Boundary has a fixture-backed Secure GitHub preview path for write-after-taint denial before upstream execution. Do not claim universal GitHub safety, full GitHub production-readiness, or live GitHub conformance from this fixture.
