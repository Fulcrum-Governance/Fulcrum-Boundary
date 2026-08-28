#!/bin/sh
# pretooluse-boundary.sh — Fulcrum Boundary PreToolUse hook for Claude Code.
#
# Claude Code invokes this script BEFORE it runs a tool, passing the PreToolUse
# event as JSON on stdin. This script does exactly two things:
#
#   1. Probe that a `boundary` binary is present AND knows `hook pretooluse`.
#   2. exec `boundary hook pretooluse`, which reads the same stdin and writes the
#      decision to stdout.
#
# All routing, classification, verdict mapping, and decision recording live in
# the binary (`boundary hook pretooluse`, internal/hookboundary). This wrapper
# parses nothing — no jq, no grep-based JSON extraction — so there is no reduced
# no-jq path to diverge from the real one.
#
# Routed surfaces (the ONLY route this hook governs is the tool call it intercepts):
#   - Bash / shell tool                       -> Command Boundary (preview)
#   - Edit / Write / MultiEdit / NotebookEdit -> Edit Boundary (preview)
#
# A tool call that does not reach this hook (a tool not wired in settings.json, a
# subprocess Claude spawns that runs another command, direct shell use outside
# Claude Code) is a BYPASS and is not governed. See docs/integrations/CLAUDE_CODE_HOOK.md.
#
# WHEN BOUNDARY CANNOT DECIDE this script prints an "ask" decision and exits 0
# rather than letting Claude Code fall through. That is deliberate: a hook that
# exits non-zero is treated as a hook error and the tool call proceeds ungoverned
# and silently. Asking makes the ungoverned state visible to the operator, who can
# approve the call themselves or fix the install. If you do not want to be asked,
# remove this hook from settings.json rather than leaving a hook installed that
# cannot decide.
#
# There are TWO such conditions, and both are resolved the same way:
#
#   - No binary at all (`command -v` finds nothing).
#   - A binary that predates the `hook` lane. An older `boundary` on PATH — a
#     brew or tarball install left over from before this hook shipped — exits 1
#     with empty stdout on `hook pretooluse`, which is exactly the ungoverned
#     fall-through above, and is otherwise indistinguishable from a working
#     install. The probe below runs `hook --help`, the same subcommand about to be
#     exec'd, because only that answers the question that matters.
#
# Environment knobs (all read by the binary except BOUNDARY_BIN, which is read here):
#   BOUNDARY_BIN            Path to the boundary binary (default: `boundary` on PATH).
#   BOUNDARY_HOOK_FAILMODE  Fault posture: `ask` (default), `open`, or `closed`.
#                           It governs internal FAULTS only; a Boundary deny always blocks.
#   BOUNDARY_HOOK_AGENT_ID  Optional agent id label recorded by Boundary (advisory).
#   BOUNDARY_HOOK_DIR       Directory decision records are written to.
#   BOUNDARY_HOOK_DEBUG     When non-empty, prints diagnostic lines to stderr —
#                           from this wrapper, and from the binary after exec.
#
# This hook leaves a classification-time verdict. It does NOT re-run the command
# or re-apply the edit (Claude Code performs the actual tool action after the hook
# allows it), so it does not double-execute. Command/Edit Boundary are delivered
# PREVIEWS; treat their verdicts as preview-grade. Nothing here is "tamper-proof"
# or a proof of safety; Boundary records are hash-verifiable integrity, not authenticity.

set -u

BOUNDARY_BIN="${BOUNDARY_BIN:-boundary}"

debug() {
	[ -n "${BOUNDARY_HOOK_DEBUG:-}" ] && printf 'boundary-hook: %s\n' "$*" >&2
	return 0
}

# Drain the event so the writer never sees a closed pipe on a branch that does not
# exec. Skipped when stdin is a terminal, i.e. when a human runs this by hand.
drain_event() {
	if [ ! -t 0 ]; then
		cat >/dev/null 2>&1
	fi
	return 0
}

# Probe FIRST, before touching stdin: a boundary that cannot decide is the one
# condition this wrapper resolves on its own.
if ! command -v "$BOUNDARY_BIN" >/dev/null 2>&1; then
	debug "boundary binary not found (looked for '$BOUNDARY_BIN'); emitting ask"
	drain_event
	# A fixed, fully static JSON object: nothing from the event reaches stdout.
	# Single-quoted so the shell performs no expansion or substitution on it.
	printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"Fulcrum Boundary is NOT installed, so this tool call was not governed: the boundary binary was not found. Approve only if you have reviewed this call yourself. Install it with: brew install fulcrum-governance/tap/boundary — or download a release tarball with curl and verify SHA256SUMS (see docs/INSTALL.md) — or set BOUNDARY_BIN to the absolute path of the binary. To stop being asked, remove the Boundary hook from your Claude Code settings.json."}}'
	exit 0
fi

# The binary exists — but does it have the lane about to be exec'd? `hook --help`
# exits 0 only on a boundary that carries `hook pretooluse`, and non-zero on one
# that predates it. Reading help is side-effect free: it decides nothing, writes
# no record, and never reads stdin.
if ! "$BOUNDARY_BIN" hook --help >/dev/null 2>&1; then
	debug "'$BOUNDARY_BIN' does not support 'hook pretooluse'; emitting ask"
	drain_event
	printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"Fulcrum Boundary did NOT govern this tool call: the installed boundary binary is too old and does not support `boundary hook pretooluse`. Approve only if you have reviewed this call yourself. Upgrade with: brew upgrade fulcrum-governance/tap/boundary — or download a newer release tarball and verify SHA256SUMS (see docs/INSTALL.md) — or set BOUNDARY_BIN to the absolute path of an up-to-date binary. To stop being asked, remove the Boundary hook from your Claude Code settings.json."}}'
	exit 0
fi

debug "delegating to '$BOUNDARY_BIN' hook pretooluse"

# Hand the process over. stdin (the PreToolUse event), stdout (the decision JSON),
# stderr, and the whole environment pass straight through; the binary always
# exits 0 on a decided path, because the JSON — not the exit code — is what tells
# Claude Code to allow, ask, or block.
exec "$BOUNDARY_BIN" hook pretooluse
