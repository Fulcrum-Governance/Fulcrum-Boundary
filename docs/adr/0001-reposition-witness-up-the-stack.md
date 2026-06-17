---
status: Accepted
date: 2026-06-16
deciders: Tony Diefenbach
consulted: Field Loop cycle 1 (strategy review)
---

# 1. Reposition under a contested, formalizing agent-governance field

> **Decision-record numbering.** This is the first Architecture Decision Record
> in this repository. No `docs/adr/` home existed before this file, so `0001` is
> the correct and unused number. ADRs here are MADR-style and are distinct from
> the runtime *decision-record* feature documented in
> `docs/DECISION_RECORDS.md`, which is a product capability, not an architecture
> log.
>
> **Scope of this ADR.** Positioning and technical direction only. It records no
> commercial, deal, or transaction strategy. Any such material lives in a
> separate **local, uncommitted** strategy record (referenced by path below),
> not on any public or committed surface.

## Status

Accepted — 2026-06-16.

## Context

Through 2026 the ground under this project shifted in two ways that, taken
together, force a repositioning.

**1. The pre-execution boundary primitive became a commodity platform feature.**
When Boundary started, "decide allow/deny/escalate *before* a routed tool call
runs, and record a hash-verifiable decision" was a differentiated capability. It
no longer is on its own. Several large platforms now ship an equivalent
pre-execution gate for agent tool calls as a built-in feature of their broader
stack. The mechanism Boundary leads with today — a routed action boundary with a
canonical, verifiable decision record — is now table stakes that a platform buyer
can obtain inside infrastructure they already run. Continuing to lead on "the
boundary" alone is positioning into a commoditized layer.

**2. The formal-methods lane around agent governance gained real traction.**
What used to read as an academic flourish — machine-checked guarantees about
agent behavior — has started to find paying interest and named adopters in the
broader market. The signal is research-grade, not fully reproduced (see the
honesty note below), but the direction is unambiguous: rigor is moving from
"nobody will pay for proofs" toward "proofs are becoming a purchasable
differentiator." That makes the formal lane both more crowded and more valuable.

These two shifts squeeze the obvious middle. The defensible position is **not**
the boundary primitive and **not** "we have some proofs"; both are now contested
or commoditized in isolation. The defensible position is the precise place where
this project already sits and others do not, stated as an **exact conjunction**:

- **(A)** a machine-checked, game-theoretic trust equilibrium — a Beta-based
  trust model with a Nash/price-of-anarchy result and a *proven-terminating*
  circuit breaker — that is **coupled to live enforcement**, not just modeled on
  paper; and
- **(B)** a **per-decision certifying witness** for each governed action, checked
  by a formally verified checker against two specific invariants: the budget
  invariant `B_prev >= C_req` and the privilege invariant `P_req ⊆ P_avail`.

Neither half is the differentiator. The differentiator is the **conjunction of
both, wired to enforcement.** The Lean correspondence for the equilibrium and the
invariants already exists in `Fulcrum-Proofs` and is documented at
`docs/PROOF_BOUNDARY.md`. That correspondence is **design-level** today: it is
**not** a claim that the Go implementation was mechanically verified, and
Boundary itself does **not** emit `proved` decisions. Boundary is the **open,
record-carrying proof-of-life** for that conjunction — the public, runnable
artifact that shows the enforced boundary and the certifying record working end
to end, while the deepest formal guarantees live in the proof and trust
repositories.

**Where the surface is heading.** The governance surface is moving *up the
stack*. Today a boundary inspects an individual routed tool call. But agents
increasingly self-decompose: an optimized, compiled language-model program (for
example a DSPy / GEPA / MIPRO-style pipeline) can do consequential work inside a
sub-call, a tool loop, or a REPL that a call-level boundary never sees. The
forward target is to lift the certifying witness from the single tool call up to
the **compiled LM-program / reasoning-trace level** — to certify that an
optimized program still preserves its *declared* invariants (budget, privilege,
and the trust contract) even after compilation and optimization. This work is
**not built** and is recorded here only as direction (see Decision). It also
names an open **existential risk** that must be red-teamed rather than assumed
away: self-decomposition can move real effect into places a naive, call-level
boundary does not observe.

## Decision

This ADR records a **positioning and technical** decision. It changes how the
project describes itself and where it points its hardest engineering. It does not
change any shipped capability or claim; existing release-truth and the claims
ledger remain the source of truth for what is delivered.

1. **Lead with the exact conjunction (A ∧ B), coupled to enforcement.**
   Positioning leads with the *conjunction* of the machine-checked trust
   equilibrium and the per-decision certifying witness on the budget and
   privilege invariants, wired to live enforcement. We do **not** lead with
   either half alone, and we attach **no** superlative or category-ownership
   phrasing to it — the conjunction is stated as a described capability, with its
   current design-level limits intact, not as a ranking claim.

