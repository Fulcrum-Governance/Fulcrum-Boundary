# Quickstart

Install the CLI and run the local smoke path.

## Install

Requires Go 1.25+.

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.7.0
boundary selftest
boundary demo github-lethal-trifecta
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

## From Source

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary
go run ./cmd/boundary selftest
go run ./cmd/boundary demo github-lethal-trifecta
```

## Useful Local Gates

```bash
make selftest
make demo-github
make release-check
```
