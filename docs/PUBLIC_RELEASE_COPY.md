# Public Release Copy

Date: 2026-05-28

This document gives reusable public copy for the Boundary public release surface.
It follows the language system in `docs/LANGUAGE_SYSTEM.md`, the copy rules in
`docs/COPY_RULES.md`, the claims ledger in `docs/CLAIMS_LEDGER.md`, and the
adapter maturity matrix in `docs/ADAPTER_READINESS_MATRIX.md`.

## Short Copy

Fulcrum Boundary is the action boundary for routed agent tools. See what your AI
tools can do, block what they should not, and record every verdict before
privileged execution. MCP is the first production route; the Command Boundary
lane brings the same pre-execution decision to routed command paths as a
delivered preview.

## Medium Copy

Fulcrum Boundary sits between AI agent intent and privileged tools. It discovers
MCP tool paths, renders risk paths, generates starter policies, red-teams risky
fixture flows, and blocks governed actions before the underlying tool executes.
Every governed verdict emits a decision record so operators can see what was
allowed, denied, warned, escalated, or sent for approval.

## Secure GitHub Preview Copy

Secure GitHub is Boundary's flagship preview Secure MCP profile. The fixture
demo models a coding agent reading untrusted GitHub content and then attempting
a private-repo mutation. Boundary tracks the tainted context, denies the tested
write-after-taint path before GitHub is touched, and records the verdict.

Secure GitHub also includes an opt-in live conformance harness for
operator-owned GitHub App credentials. The denied-write live conformance path
records that a protected write-after-taint action was denied before any GitHub
mutation client call. Secure GitHub remains preview until deployment bypass
evidence and broader live coverage are recorded.

## MCP Firewall Copy

Boundary Firewall inventories local MCP client configs, classifies reachable
tools, renders risk paths, generates starter policies, installs reversible
routes, verifies descriptor locks, runs fixture redteam packs, and renders a
local-only dashboard over the resulting artifacts.

Boundary governs routed tools. Direct access to the same tool is a bypass path
unless deployment topology blocks it.

## What The Demo Proves

| Area | Claim-safe wording |
|---|---|
| Inventory | Boundary can read fixture MCP client config and identify reachable tools. |
| Risk graph | Boundary can connect an untrusted source to a privileged mutation path. |
| Starter policy | Boundary can generate policies that verify locally and are ready for operator review. |
| Secure GitHub | Boundary can deny the tested private-repo write-after-taint fixture before upstream execution. |
| Decision record | Boundary records the action, reason, matched rule, and request context for the governed route. |

## What The Demo Does Not Prove

| Area | Claim-safe limit |
|---|---|
| Prompt injection | The demo does not claim universal prompt-injection defense. |
| GitHub production status | The demo does not claim production GitHub security. |
| Direct access | The demo does not protect tools that bypass Boundary. |
| Policy completeness | Generated policies are starter policies for operator review. |
| Hosted monitoring | The dashboard is local-only unless a hosted service is actually implemented. |

## Approved Phrases

- The action boundary for routed agent tools.
- Before an AI agent touches a real system, Boundary decides whether that action is allowed.
- See what your AI tools can do. Block what they should not.
- Boundary governs routed tools before privileged execution.
- Inventory shows what exists. Boundary decides what can act.
- Secure GitHub is preview until deployment bypass evidence and broader live
  coverage exist.

## Forbidden Or Controlled Public Copy

These phrases are forbidden as public capability claims:

- Do not say Boundary prevents all prompt injection.
- Do not say Boundary provides universal agent safety.
- Do not say Secure GitHub fully secures GitHub.
- Do not say Secure GitHub is production GitHub security.
- Do not say all adapters are production.
- Do not say generated policies are complete production policy.
- Do not say Boundary protects direct tool calls that do not route through Boundary.
