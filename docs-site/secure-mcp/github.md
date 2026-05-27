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
- Live GitHub App conformance is required before production status.
- Deployment bypass evidence is required before production status.
- Direct GitHub API or upstream MCP access remains outside Boundary unless the
  operator removes that route.
