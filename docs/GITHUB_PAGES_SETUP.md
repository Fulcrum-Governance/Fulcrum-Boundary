# GitHub Pages Setup

Date: 2026-05-27

Fulcrum Boundary's docs site is built by the `Docs` workflow.

## Repository Setting

Use GitHub repository settings:

- Settings -> Pages
- Source: GitHub Actions

## Workflow

The Pages deployment workflow is:

```text
.github/workflows/docs.yml
```

The site builds from:

```text
mkdocs.yml
docs-site/
```

Local verification:

```bash
./scripts/docs-build.sh
```

## Publication Rule

Do not publish or advertise a docs URL until the `Docs` workflow deploys
successfully from `main`.

## Claim Boundary

The docs site is a static publication surface. It is not hosted monitoring,
runtime protection, telemetry, or a managed Boundary service.
