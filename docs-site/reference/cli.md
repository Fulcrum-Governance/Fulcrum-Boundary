# CLI Reference

Canonical repository reference:
[docs/CLI_REFERENCE.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/CLI_REFERENCE.md)

First-run commands:

```bash
boundary version
boundary selftest
boundary doctor --json
boundary demo action-boundary
boundary demo github-lethal-trifecta
boundary demo command-secret-exfil
boundary evidence bundle --include-demo
boundary evidence verify boundary-evidence
```

Firewall commands:

```bash
boundary inventory --help
boundary graph --help
boundary policy generate --help
boundary inventory ingest --help
boundary dashboard --help
```

Secure GitHub commands:

```bash
boundary secure github --help
boundary secure github conformance --help
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance denied-write
boundary redteam --pack github-lethal-trifecta
```

Command Boundary preview commands:

```bash
boundary command classify --help
boundary command run --help
boundary command install --project
boundary shell --help
boundary demo command-secret-exfil
```

Decision-record commands:

!!! note "Availability — in the v0.8.0 release"
    `boundary explain`, `boundary replay`, and `schema_version "2"` (route-context)
    decision records are in the `v0.8.0` release, so `go install …@v0.8.0` includes
    them. `boundary verify-record` on a `schema_version "1"` or `"2"` record is
    included in `v0.8.0` as well.

```bash
boundary verify-record record.json
boundary explain record.json
boundary explain --json docs/examples/decision-record-v2.example.json
boundary replay record.json --request request.json --policies ./policies/
```

`boundary explain` is local-only and read-only: it describes a decision record
(schema_version 1 or 2) and does not verify its hashes. Run
`boundary verify-record` to recompute them.

`boundary replay` is local-only and fixture-safe: it re-evaluates the recorded
request against the recorded policy bundle and compares the decision-defining
fields (`action`, `reason`, `decision_mode`, `matched_rule`, `policy_file`) —
not `action` alone. It reproduces the decision, not enforcement, and does not
prove that no upstream bytes moved.

Policy test commands:

!!! note "Availability — post-v0.8.0 on main"
    `boundary test` is a post-`v0.8.0` command on `main` after the Phase 1
    policy-testing lane merges. The `@v0.8.0` install does not include it.

```bash
boundary test --path tests/fixtures/policy-test/cases
boundary test --path tests/fixtures/policy-test/cases --format json
```

`boundary test` is local-only and fixture-safe. It reports policy verdicts for
routed request fixtures only; it does not prove production route enforcement or
deployment bypass resistance.

Release verification commands:

```bash
make selftest
make demo-github
make release-check
boundary test --path tests/fixtures/policy-test/cases
boundary evidence bundle --include-demo --out /tmp/boundary-evidence
boundary evidence verify /tmp/boundary-evidence
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
```
