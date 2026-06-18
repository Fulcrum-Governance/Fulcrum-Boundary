# Boundary Copy Rules

This document turns the language system into rules for README copy, release
notes, demo scripts, screenshots, and external-facing docs.

## Main Rule

Lead with the concrete moment of danger:

```text
Your agent is about to touch a real system. Boundary decides whether that action
is allowed before the tool executes.
```

## Required Pattern

Use this order for user-facing explanations:

1. Name the dangerous action.
2. Name the system the agent is about to touch.
3. Name the Boundary decision.
4. Name the record that proves the decision.

Example:

```text
The agent read a public GitHub issue and tried to create a pull request in a
private repo. Boundary denied the repo write before GitHub was touched and
recorded the verdict.
```

## Preferred Words

Use:

- action boundary
- proposed action
- privileged tool
- governed route
- bypass path
- verdict
- decision record
- receipt-grade record
- tainted context
- risk path
- source
- sink
- mutation

Use concrete verbs:

- read
- write
- send
- delete
- deploy
- refund
- merge
- mutate
- exfiltrate

## Words To Handle Carefully

| Word | Rule |
|---|---|
| governance | Category language only. Do not make it the hook. |
| platform | Avoid unless naming integration scope that is already evidenced. |
| proof | Use for proof-correspondence and receipt verification only. |
| secure | Use only with a named boundary and evidence. |
| production | Use only when the readiness matrix supports it. |
| prevents | Avoid absolute prevention claims. Prefer "blocks this governed route when..." |

## Forbidden Or Controlled Phrases

These phrases must not appear as public capability claims. They may appear in
claim-control, language-control, or historical docs that explicitly say they are
forbidden or false.

| Phrase | Replacement |
|---|---|
| AI governance platform | action boundary for agent actions |
| relabel `"coupled to enforcement"` | say "wired witness (budget/static-privilege) + circuit-transition (termination) + machine-checked equilibrium analysis (Nash/PoA design correspondence)" |
| SQL firewall | Postgres AST guard for routed requests |
| prevents all prompt injection | blocks tested write-after-taint fixture attacks before execution |
| prevents all SQL injection | classifies and denies tested destructive SQL patterns in governed routes |
| universal agent safety | pre-execution control for governed tool routes |
| proved decision | structured decision record or proof-correspondence boundary |
| secure sandbox | named and tested execution boundary |
| all adapters production | one production MCP adapter plus preview adapters tracked by readiness |
| no other tool does this | Boundary is designed to detect this specific pattern |
| fully secures GitHub | governs supported GitHub actions routed through Boundary |
| production GitHub security | preview Secure GitHub profile until live conformance evidence exists |
| detects every malicious issue | detects configured taint sources and tested write-after-taint paths |
| AI gateway (as the product headline) | action boundary for routed agent tools |
| category of one | names a specific gap; describe the exact capability and its evidence instead |
| first formally verified | a formally verified checker validates stated invariants (only where that checker exists and is named) |
| only formally verified | a formally verified checker validates stated invariants (only where that checker exists and is named) |
| empty intersection of rigor and traction | describe the traction and the rigor work separately, each scoped to evidence |
| provably safe | denies tested patterns on governed routes; emits verifiable decision records (never "safe" unqualified) |

Note: `"coupled to enforcement as a runtime certificate"` stays forbidden.

Notes on the additions above:

- The boundary / decide-before-execution primitive is commoditized in
  general-purpose agent platforms and cloud gateways. Do not headline Boundary as
  "an AI gateway" or imply the primitive is the differentiator; lead with the
  operator-verifiable decision record on a governed route.
- "Category of one", "first/only formally verified", and "empty intersection of
  rigor and traction" are unbounded standing claims about the whole field. They
  cannot be checked against this repo's evidence and do not belong in public
  copy. State the specific capability and where it is proven.
- "Provably safe" (unqualified) overstates a routed, fixture-scoped guarantee. A
  separate, formally verified checker validates stated invariants only where that
  checker exists — and it is not part of this standalone repo.

## Adapter Maturity

Use the exact maturity posture from
[`docs/ADAPTER_READINESS_MATRIX.md`](./ADAPTER_READINESS_MATRIX.md):

- MCP is production when deployed through the documented protected topology.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, and Secure GitHub are
  preview unless their readiness gates are satisfied.
- Preview adapters can be useful. Preview does not mean production.

## Claim Synchronization

Before changing public copy, check:

1. [`claims/boundary_claims.yaml`](../claims/boundary_claims.yaml)
2. [`docs/CLAIMS_LEDGER.md`](./CLAIMS_LEDGER.md)
3. [`docs/ADAPTER_READINESS_MATRIX.md`](./ADAPTER_READINESS_MATRIX.md)
4. The tests that prove the behavior being claimed.

If a claim is partial, include the gap. If it is planned, present it as roadmap.
If it is false, use it only as a rejected claim.

## Good Examples

```text
Boundary evaluates the proposed action before it reaches the privileged tool.
```

```text
The MCP Safety Gateway blocks a destructive Postgres request when the agent's
route is forced through Boundary.
```

```text
Secure GitHub MCP is preview until deployment bypass evidence and broader live
coverage exist.
```

## Bad Examples

```text
Boundary is an AI governance platform for universal agent safety.
```

```text
Boundary prevents all prompt injection.
```

```text
Secure GitHub fully secures GitHub for coding agents.
```
