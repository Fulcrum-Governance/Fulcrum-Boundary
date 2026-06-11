# Adapter Production Bar

This document explains what the `production` maturity label means for a
Boundary adapter, enumerates the mechanical bar field by field, and describes
the process a contributor must follow to earn it. Read it before opening a
pull request that proposes advancing an adapter's status from `preview` to
`production`.

The machine-readable source of truth for each adapter's current state is its
`readiness.yaml` declaration. The automated gate that enforces the label is
`tests/adapter_conformance/adapter_readiness_test.go`.

---

## What the Production Label Means (and Does Not Mean)

**What it means.** A `production` adapter has a complete, formally declared
lifecycle: every one of the ten lifecycle steps is either directly implemented
or formally delegated to a named owner with a documented contract, at least one
fail-closed transport is declared, and integration test evidence is on disk. The
label reflects what has been verified in the repository today.

**What it does not mean.** The label is not a bypass-resistance guarantee.
Boundary governs only routes forced through it. An adapter can be `production`
and still be bypassable if the deployment topology permits a direct path to the
upstream tool. The `bypass_proof` step is `delegated` even for the MCP adapter
— it is assigned to `deployment network topology` because code alone cannot
prove that no direct path exists. `production` means the adapter is ready for
deployment scenarios that close that path; it does not mean those scenarios are
in place for any particular installation.

Routed-only coverage is a foundational constraint, not a gap to fix. Every
public description of a production adapter should preserve that framing.

---

## The Mechanical Bar, Field by Field

The gate `TestProductionAdaptersPassConformanceRules` in
`tests/adapter_conformance/adapter_readiness_test.go` enforces these rules
automatically. Each entry below maps the YAML field to the check that fails if
the field is wrong.

### `status: production`

The field in `readiness.yaml`. Setting this to `production` activates all
checks below. Changing it from `preview` to `production` is the gated
action — the conformance suite rejects anything that fails any of the following
conditions.

### No `stub` lifecycle steps

```yaml
lifecycle:
  parse: implemented        # or delegated — stub is never allowed
  identify: implemented
  evaluate: implemented
  deny: implemented
  forward: implemented
  inspect: implemented
  metadata: implemented
  record: delegated
  bypass_proof: delegated
  fail_closed: implemented
```

`stub` is the only disqualifying state. `delegated` is fully legitimate: it
means the step is handled by a named owner whose contract is documented (see
`delegated_steps` below). A step listed as `stub` means the implementation is
a placeholder that returns a zero value without real behavior. The gate in
`TestProductionAdaptersPassConformanceRules`:

```go
for step, state := range decl.Lifecycle {
    if state == string(governance.AdapterStepStub) {
        t.Fatalf("%s is production but lifecycle step %s is still stub", ...)
    }
}
```

The ten lifecycle step names are defined in `governance/adapter_lifecycle.go`
and must all be present (checked by `requireDeclaration`). Their meanings are
documented in `docs/ADAPTER_READINESS_MATRIX.md`.

### `bypass_proof` is `implemented` or `delegated`

`bypass_proof` must not be `stub` or absent. `delegated` is acceptable because
network-topology proof cannot be encoded in a Go source file. When delegated,
the `delegated_steps` block must name an owner and a contract file that exists
on disk:

```yaml
delegated_steps:
  - step: bypass_proof
    owner: deployment network topology
    contract: docs/BOUNDARY_CONDITIONS.md
```

The gate:

```go
state := decl.Lifecycle[string(governance.AdapterStepBypassProof)]
if state != string(governance.AdapterStepImplemented) &&
   state != string(governance.AdapterStepDelegated) {
    t.Fatalf(...)
}
```

The `requireDeclaration` helper additionally checks that every file named in
`delegated_steps[*].contract` exists on disk.

### At least one `fail_closed_transports` entry

```yaml
fail_closed_transports:
  - mcp
```

This is not just documentation — the conformance gate enforces it:

```go
if len(decl.FailClosedTransports) == 0 {
    t.Fatalf("%s is production but declares no fail-closed transports", ...)
}
```

The `fail_closed` lifecycle step describes whether the adapter returns a denial
on governance errors. `fail_closed_transports` names which transport names
trigger that behavior at the pipeline level. At least one is required for a
`production` declaration. The choice of which transports to include is
adapter-specific; consult `docs/BOUNDARY_CONDITIONS.md` and
`docs/security/FAIL_MODE_MATRIX.md` when deciding.

### Test evidence paths exist on disk

```yaml
evidence:
  tests:
    - adapters/mcp/adapter_test.go
    - tests/integration/mcp_gateway_lifecycle_test.go
    - internal/boundarycli/cli_test.go
  docs:
    - docs/BOUNDARY_CONDITIONS.md
    - docs/adapters/MCP.md
    - docs/ADAPTER_READINESS_MATRIX.md
```

For `production` adapters the `evidence.tests` list must be non-empty:

```go
if len(decl.Evidence.Tests) == 0 {
    t.Fatalf("%s is production but has no conformance test evidence", ...)
}
```

The `requireDeclaration` helper checks that every path under both
`evidence.tests` and `evidence.docs` exists on disk. A path that does not
exist fails the build. Test evidence does not need to be in the same file as
the adapter; the integration suite in `tests/integration/` and the
adapter-conformance suite in `tests/adapter_conformance/` both count.

