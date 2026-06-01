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

```bash
boundary verify-record record.json
boundary explain record.json
boundary explain --json docs/examples/decision-record-v2.example.json
```

`boundary explain` is local-only and read-only: it describes a decision record
(schema_version 1 or 2) and does not verify its hashes. Run
`boundary verify-record` to recompute them.

Release verification commands:

```bash
make selftest
make demo-github
make release-check
boundary evidence bundle --include-demo --out /tmp/boundary-evidence
boundary evidence verify /tmp/boundary-evidence
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
```
