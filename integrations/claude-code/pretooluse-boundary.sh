#!/bin/sh
# pretooluse-boundary.sh — Fulcrum Boundary PreToolUse hook for Claude Code.
#
# Claude Code invokes this script BEFORE it runs a tool, passing the PreToolUse
# event as JSON on stdin (fields used here: .tool_name and .tool_input). The hook
# routes the tool through the matching Boundary classifier and BLOCKS the call
# when Boundary returns a `deny` verdict — before the tool runs. Everything else
# is allowed silently (exit 0, no output), so the hook is quiet on the happy path.
#
# Routed surfaces (the ONLY route this hook governs is the tool call it intercepts):
#   - Bash / shell tool        -> `boundary command classify --json` (Command Boundary, preview)
#   - Edit / Write / MultiEdit -> `boundary edit inspect --json`     (Edit Boundary, preview)
#
# A tool call that does not reach this hook (a tool not wired in settings.json, a
# subprocess Claude spawns that runs another command, direct shell use outside
# Claude Code) is a BYPASS and is not governed. See docs/integrations/CLAUDE_CODE_HOOK.md.
#
# Deny contract: this hook uses Claude Code's JSON PreToolUse decision form. On a
# Boundary `deny` it prints a single JSON object carrying BOTH the legacy
# {"decision":"block","reason":...} keys AND the current
# {"hookSpecificOutput":{"permissionDecision":"deny",...}} keys, so older and newer
# Claude Code clients both block the call; it exits 0 (the JSON, not the exit code,
# drives the block). The exit-code-2 + stderr form is an alternative the contract
# also accepts; this hook standardizes on the JSON form. See the canonical doc.
#
# Dependencies:
#   - `boundary` on PATH (or set BOUNDARY_BIN to its absolute path). REQUIRED.
#   - `jq` on PATH. STRONGLY RECOMMENDED. Without jq the hook degrades to a
#     reduced POSIX-only parse that handles the common single-line tool_input
#     shapes; if it cannot parse the event it fails per BOUNDARY_HOOK_FAILMODE.
#
# Environment knobs:
#   BOUNDARY_BIN            Path to the boundary binary (default: `boundary` on PATH).
#   BOUNDARY_HOOK_FAILMODE  `open` (default) or `closed`. On an internal fault
#                           (boundary missing, event unparseable, classifier error),
#                           `open` allows the call (exit 0) so a flaky hook never
#                           bricks an interactive session; `closed` blocks it.
#   BOUNDARY_HOOK_AGENT_ID  Optional agent id label recorded by Boundary (advisory).
#   BOUNDARY_HOOK_DEBUG     When non-empty, prints diagnostic lines to stderr.
#
# This hook leaves a classification-time verdict. It does NOT re-run the command
# or re-apply the edit (Claude Code performs the actual tool action after the hook
# allows it), so it does not double-execute. Command/Edit Boundary are delivered
# PREVIEWS; treat their verdicts as preview-grade. Nothing here is "tamper-proof"
# or a proof of safety; Boundary records are hash-verifiable integrity, not authenticity.

set -u

BOUNDARY_BIN="${BOUNDARY_BIN:-boundary}"
FAILMODE="${BOUNDARY_HOOK_FAILMODE:-open}"

debug() {
	[ -n "${BOUNDARY_HOOK_DEBUG:-}" ] && printf 'boundary-hook: %s\n' "$*" >&2
	return 0
}

# allow_call: permit the tool call. Quiet on the happy path (no stdout, exit 0).
allow_call() {
	debug "allow: $*"
	exit 0
}

# block_call <reason>: emit Claude Code's JSON block decision and exit 0. The JSON
# decision — not the exit code — is what tells Claude Code to stop the tool call.
block_call() {
	reason="$1"
	# Prefer jq to JSON-encode the reason safely; fall back to a conservative
	# manual escape (backslash and double-quote) when jq is unavailable.
	if [ -n "${HAVE_JQ:-}" ]; then
		printf '%s' "$reason" | jq -R -s '{decision:"block", reason:., hookSpecificOutput:{hookEventName:"PreToolUse", permissionDecision:"deny", permissionDecisionReason:.}}'
	else
		esc=$(printf '%s' "$reason" | sed 's/\\/\\\\/g; s/"/\\"/g')
		printf '{"decision":"block","reason":"%s","hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n' "$esc" "$esc"
	fi
	exit 0
}

# fault <message>: an internal hook fault (not a Boundary decision). Honor FAILMODE.
fault() {
	debug "fault ($FAILMODE): $1"
	if [ "$FAILMODE" = "closed" ]; then
		block_call "Fulcrum Boundary hook fault, failing closed: $1"
	fi
	# fail open: allow, but leave a single advisory line on stderr.
	printf 'boundary-hook: fault, allowing (set BOUNDARY_HOOK_FAILMODE=closed to block): %s\n' "$1" >&2
	exit 0
}

# Detect jq once.
if command -v jq >/dev/null 2>&1; then
	HAVE_JQ=1
else
	HAVE_JQ=""
fi

# json_str <key> <json>: no-jq fallback extractor. Returns the string value of the
# FIRST "<key>":"..." pair, for the common single-line PreToolUse shapes that carry
# no escaped quotes inside the value. This is a REDUCED parse: values containing an
# embedded double-quote are not handled (that is the jq path's job). Empty output
# means "not found / not parseable", which the caller treats as a fault or no-op.
json_str() {
	_k="$1"
	printf '%s' "$2" \
		| grep -o "\"${_k}\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" \
		| head -n1 \
		| sed "s/.*\"${_k}\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\"/\\1/"
}

