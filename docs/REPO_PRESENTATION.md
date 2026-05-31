# Repository Presentation

This page records the intended GitHub repository presentation for Fulcrum
Boundary. It should stay claim-safe, concrete, and free of fake adoption or
release signals.

## Repository Description

Use this GitHub repository description:

> The action boundary for routed agent tools. See what your AI tools can do; block what they should not.

This is shorter than the README tagline and fits GitHub's repository
description field.

## Topics

Use these topics:

- `mcp`
- `model-context-protocol`
- `ai-agents`
- `agent-security`
- `agent-governance`
- `mcp-security`
- `golang`
- `security-tools`
- `developer-tools`

Do not add `compliance` unless the repo is deliberately being positioned for
that buyer signal.

## Badges

Keep only badges that reflect live, verifiable repo signals:

- CI
- Go Reference
- Go Report Card
- License
- Release tag, only after a release tag exists
- GitHub Action, only after the action has a real tag or marketplace surface

Do not add coverage, downloads, Homebrew, npm, Docker, SOC2, or
production-ready badges unless those signals are true and maintained.

## Social Preview

The repo-owned social preview source is:

```text
docs/assets/social-preview.svg
```

Suggested text:

```text
Fulcrum Boundary
The action boundary for routed agent tools
Before an agent touches a dangerous tool, Boundary decides.
```

GitHub repository social preview settings may reject SVG uploads. If that
happens, export a PNG manually from this SVG and upload the PNG through GitHub
settings. Do not add a README image reference until the asset is actually
served somewhere stable.

## Public Demo Asset

Primary demo asset: `docs/assets/github-lethal-trifecta-demo.gif`, a real run
of `boundary demo github-lethal-trifecta` recorded from the flagship GitHub
write-after-taint fixture. The recording dwells on the verdict block: `actual
action: DENY`, `upstream_called=false`, and the decision hash.

Walkthrough fallback: `docs/assets/boundary-demo-walkthrough.svg`, a static
deny-before-upstream walkthrough for no-JS and mobile surfaces. It is a stylized
diagram, not a literal capture.

Use "demo recording" and "decision record" as the public framing. Do not lead
public surfaces with "Terminal receipt." The recording source is committed
beside the rendered assets so the GIF and MP4 can be recreated by maintainers.

Keep the asset honest: no credentials, no network, no live GitHub mutation, and
no production-readiness language beyond the claims ledger.
