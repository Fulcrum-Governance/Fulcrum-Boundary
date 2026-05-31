# Demo

The flagship local demo is the GitHub write-after-taint fixture.

## Exact Command

```bash
boundary demo github-lethal-trifecta
```

Expected success signal:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

![Boundary denies a GitHub write-after-taint action before upstream execution](assets/github-lethal-trifecta-demo.gif)

A real run of `boundary demo github-lethal-trifecta`. The static
[deny-before-upstream walkthrough](assets/boundary-demo-walkthrough.svg) is a
no-JS fallback and is a stylized diagram, not a literal capture.

## What It Proves

- Boundary can inventory a fixture GitHub MCP server.
- Boundary can render an untrusted GitHub issue to private-repo mutation risk
  path.
- Boundary can generate starter policies that parse through its verifier.
- Secure GitHub preview can deny the tested write-after-taint fixture before
  upstream GitHub mutation.
- Boundary emits a decision record for the governed route.

## What It Does Not Prove

- It does not prove protection against every malicious prompt.
- It does not make Secure GitHub a production surface.
- It does not call live GitHub or mutate a real repository.
- It does not protect tools that bypass Boundary.
- It does not prove production route enforcement.

## Decision Record

Each governed denial emits a hash-verifiable decision record carrying the
verdict, reason, and a decision hash. It is recorded evidence of the fixture
run, not a live GitHub run, and does not prove production route enforcement.

## Source Doc

Read the canonical source demo doc:
[docs/DEMO_GITHUB_LETHAL_TRIFECTA.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/DEMO_GITHUB_LETHAL_TRIFECTA.md).
