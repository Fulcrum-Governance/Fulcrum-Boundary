# Quickstart

Install the CLI and run the local smoke path.

## Install

Requires Go 1.25+.

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0
boundary selftest
boundary demo github-lethal-trifecta
boundary demo command-secret-exfil
boundary test --path tests/fixtures/policy-test/cases
```

Prebuilt channels are also available for the current launch release:
Homebrew (`brew install fulcrum-governance/tap/boundary`), release archives,
and the container image (`ghcr.io/fulcrum-governance/boundary:v0.11.0`). See
the canonical [Install](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/INSTALL.md)
guide before choosing static versus cgo builds.

Expected MCP demo success signal:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

Expected Command Boundary demo success signal:

```text
actual: DENY
executed=false
class=C6
```

No credentials are required. The selftest and demo use fixture data and do not
perform live calls or real system mutation.

Boundary governs actions only when the route is forced through Boundary.

`boundary test` is the local developer-trust step: it evaluates local policy
bundles against routed request fixtures and exits non-zero on unexpected
verdicts. It is included in the current `v0.11.0` release. See
[Policy Testing](reference/policy-testing.md).

## From Source

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary
go run ./cmd/boundary selftest
go run ./cmd/boundary demo github-lethal-trifecta
go run ./cmd/boundary demo command-secret-exfil
go run ./cmd/boundary test --path tests/fixtures/policy-test/cases
```

## Useful Local Gates

```bash
make selftest
make demo-github
make release-check
```
