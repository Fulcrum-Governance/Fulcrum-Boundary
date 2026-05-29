# Boundary Public Surface Reset Design

Date: 2026-05-29

Status: approved for implementation planning

Branch: `codex/2026-05-29-public-surface-reset`

## Purpose

This lane turns the supplied stabilization packet into an execution-ready design
for Boundary public-surface polish. The work is deliberately not product
expansion. It cleans the public presentation around the existing v0.6.1 truth:
Boundary is the downloadable action boundary for routed AI-agent tool paths.

Boundary decides whether a proposed privileged action is allowed before
execution, records the verdict, and governs only routes forced through
Boundary. The public surface must make that concrete without implying broader
coverage than the repo has proved.

## Authorities

- Supplied stabilization packet for phase scope and acceptance criteria.
- `docs/RELEASE_TRUTH_PUBLIC.md` for current public release truth.
- `docs/CLAIMS_LEDGER.md` and `claims/boundary_claims.yaml` for claim status.
- `docs/RELEASE_TRUTH_V061.md` for v0.6.1 utility-train limits.
- `docs/COPY_RULES.md`, `docs/LANGUAGE_SYSTEM.md`, and `docs/LEXICON.md` for
  public wording.
- Current repo implementation and verification gates for what can be claimed.

The vision brief is directional product context only. It is not the source of
current release facts when it conflicts with v0.6.1 repo truth.

## Scope

In scope:

- Public contact and naming hygiene.
- Public README hierarchy and claim alignment.
- Demo hero replacement with a static walkthrough asset.
- Demotion of the current terminal recording to receipt evidence.
- README and docs-site diagram cleanup.
- Docs-site homepage, demo page, quickstart, navigation, and link hygiene.
- Verification receipts for claims, docs build, release check, and diff hygiene.

Out of scope for this branch:

- New runtime features.
- Secure GitHub production promotion.
- Command Boundary production promotion.
- Edit Boundary production promotion.
- Hosted dashboard or team control-plane work.
- New live credentials, secret rotation, or package publishing.

Runtime behavior should not change unless a docs or asset validation test must
be adjusted to reflect the public-surface changes.

## Current Truth To Preserve

- Active public release surface is v0.6.1.
- MCP is the production adapter path.
- Secure GitHub remains preview.
- Command Boundary is delivered preview and routed-path-only.
- Edit Boundary is delivered preview and routed-edit-envelope-only.
- First-run demos are fixture-only: no credentials, no live GitHub calls, and
  no real mutations.
- Generated policies are starter policies for operator review.
- Dashboard output is local-only artifact visibility, not hosted monitoring.
- Boundary governs routed tools only. Direct bypass paths remain outside
  Boundary unless deployment topology removes them.
- Evidence bundles and local diagnostics do not prove production safety.

## Design Overview

The reset has six implementation phases. Each phase narrows the public surface
before the next one adds presentation polish, so the final repo tells one
current, claim-safe story:

1. Public-surface hygiene removes stale contacts, legacy current-product naming,
   and private planning residue from public files.
2. Demo reset replaces the primary animated terminal asset with a readable
   static walkthrough.
3. README repair compresses the first screen into product identity, one-minute
   proof, and claim-safe limits.
4. Diagram replacement removes dated architecture blocks and keeps Boundary, not
   routing, at the center.
5. Docs-site polish mirrors the README hierarchy and makes the clickthrough
   path coherent.
6. Verification records exact commands, results, limitations preserved, and any
   exceptions.

## Components

### Public Hygiene

Primary files:

- `SECURITY.md`
- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `CHANGELOG.md`
- `README.md`
- `docs/`
- `docs-site/`

Design:

- Use `agent@fulcrumlayer.io` as the only Fulcrum email allowed in public repo
  content.
- Replace legacy current-product naming with Boundary naming.
- Keep historical naming only when the sentence is explicitly historical and
  necessary; otherwise remove it.
- Keep the security policy limited and practical. Do not invent a bounty, SLA,
  incident process, or support channel that the repo does not define.
- Update contributor checks to the current gates:
  - `make release-check`
  - `go test ./claims/... -count=1`
  - `go test ./... -short -count=1 -timeout 5m`
  - `make docs-build`
- Replace release-note language about internal demo planning with product-facing
  fixture-safe demo language.

### Demo Walkthrough Asset

Primary file:

- `docs/assets/boundary-demo-walkthrough.svg`

Design:

- The SVG is the primary README and docs-site demo visual.
- It must read as a still image at GitHub README width and mobile width.
- It shows four frames:
  - poisoned GitHub issue enters agent context
  - agent attempts private-repo mutation
  - Boundary evaluates before upstream execution
  - Boundary denies and records the verdict
- It uses claim-safe labels:
  - fixture-only
  - no credentials
  - no live GitHub calls
  - no real mutations
  - DENY before upstream mutation
  - decision record emitted
  - routed paths only
- It must not visually imply live GitHub mutation, production Secure GitHub, or
  global command/edit control.

The existing GIF and MP4 may remain only as terminal receipt evidence. Any page
that keeps them must label them as receipt evidence, not the main demo.