2. **Demote "we have Lean proofs" and "we sign receipts" to table stakes.** Lean
   correspondence and Ed25519-signed decision records remain real, documented,
   and load-bearing — but they are *supporting evidence under the conjunction*,
   not the headline, because each in isolation is now reproducible or
   commoditized elsewhere.

3. **Stop positioning Boundary as a gateway or firewall.** The product headline
   must not present Boundary as a generic gateway product or as a firewall. (The
   controlled phrase "SQL firewall" is named here only to forbid it as a
   headline; it does **not** describe this product.) Boundary is positioned as
   the open, enforced proof-of-life for the conjunction above — not as another
   entrant in the commoditized gate layer.

4. **Set "witness up the stack" as the forward technical direction — tagged
   `[INTENT]`.** Lifting the certifying witness from the single tool call to the
   compiled-LM-program / reasoning-trace level is adopted as the primary forward
   engineering bet. It is recorded as **`[INTENT]` / conjectured**: it is **not**
   built, **not** delivered, and must not appear as a present capability in any
   public surface, the README, or the claims ledger until it ships with tests and
   a ledger entry. The self-decomposition existential risk is adopted as a
   first-class red-team objective, not a footnote.

5. **Commit to standards legibility.** We commit to making Boundary *legible* to
   the emerging agent-governance and assurance standards landscape — mapping our
   decision record, witness, and invariants to recognized references so an
   evaluator can place us. This is a legibility and mapping commitment; it is
   **not** a blanket conformance claim. Any conformance statement remains scoped
   to the decision record (whose canonicalization is RFC 8785 / JCS), and it is
   **not** a claim that Boundary as a whole is standards-conformant. Known
   reference liabilities tracked in the internal record must be corrected before
   any external citation rests on them.

## Consequences

### Positive

- The lead claim sits where the project is genuinely hard to copy — the enforced
  conjunction (A ∧ B) — instead of a commoditized primitive, so the positioning
  survives the platformization of the boundary layer.
- Demoting proofs and receipts to supporting evidence makes the public story more
  honest *and* harder to dismiss: each piece is checkable, and the claim rests on
  their combination rather than on any single contested part.
- Naming "witness up the stack" as `[INTENT]` gives engineering a clear, dated
  forward target and a named existential risk to attack, without leaking it into
  delivered-capability language.
- A standards-legibility commitment improves how third parties can evaluate the
  work, while the record-scoped framing keeps the claim defensible under the
  repository's language gate.

### Negative

- The conjunction is a more demanding story to tell than "we block bad tool
  calls." It requires explaining two coupled mechanisms and their enforcement
  link, which raises the bar on documentation and demos.
- Pointing the hardest engineering at the compiled-LM-program level commits
  scarce effort to an `[INTENT]` direction that is unproven and carries a real
  existential risk; if self-decomposition cannot be witnessed soundly, the bet
  must be revisited.
- Walking away from "gateway/firewall" framing forgoes the easy familiarity of a
  known category label and asks audiences to learn a less familiar position.

### Neutral

- No shipped capability, test, or ledger claim changes as a result of this ADR;
  it governs language and direction. Release-truth, `LIMITATIONS.md`, and the
  claims ledger remain authoritative for what is delivered.
- The Lean correspondence stays exactly as documented in `docs/PROOF_BOUNDARY.md`
  — design-level, with Boundary still not emitting `proved` decisions. This ADR
  reframes its *role* in the narrative, not its technical status.
- Competitive intelligence underlying the Context section is **research-grade**
  (gathered 2026-06-16, not all independently reproduced). It informs positioning
  but is not asserted here as settled fact.

## References

Strategy and competitive detail are **not reproduced or described** here. A
single **local, uncommitted** strategy record holds that material and is kept
entirely outside this repository — never committed, and not referenced by path on
any public surface; its contents are not summarized on
this or any public surface.

Public anchors this ADR is consistent with (delivered truth lives in these, not
in this ADR):

- `docs/PROOF_BOUNDARY.md` — the Lean correspondence and the standing rule that
  Boundary does not emit `proved` decisions.
- `docs/TRUST_INTEGRATION.md` — the Stage-1 trust backend (Beta evaluator and the
  `fulcrum-trust` path).
- `docs/BOUNDARY_ROADMAP.md` — shipped-on-main versus planned, by phase.
- `claims/boundary_claims.yaml` and `docs/CLAIMS_LEDGER.md` — the authoritative
  claims ledger; any `[INTENT]` item above must not appear as delivered until it
  is entered here with tests and docs.
- `LIMITATIONS.md` — the standing limitations, including the routed-only boundary
  caveat that constrains every claim in this repository.