# Govern Claude Code Tool Calls

Put Boundary in front of [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
with a `PreToolUse` hook so an agent's tool calls are decided **before** they
run. Claude Code fires the hook after a tool is selected but before it executes;
the hook runs Boundary's preview classifiers and **blocks the tool call on a
`deny` verdict** — adding portable, redaction-aware policy that a raw hook config
lacks.

Boundary governs Claude Code **only for the tool calls this hook is wired to
intercept** — that routed interception is the boundary. A tool call that does not
reach the hook (an un-wired tool, an MCP tool, a subprocess Claude spawns, direct
shell use outside Claude Code) is a bypass and is not governed. Closing those
paths is a deployment responsibility, not a hook flag.

Routed surfaces:

```text
Bash / shell tool            -> boundary command classify   (Command Boundary, preview)
Edit / Write / MultiEdit     -> boundary edit inspect        (Edit Boundary, preview)
```

Command Boundary and Edit Boundary are **delivered previews**, not production GA.
Treat their verdicts as preview-grade. The hook leaves a hash-verifiable
classification (integrity, not authenticity); it does not re-run the tool, makes
no claim of total coverage, and does not emit `proved` decisions.

Expected deny signal:

```json
{ "decision": "block", "reason": "Fulcrum Boundary (Command Boundary preview) denied this command [C4]: destructive local mutation. ..." }
```

Canonical walkthrough:
[docs/integrations/CLAUDE_CODE_HOOK.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/integrations/CLAUDE_CODE_HOOK.md)

Related references:

- [integrations/claude-code/](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/integrations/claude-code/README.md)
  — the hook script, the `settings.json` snippet, and install steps.
- [docs/command-boundary/](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/README.md)
  — the Command Boundary preview the Bash route uses.
- [docs/edit-boundary/](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/README.md)
  — the Edit Boundary preview the Edit/Write route uses.
- [LIMITATIONS.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/LIMITATIONS.md)
  — the routed-only constraint in full.