### Readiness matrix and README rows

`TestEveryAdapterDeclaresReadiness` enforces that:

- `README.md` contains a maturity heading for the adapter's status (e.g.
  `### Production`) and a reference to `` `adapters/<name>` ``.
- `docs/ADAPTER_READINESS_MATRIX.md` contains a row matching
  `| <adapter> | production |`.

Both files must be updated in the same change that advances the status.

---

## The Process

1. **Open an issue first.** CONTRIBUTING.md requires this for adapter
   status changes. State which adapter you are advancing, what gaps currently
   block it (see its `gaps:` list), and how you plan to close them.

2. **Close every listed gap.** A `production` declaration must have an empty
   `gaps:` list, or no `gaps:` key. Each gap is identified by a structured ID
   (`BND-<AREA>-<NNN>`) with a description and a spec reference. Gaps are not
   suppressed — they are resolved by delivering the missing behavior.

3. **Fill `readiness.yaml` truthfully.** Change `status:` to `production` and
   `target_status:` to `production`. Advance every lifecycle step from `stub`
   to `implemented` or `delegated`. Add `delegated_steps` entries for every
   delegated step. Add at least one `fail_closed_transports` entry. Ensure all
   evidence paths exist on disk.

4. **Run the conformance suite green.**

   ```bash
   go test ./tests/adapter_conformance/... -count=1 -run TestProductionAdaptersPassConformanceRules
   go test ./tests/adapter_conformance/... -count=1 -run TestEveryAdapterDeclaresReadiness
   ```

   Both must pass. Running the full claims gate is also required:

   ```bash
   go test ./claims/... -count=1
   ```

5. **Update the readiness matrix and README in the same change.** The
   conformance gate checks both. See `docs/ADAPTER_READINESS_MATRIX.md` for
   the table format and README.md for the `### Production` / `### Preview`
   adapter lists.

6. **Add a `claims/boundary_claims.yaml` entry if the public story changes.**
   If advancing to `production` causes any public claim to change (for example,
   a new transport now forms part of the production surface), add or update the
   ledger entry in the same PR. Claims that are `delivered` must reference test
   and doc paths on disk. See `claims/claims_test.go` for the full build-gate
   rules.

7. **Run `make release-check` before requesting review.** The release gate runs
   the full suite including live CLI invocations and the docs build.

---

## Worked Example: What MCP Has That Webhook Does Not

The following table compares the two adapters to make the gap concrete. All
values are taken directly from their `readiness.yaml` declarations.

| Field | `adapters/mcp` | `adapters/webhook` |
|---|---|---|
| `status` | `production` | `preview` |
| `lifecycle.forward` | `implemented` | `delegated` |
| `lifecycle.metadata` | `implemented` | `delegated` |
| `lifecycle.record` | `delegated` (to `governance.AuditPublisher`) | `delegated` (to `governance.AuditPublisher`) |
| `lifecycle.bypass_proof` | `delegated` (to deployment topology, contract `docs/BOUNDARY_CONDITIONS.md`) | `delegated` (to deployment topology, contract `docs/adapters/WEBHOOK.md`) |
| `lifecycle.fail_closed` | `implemented` | `delegated` (to `webhook.Handler execution mode`) |
| `fail_closed_transports` | `[mcp]` | `[webhook]` |
| `gaps` | `[]` (empty) | `[BND-WEB-001]` |
| `evidence.tests` | 3 paths, all on disk | 2 paths, both on disk |

**What MCP has that webhook does not.** MCP implements `forward` and `metadata`
directly inside the `mcp.Gateway`: the adapter owns the transport end-to-end
and can attach governance headers to the upstream response before returning it.
Webhook's `forward` step is delegated — in informational mode, Boundary
receives a webhook after execution and cannot deny before the fact; in
execution mode, Boundary is in the approval path but the forwarding
responsibility sits with the host. That architectural distinction accounts for
most of the gap.

Webhook also carries `BND-WEB-001`: production status requires deployment
evidence that execution webhooks are the sole downstream action path, and that
informational webhooks do not serve as a bypass for execution webhooks. Until
that evidence is recorded and the gap removed, the adapter is `preview`
regardless of how many lifecycle steps are `implemented`.

MCP's `gaps` list is empty. Every step is either directly implemented or
delegated to a named owner with a contract file that exists on disk. The
conformance suite, integration tests, and CLI smoke tests all pass against real
`boundary serve` behavior.

---

## The Bar Earns a Label, Not a Guarantee

Passing the conformance suite means the adapter satisfies a documented,
mechanically tested set of criteria. It does not mean:

- The adapter is protected in any particular deployment. A `production` adapter
  in a topology that does not remove the direct tool path does not govern that
  tool.
- The adapter is free of bugs or edge-case failures. The test suite covers
  declared behavior; it does not exhaustively cover adversarial inputs.
- The adapter's bypass proof is in place. `bypass_proof: delegated` means the
  responsibility is acknowledged and documented, not that it has been enforced.

The `production` label is accurate as far as it reaches: a complete lifecycle
is declared, integration evidence is on disk, and the conformance gate is green.
What that covers is exactly what the readiness declaration says it covers —
nothing more.
