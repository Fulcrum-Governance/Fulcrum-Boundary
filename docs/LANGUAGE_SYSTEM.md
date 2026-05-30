# Fulcrum Boundary Language System

This document defines how Boundary talks about itself in public docs, release
notes, demos, and examples. It is a language contract, not a marketing layer.
Every phrase here must stay inside the evidence tracked by
[`docs/CLAIMS_LEDGER.md`](./CLAIMS_LEDGER.md) and
[`claims/boundary_claims.yaml`](../claims/boundary_claims.yaml).

## Core Sentence

Before an AI agent touches a real system, Boundary decides whether that action is
allowed.

## Approved Variants

| Context | Sentence |
|---|---|
| Compressed | Before agents act, Boundary decides. |
| Boundary | Boundary sits between agent intent and privileged tools. |
| Developer | See what your AI tools can do. Block what they should not. |
| MCP Firewall | Find and block dangerous MCP tool paths before your agent executes them. |
| Secure GitHub | Stop coding agents from turning untrusted GitHub content into private-repo mutations. |

## Product Feeling

Boundary should sound like an action boundary for agent behavior. It should not
sound like generic compliance, observability, AI safety, or dashboard software.

Use product language that names:

1. The dangerous action.
2. The real system the agent is about to touch.
3. The Boundary verdict.
4. The decision record that proves what happened.

Example:

```text
Your agent read a public GitHub issue and tried to write to a private repo.
Boundary denied the GitHub mutation before the API call executed and recorded
the verdict.
```

## Mental Model

```text
Agent intent
  |
  v
Proposed action
  |
  v
Boundary
  |
  v
Verdict: allow | deny | warn | escalate | require_approval
  |
  v
Execution only if allowed
  |
  v
Decision record
```

The router is a deployment pattern. The boundary is the product.

## Language Rules

- Start with the dangerous action, not the architecture.
- Say what system the agent is about to touch.
- Say what Boundary decides.
- Use concrete verbs: read, write, send, delete, deploy, refund, merge,
  mutate, exfiltrate.
- Use "governance" as category language only. Do not make it the product hook.
- Do not lead with formal verification. Proof is a credibility layer after the
  action boundary is understood.
- MCP is the first wedge, not the whole identity.
- Never say Boundary protects a tool unless the route passes through Boundary.
- Keep preview and production adapter language aligned with
  [`docs/ADAPTER_READINESS_MATRIX.md`](./ADAPTER_READINESS_MATRIX.md).
- Keep receipt-grade language separate from basic decision-record language.

## Preferred Public Frame

```text
Fulcrum Boundary is the action boundary for MCP-native agents. It discovers
dangerous MCP tool paths, generates policies, red-teams risky flows, and blocks
unsafe actions before privileged tools execute.
```

Use this only as each listed capability becomes supported by repo evidence. If a
capability is still planned or partial, caveat it with the current claim status.

## What Proof Means Here

Boundary runtime docs may describe tested conformance, decision records,
receipt-grade records, and proof-correspondence boundaries. They must not imply
that every runtime decision is mechanically proved or that Boundary emits proved
decisions.

Use:

```text
Boundary emits structured decision records and, where configured, receipt-grade
records with request, policy bundle, and decision hashes.
```

Avoid:

```text
Boundary emits proved decisions for every action.
```

## What Secure GitHub Means Here

Secure GitHub MCP is the flagship governed-tool profile for the next release
train. Its first proof path is fixture-based:

1. Untrusted GitHub content enters the agent context.
2. The agent attempts a private-repo mutation.
3. Boundary denies the write before GitHub is touched.
4. A decision record captures the verdict.

Do not claim production GitHub security, universal prompt-injection prevention,
or live GitHub App conformance until the corresponding evidence exists.
