# CLI Reference

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
boundary redteam --pack github-lethal-trifecta
```

Release verification commands:

```bash
make selftest
make demo-github
make release-check
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```
