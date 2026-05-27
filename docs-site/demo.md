# Demo

The flagship local demo is the GitHub write-after-taint fixture:

```bash
boundary demo github-lethal-trifecta
```

Expected success signal:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

What it proves:

- Boundary can inventory a fixture GitHub MCP server.
- Boundary can render an untrusted GitHub issue to private-repo mutation risk
  path.
- Boundary can generate starter policies that parse through its verifier.
- Secure GitHub preview can deny the tested write-after-taint fixture before
  upstream GitHub mutation.
- Boundary emits a decision record for the governed route.

What it does not prove:

- It does not prove universal prompt-injection prevention.
- It does not prove production GitHub security.
- It does not call live GitHub or mutate a real repository.
- It does not protect tools that bypass Boundary.
