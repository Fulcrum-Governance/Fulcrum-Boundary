# Quickstart

Install the CLI and run the local smoke path.

## Install

Requires Go 1.25+.

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.9.0
boundary selftest
boundary demo github-lethal-trifecta
boundary test --path tests/fixtures/policy-test/cases
```

Expected demo success signal:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

No credentials are required. The selftest and demo use fixture data and do not
perform live GitHub calls or real system mutation.

Boundary governs actions only when the route is forced through Boundary.

`boundary test` is the v0.9.0 developer-trust step: it evaluates local policy
bundles against routed request fixtures and exits non-zero on unexpected
verdicts. See [Policy Testing](reference/policy-testing.md).

## From Source

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary
go run ./cmd/boundary selftest
go run ./cmd/boundary demo github-lethal-trifecta
go run ./cmd/boundary test --path tests/fixtures/policy-test/cases
```

## Useful Local Gates

```bash
make selftest
make demo-github
make release-check
```
