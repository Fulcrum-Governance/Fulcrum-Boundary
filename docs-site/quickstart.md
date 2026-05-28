# Quickstart

Install the CLI and run the local smoke path:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.1
boundary selftest
boundary demo github-lethal-trifecta
```

No credentials are required. The selftest and demo use fixture data and do not
perform live GitHub calls or real system mutation.

From source:

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary
go run ./cmd/boundary selftest
go run ./cmd/boundary demo github-lethal-trifecta
```

Useful local gates:

```bash
make selftest
make demo-github
make release-check
```
