# Troubleshooting

Canonical repository reference:
[docs/TROUBLESHOOTING.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/TROUBLESHOOTING.md)

Covers the most common first-run problems and how to resolve them:

- Go 1.25+ toolchain requirement.
- Static versus cgo builds: default source builds link the full Postgres AST
  guard through cgo and need a C toolchain; `CGO_ENABLED=0` source builds,
  Homebrew, container images, and `_static-nocgo` archives run the static
  fail-closed SQL classifier.
- `PATH` issues after `go install` (GOBIN / GOPATH).
- The failure modes of each first-run command (`selftest`, `doctor`, the two
  demos, `evidence bundle` / `verify`, `verify-record`) and how to resolve them.
- How to read `boundary doctor --json`: what each field means, and what a clean
  versus flagged result looks like.

The canonical first-run sequence these notes follow:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0
boundary selftest
boundary doctor --json
boundary demo github-lethal-trifecta
boundary demo command-secret-exfil
boundary evidence bundle --include-demo --out boundary-evidence
boundary evidence verify boundary-evidence
```

MCP is the first production route. Command Boundary, Edit Boundary, Secure
GitHub, and the remaining adapters are preview. A passing first-run sequence
exercises local fixtures; it does not prove production deployment protection.
