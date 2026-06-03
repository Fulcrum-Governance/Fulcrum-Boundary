# Boundary Phase 0B Demo Hierarchy Lane

Date: 2026-06-03

## Summary

This Phase 0B follow-up aligns the public demo surfaces around Boundary's
two-lane proof spine:

- Lane 1: MCP, the first production route, via
  `boundary demo github-lethal-trifecta`.
- Lane 2: Command Boundary, a delivered preview routed-only surface, via
  `boundary demo command-secret-exfil`.

The lane is presentation-only. It does not add a governed action surface, change
policy behavior, change verdicts, promote any preview surface to production, or
change release tags.

## Authority

- Product spine: `README.md` and `docs/DEMOS.md`.
- Roadmap: `docs/BOUNDARY_ROADMAP.md` Phase 0B.
- MCP lane detail: `docs/DEMO_GITHUB_LETHAL_TRIFECTA.md`.
- Command lane detail: `docs/command-boundary/DEMO.md`.
- Public claims: `docs/CLAIMS_LEDGER.md` and `docs/RELEASE_TRUTH_PUBLIC.md`.

## Delivered In This Slice

- Add a source demo index at `docs/DEMOS.md`.
- Point the README top link at the two-lane demo index instead of the MCP-only
  lane detail.
- Make the docs-site demo landing page present both proof lanes equally.
- Make the Command Boundary demo page lead with `boundary demo
  command-secret-exfil` as the user-facing Lane 2 demo.
- Keep all claims fixture-only, routed-only, and release-tag-aware.

## Non-Goals

- No release tag.
- No new ledger claim.
- No new adapter readiness entry.
- No production deployment proof.
- No route-bypass closure claim.

## Verification

Required gates before merge:

```bash
mkdocs build -s
go test ./claims/... -count=1
git diff --check
```
