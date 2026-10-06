# Boundary Enforcement Report

This report details the explicit coverage of the Claude Code `PreToolUse` hook path, what it enforces before execution, what it denies fail-closed, what it records, and what remains operator responsibility.

## Governed Routes

Boundary decides the proposed action only when the agent's route to the tool passes through the boundary. The `PreToolUse` hook routes tools based on its configured matcher.

**Routed Tool Classes:**
These tool classes match the hook configuration and are routed to Boundary for a pre-execution verdict:
- `Bash`
- `Edit`
- `Write`
- `MultiEdit`
- `NotebookEdit`

**Partially Routed / Not Routed:**
Tools not routed to Boundary are bypass paths. They execute un-governed and leave no decision record:
- Unmatched aliases like `bash`, `Shell`, `shell` are not routed by the plugin but would be supported if matched.
- `Read`, `WebFetch`, `Grep`, `Glob`, `Task`
- MCP tools (e.g., `mcp__postgres__query`, `mcp__Bash__run`) and any other unmatched tool.

## What Is Enforced Pre-Execution

For the **routed** tool classes, Boundary makes a verdict before execution:

- **Command Decomposing:** Command Boundary decomposes the proposed action into segments (e.g., `&&`, `||`, pipes) and nested shells (`sh -c`, `bash -c`, command substitutions, backticks). It takes the most restrictive verdict among all discovered segments.
- **Escape Classes Enforced:** The hook explicitly classifies and denies indirection patterns like `env`, `xargs`, `find -exec`, absolute paths (e.g., `/bin/rm`), and tested nested evaluation forms.
- **Undecomposable Lines:** Commands that the tokenizer cannot safely model (e.g., here-docs, `alias` definitions, `eval`) are flagged as undecomposable. They escalate to require approval (`ask`) and are never allowed silently.

### What Is Denied Fail-Closed

The request is denied fail-closed under specific conditions:

- **Self-Protection:** Writes to Boundary's own binaries, configurations, or policy records (e.g., `.claude/settings.json`, `.claude/hooks/pretooluse.sh`) are strictly denied before the files are touched.
- **Indeterminate Checks:** When a synchronous policy, budget, or trust check cannot produce a valid result (e.g., timeout, cancellation, missing identity), the outcome is `CHECK_INDETERMINATE`. It blocks execution rather than allowing it.

*Note: Missing binaries or other hook faults default to `ask` per the `BOUNDARY_HOOK_FAILMODE` mechanism, enabling the user to approve rather than fail-closed.*

## What Is Recorded

For every **routed** tool call that passes through the hook, a decision record is emitted.

- The structured decision record proves the verdict (allow, ask, deny) and is preserved as an inspectable receipt for the governed route.
- The decision record scope covers only routed actions. Un-routed tools produce **no decision records** through this hook path.

## Deployment Responsibility

Boundary evaluates the proposed action only when the tool reaches it. The following remain the operator or deployment topology's responsibility:

- **Un-Routed Tools:** Tools not explicitly routed through the boundary execute un-governed.
- **Interpreter Payloads:** Interpreter one-liners (e.g., `python -c`, `node -e`, `perl -e`) are documented bypasses. They are currently classified as `ask` to trigger user approval, but Boundary does not parse or govern their internal execution.
- **Bypass Paths:** Processes spawned outside of the agent, bare network egress, or any transport connection not intercepted by the configured adapter bypass the boundary entirely. The deployment infrastructure must make the bypass path unavailable.
