# Boundary Selftest

`boundary selftest` runs a local, no-credential release smoke test for the public Boundary surface.

It is intentionally fixture-only. It does not contact GitHub, does not start a network listener, does not mutate live systems, and does not require secrets.

## Run

```bash
boundary selftest
boundary selftest --json
boundary selftest --no-color
```

The text output is meant for humans and stable enough for fixture tests. The JSON output uses schema `boundary.selftest.v1` and is suitable for scripts.

## Checks

The selftest currently verifies:

1. The CLI command dispatcher reaches `boundary selftest`.
2. A GitHub MCP inventory fixture loads with env values redacted.
3. The inventory risk graph renders, including repo-write paths.
4. Starter firewall policies generate and pass the Boundary policy loader.
5. A descriptor lock verifies cleanly against the baseline fixture.
6. A modified descriptor is detected as drift and fails closed by default.
7. The GitHub lethal-trifecta redteam fixture denies before upstream execution.
8. Secure GitHub live mode fails closed because live GitHub App mode is not implemented in the preview profile.
9. A receipt-grade decision record is emitted by the redteam fixture.
10. The output points to `go test ./claims/... -count=1` for claims validation without running the full claims suite by default.

## Boundaries

Selftest is not a production conformance suite. It proves the local release fixtures still boot and agree with the current public release story.

- Credentials: none.
- Network: none.
- Live mutation: none.
- GitHub: fixture-only.
- Claims validation: pointed to, not run automatically.

Run the full release gates before a release candidate:

```bash
make release-check
make docs-build
go test ./internal/selftest/... -count=1 -timeout 5m
go test ./tests/selftest/... -count=1 -timeout 5m
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git ls-files '*.go' | xargs gofmt -l
```

`make release-check` is the release superset. It runs the public-surface guards,
vet, the root and gRPC test suites, claims tests, policy verification,
`verify-record --help`, `boundary selftest`, the fixture demos, doctor/evidence
checks, and `boundary test --path tests/fixtures/policy-test/cases`.
