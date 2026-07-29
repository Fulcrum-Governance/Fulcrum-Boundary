# Decisions — where they live

**Cross-repo architecture and strategy decisions for the Fulcrum four-repo
system live in one canonical series: fulcrum-io
[`.claude/decisions/`](https://github.com/Fulcrum-Governance/Fulcrum-IO/tree/main/.claude/decisions)
— see its `INDEX.md` for the full numbered table (ADR-001…).** This repo's
local decision surfaces are frozen.

## Freeze rule (FUL-353, program parent FUL-266)

- Do **not** create new ADR files in this repo. New decision records —
  including decisions that primarily concern Fulcrum-Boundary — land in the
  fulcrum-io canonical series and reference this repo from there.
- `docs/adr/0001-reposition-witness-up-the-stack.md` and the
  `docs/DECISION_RECORDS.md` register are **frozen as historical records**:
  read them, cite them, never grow or renumber them. Nothing here is ever
  deleted — archive-only is a program hard rule.

## Explicit non-goal

Reorganizing `docs/DECISION_RECORDS.md` (18.8K monolith) or this repo's
root-level docs set is **out of scope** for the freeze (FUL-353 AC3). If that
reorganization ever happens it is its own program with its own ledger — this
note only stops the drift from growing.

## Why

The 2026-05-17 consolidation established a single numbered series so that
"why did you decide X" has exactly one answer path. Per-repo ADR dirs drifted,
renumbered, and contradicted each other; the canonical series plus this freeze
note is the repair. Numbering authority is the fulcrum-io filesystem
(`ls .claude/decisions/ADR-*.md | sort | tail -1`) — never quoted from prose.
