# Command Boundary Preview Claims

Command Boundary is a planned preview surface. This document controls the
language that may be used before runtime implementation lands.

## Planned Claim

`BND-CLAIM-CMD-001`:

> Boundary defines a preview Command Boundary design for project-local command
> governance.

Status: `planned`

This means the repo may describe the design and roadmap. It must not state that
Command Boundary currently governs commands until implementation, tests, and
release truth reconciliation land.

## Approved Copy

The following language is approved for design and roadmap docs:

- Boundary can govern project-local command paths when commands route through
  `boundary command run`, `boundary shell`, or project-local shims.
- Command Boundary is a preview design for project-local command governance.
- Command Boundary governs routed command paths only.
- Direct shell access is outside Boundary unless the environment routes commands
  through the wrapper or project-local shims.

## Forbidden Copy

Do not use these statements as product claims:

- Boundary controls your shell.
- Boundary protects all CLI activity.
- Boundary prevents every overeager agent action.
- Boundary protects direct shell access.
- Boundary provides production command governance.
- Boundary provides shell sandboxing.
- Boundary controls CI jobs by default.
- Boundary controls remote SSH by default.

## Claim Advancement Requirements

The planned claim can move beyond `planned` only after implementation branches
add evidence for the routed command path.

Minimum evidence for a partial preview claim:

- `boundary command classify` classifies commands without executing them;
- `boundary command run` denies blocked commands before execution;
- allowed commands execute exactly once;
- command decision records are emitted;
- secret-looking arguments are redacted;
- project-local shims do not mutate global shell startup files;
- docs state the bypass model clearly.

Minimum evidence for a delivered preview claim:

- tests cover classifier, runner, shims, bypass model, and redteam fixtures;
- command redteam packs run without live mutation;
- public copy says "preview" and "when routed through Boundary";
- release truth reconciliation records what Command Boundary proves and does not
  prove.

Production command governance requires additional deployment evidence that the
Boundary route is the relevant command path for the protected project or
workflow.

## Relationship To Existing Claims

Command Boundary does not alter v0.3.0 release truth. MCP remains the production
adapter path for v0.3.0. Secure GitHub remains preview. Command Boundary remains
roadmap/design until implementation evidence exists.