# Resolve the boundary binary.
if ! command -v "$BOUNDARY_BIN" >/dev/null 2>&1; then
	fault "boundary binary not found (looked for '$BOUNDARY_BIN'; set BOUNDARY_BIN)"
fi

# Read the entire PreToolUse event from stdin.
EVENT=$(cat)
if [ -z "$EVENT" ]; then
	fault "empty PreToolUse event on stdin"
fi

# ----------------------------------------------------------------------------
# Field extraction. jq path is authoritative; the no-jq path is a reduced parse.
# ----------------------------------------------------------------------------
if [ -n "$HAVE_JQ" ]; then
	TOOL_NAME=$(printf '%s' "$EVENT" | jq -r '.tool_name // empty')
else
	TOOL_NAME=$(json_str tool_name "$EVENT")
fi

if [ -z "${TOOL_NAME:-}" ]; then
	fault "could not read .tool_name from event"
fi
debug "tool_name=$TOOL_NAME"

case "$TOOL_NAME" in
Bash | Shell | shell | bash)
	# --- Command Boundary route -------------------------------------------
	if [ -n "$HAVE_JQ" ]; then
		CMD=$(printf '%s' "$EVENT" | jq -r '.tool_input.command // empty')
	else
		CMD=$(json_str command "$EVENT")
	fi
	if [ -z "${CMD:-}" ]; then
		# No command field (or unparseable) — nothing to classify; allow.
		allow_call "no .tool_input.command to classify"
	fi
	debug "command=$CMD"

	# Classify the command's leading argv. Boundary parses argv only; it does NOT
	# interpret shell operators (&&, ||, |, ;), so a compound line is classified
	# by its LEADING command. This is a documented gap, not total coverage.
	# We word-split CMD intentionally (so `git push origin main` -> argv tokens).
	# shellcheck disable=SC2086
	OUT=$("$BOUNDARY_BIN" command classify --json -- $CMD 2>/dev/null)
	if [ -z "$OUT" ]; then
		fault "boundary command classify produced no output"
	fi

	if [ -n "$HAVE_JQ" ]; then
		ACTION=$(printf '%s' "$OUT" | jq -r '.recommended_action // empty')
		REASON=$(printf '%s' "$OUT" | jq -r '.reason // "policy match"')
		CLASS=$(printf '%s' "$OUT" | jq -r '.class // "?"')
	else
		ACTION=$(json_str recommended_action "$OUT")
		REASON=$(json_str reason "$OUT")
		CLASS=$(json_str class "$OUT")
		[ -z "${REASON:-}" ] && REASON="policy match"
		[ -z "${CLASS:-}" ] && CLASS="?"
	fi
	debug "command action=$ACTION class=$CLASS"

	if [ "${ACTION:-}" = "deny" ]; then
		block_call "Fulcrum Boundary (Command Boundary preview) denied this command [$CLASS]: $REASON. This is a routed pre-execution deny; the command was not run."
	fi
	# allow / warn / require_approval / unknown -> allow (Claude Code runs the tool).
	allow_call "command action=${ACTION:-none}"
	;;

Edit | Write | MultiEdit | NotebookEdit)
	# --- Edit Boundary route ----------------------------------------------
	if [ -n "$HAVE_JQ" ]; then
		FILE_PATH=$(printf '%s' "$EVENT" | jq -r '.tool_input.file_path // .tool_input.notebook_path // empty')
	else
		FILE_PATH=$(json_str file_path "$EVENT")
		[ -z "$FILE_PATH" ] && FILE_PATH=$(json_str notebook_path "$EVENT")
	fi
	if [ -z "${FILE_PATH:-}" ]; then
		allow_call "no .tool_input.file_path to classify"
	fi
	debug "file_path=$FILE_PATH"

	# Strip a leading slash so the synthesized diff names a repo-relative path that
	# the Edit Boundary path classifier reads (it matches secret-bearing and source
	# path shapes). We classify by PATH SHAPE only; we do not synthesize the content
	# hunk, so content-based classes are not asserted here — path-based deny (E4
	# secret-bearing, and destructive/source posture) is what this route enforces.
	REL=$(printf '%s' "$FILE_PATH" | sed 's#^/*##')
	DIFF=$(printf 'diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n' "$REL" "$REL" "$REL" "$REL")

	OUT=$(printf '%s' "$DIFF" | "$BOUNDARY_BIN" edit inspect --json --stdin 2>/dev/null)
	if [ -z "$OUT" ]; then
		fault "boundary edit inspect produced no output"
	fi

	if [ -n "$HAVE_JQ" ]; then
		ACTION=$(printf '%s' "$OUT" | jq -r '.recommended_action // empty')
		CLASS=$(printf '%s' "$OUT" | jq -r '.highest_class // "?"')
		EREASON=$(printf '%s' "$OUT" | jq -r '.findings[0].reason // "policy match"')
	else
		ACTION=$(json_str recommended_action "$OUT")
		CLASS=$(json_str highest_class "$OUT")
		EREASON="policy match"
		[ -z "${CLASS:-}" ] && CLASS="?"
	fi
	debug "edit action=$ACTION class=$CLASS"

	if [ "${ACTION:-}" = "deny" ]; then
		block_call "Fulcrum Boundary (Edit Boundary preview) denied this edit to '$FILE_PATH' [$CLASS]: $EREASON. This is a routed pre-execution deny; the file was not written."
	fi
	allow_call "edit action=${ACTION:-none}"
	;;

*)
	# Tool this hook does not govern (e.g. Read, Grep, WebFetch, an MCP tool).
	# Allow silently. Govern MCP tools at the MCP route, not here.
	allow_call "untracked tool: $TOOL_NAME"
	;;
esac
