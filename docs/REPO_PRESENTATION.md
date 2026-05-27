# Repository Presentation

This page records the intended GitHub repository presentation for Fulcrum
Boundary. It should stay claim-safe, concrete, and free of fake adoption or
release signals.

## Repository Description

Use this GitHub repository description:

> The action boundary for MCP-native agents. See what your AI tools can do; block what they should not.

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
The action boundary for MCP-native agents
See what your AI tools can do. Block what they should not.
```

GitHub repository social preview settings may reject SVG uploads. If that
happens, export a PNG manually from this SVG and upload the PNG through GitHub
settings. Do not add a README image reference until the asset is actually
served somewhere stable.

## First Screenshot Or GIF Plan

The first moving visual should be an operator-grade terminal recording, not a
marketing animation:

1. Run `boundary selftest`.
2. Run `boundary demo github-lethal-trifecta`.
3. Show `actual action: DENY`, `reason: lethal_trifecta_detected`, and
   `upstream_called=false`.
4. Optionally show `boundary graph --format mermaid` rendering the risk path.
5. End on the decision-record line and the fixture/live boundary statement.

Keep the capture honest: no credentials, no network, no live GitHub mutation,
and no production-readiness language beyond the claims ledger.
