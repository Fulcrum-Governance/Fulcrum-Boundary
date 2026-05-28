# CLI Reference

Canonical repository reference:
[docs/CLI_REFERENCE.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/CLI_REFERENCE.md)

First-run commands:

```bash
boundary selftest
boundary demo github-lethal-trifecta
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
boundary redteam --pack command-secret-exfil
```

Release verification commands:

```bash
make selftest
make demo-github
make release-check
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```