### README Hierarchy

Primary file:

- `README.md`

Design:

- Keep `Fulcrum Boundary` as the page title.
- Keep the hook: `The action boundary for MCP-native agents.`
- Add a concrete danger sentence near the top: the agent is about to touch a
  real system; Boundary decides before the tool executes.
- Keep real badges only.
- Put primary links near the top:
  - Quickstart
  - Demo
  - Docs
  - Claims
  - Release Truth
  - Security
- Keep the one-minute install path:
  - Go 1.25+
  - install at v0.6.1
  - `boundary selftest`
  - `boundary demo github-lethal-trifecta`
  - fixture-only caveat
- Move or remove bulky implementation examples from the top.
- Remove the large ASCII architecture block.
- Keep compact tables for:
  - what the demo proves
  - what it does not prove
  - product surfaces and limits
- Link to deeper docs instead of reproducing every example inline.

### Diagrams

Primary files:

- `README.md`
- `docs/diagrams/action-boundary.mmd`
- `docs/diagrams/github-write-after-taint.mmd`
- `docs/diagrams/surface-status.mmd`

Design:

- README contains one small Mermaid diagram with no more than six nodes.
- Boundary is the center of the diagram.
- The caveat immediately below the diagram says Boundary governs actions only
  when the route is forced through Boundary.
- Deeper diagrams live as source files under `docs/diagrams/`.
- MkDocs already supports Mermaid, so generated SVGs are only needed if the docs
  build or rendered pages require them.
- Terminal screenshots are not architecture diagrams.

### Docs Site

Primary files:

- `docs-site/index.md`
- `docs-site/demo.md`
- `docs-site/quickstart.md`
- `mkdocs.yml`

Design:

- Homepage gets a clear hero, one-minute quickstart, walkthrough visual, release
  truth cards, and links to Demo, Quickstart, Concepts, Claims, and Release
  Utilities.
- Demo page uses the walkthrough as the primary asset.
- Demo page includes:
  - exact command
  - expected success signal
  - what it proves
  - what it does not prove
  - terminal receipt section, if the existing recording is retained
  - link to the canonical source demo doc
- Quickstart includes expected output summary plus fixture-only and routed-only
  caveats.
- Clickthrough audit covers README links, docs-site navigation, GitHub Action
  docs, release-truth docs, claims docs, security/conduct/contributing links,
  and asset paths.

## Data Flow

This lane changes public artifacts, not runtime request flow.

Implementation data flow:

1. Read stabilization packet and repo authority docs.
2. Scan public files for stale contact, naming, internal residue, and overclaim
   patterns.
3. Rewrite public text so every capability statement maps to the claims ledger
   or release truth.
4. Add walkthrough asset and update README/docs-site references.
5. Add diagram source files and simplify README diagram presentation.
6. Build docs and run release/claim gates.
7. Record exact verification results in the final report.

Product story data flow shown by the public surface:

1. Untrusted GitHub issue enters agent context.
2. Agent proposes a private-repo mutation.
3. Boundary evaluates before upstream execution.
4. Boundary denies the routed fixture action.
5. GitHub is not called.
6. A decision record is emitted.

## Error Handling And Edge Cases

- If a grep check finds claim-control or limitation text, classify the hit
  instead of deleting useful caveat language blindly.
- If a public email occurrence is not `agent@fulcrumlayer.io`, replace it or
  remove it.
- If docs-site rendering cannot use an asset path shared with README, copy the
  asset to the docs-site asset tree and keep both references consistent.
- If an SVG is readable in source but not in README/docs output, revise layout
  dimensions, font sizes, and label density before proceeding.
- If a link check fails because a target doc does not exist, either correct the
  link or create the minimal claim-safe target when the spec requires it.
- If a verification command fails for unrelated environment reasons, preserve
  the failure output and rerun after fixing local setup only when it is safe.

## Testing And Verification

Required gates:

- Contact grep: no non-approved Fulcrum contact aliases.
- Public email grep: only `agent@fulcrumlayer.io` appears.
- Internal residue grep: no prohibited private planning/session terms in public
  surfaces.
- Current-product naming grep: no current-product legacy naming misuse in
  README, SECURITY, CONTRIBUTING, CODE_OF_CONDUCT, docs, or docs-site.
- Overclaim grep: no unsupported capability claims in README, docs, or
  docs-site except explicit limitation or claim-control context.
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`
- `make docs-build`
- `make release-check`
- `git diff --check`

The final implementation report must include:

- files changed
- claims changed
- public-surface risks fixed
- known limitations preserved
- exact commands run and results
- failures or skipped checks

## Sequencing

1. Hygiene first. Do not polish the hero around stale contact or naming facts.
2. Demo asset second. The walkthrough becomes the primary explanation before
   README/docs-site hierarchy work.
3. README and diagrams third. Keep the README compact and route detail to docs.
4. Docs-site fourth. Mirror the README story and verify navigation.
5. Verification last. Run gates only after public files and assets settle.

## Open Decisions

None. The supplied stabilization packet is treated as the active scope. Future
product arcs named in the packet remain ordered follow-on lanes after this
public reset lands.
