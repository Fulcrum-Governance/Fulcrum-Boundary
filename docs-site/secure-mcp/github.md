# Secure GitHub

Secure GitHub is a preview Secure MCP profile for the tested GitHub
write-after-taint path.

```bash
boundary secure github setup --out secure-github-fixture
boundary secure github serve --fixture --dry-run
boundary redteam --pack github-lethal-trifecta
```

Preview status matters:

- Fixture mode proves the tested denial path.
- Opt-in live GitHub App conformance can record read-taint and no-mutation
  evidence for an operator-owned test repository.
- Deployment bypass evidence is required before production status.
- Direct GitHub API or upstream MCP access remains outside Boundary unless the
  operator removes that route.

Live conformance:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance denied-write
```

Expected signals include `actual action: DENY`,
`reason: lethal_trifecta_detected`, `upstream_called=false`, and
`github_mutation_called=false`.
