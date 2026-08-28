#!/bin/sh
# sessionend-boundary.sh — Fulcrum Boundary SessionEnd hook for Claude Code.
#
# Claude Code invokes this script when a session terminates, passing the
# SessionEnd event as JSON on stdin. This script does exactly two things:
#
#   1. Probe that a `boundary` binary is present AND knows `hook sessionend`.
#   2. exec `boundary hook sessionend`, which reads the same stdin.
#
# All behavior lives in the binary (`boundary hook sessionend`,
# internal/hookboundary). This wrapper parses nothing — no jq, no grep-based
# JSON extraction — so there is no reduced no-jq path to diverge from the real
# one. It is the SessionEnd sibling of pretooluse-boundary.sh and shares its
# probe-then-exec shape; see that script and
# docs/integrations/CLAUDE_CODE_HOOK.md for the fuller write-up of the pattern.
#
# SessionEnd does NOT block anything — Claude Code does not read a
# permissionDecision for this event, so unlike the PreToolUse wrapper, this
# script never emits an "ask" object when Boundary cannot decide. A session
# ending is not an action to gate; nagging the operator every time a session
# ends because Boundary is not installed, or predates this lane, would be pure
# noise. SO: WHEN BOUNDARY CANNOT RUN THIS, THAT SCRIPT EXITS 0 SILENTLY —
# no stdout, no stderr (beyond opt-in BOUNDARY_HOOK_DEBUG diagnostics) — and
# lets the session end exactly as it would with no hook installed at all.
#
# There are TWO such conditions, and both are resolved the same way (silent
# exit 0):
#
#   - No binary at all (`command -v` finds nothing).
#   - A binary that predates the `hook sessionend` lane. Probing the exact leaf
#     subcommand about to be exec'd — `hook sessionend --help` — catches both
#     "no hook lane at all" and "hook lane exists but sessionend does not",
#     because either shape exits non-zero on a binary that lacks it. Reading
#     help is side-effect free: it decides nothing, writes no record, and never
#     reads stdin.
#
# Environment knobs (all read by the binary except BOUNDARY_BIN, which is read
# here):
#   BOUNDARY_BIN            Path to the boundary binary (default: `boundary` on PATH).
#   BOUNDARY_HOOK_DIR       Directory decision records are written to.
#   BOUNDARY_HOOK_AGENT_ID  Optional agent id label recorded by Boundary (advisory).
#   BOUNDARY_HOOK_DEBUG     When non-empty, prints diagnostic lines to stderr —
#                           from this wrapper, and from the binary after exec.
#
# Boundary governs only the events routed to this hook. A session that ends
# without this hook wired — or a boundary binary too old to carry this lane —
# leaves no session receipt, silently. See LIMITATIONS.md.

set -u

BOUNDARY_BIN="${BOUNDARY_BIN:-boundary}"

debug() {
	[ -n "${BOUNDARY_HOOK_DEBUG:-}" ] && printf 'boundary-sessionend: %s\n' "$*" >&2
	return 0
}

# Drain the event so Claude Code never sees a closed pipe on a branch that does
# not exec. Skipped when stdin is a terminal, i.e. when a human runs this by hand.
drain_event() {
	if [ ! -t 0 ]; then
		cat >/dev/null 2>&1
	fi
	return 0
}

# Probe FIRST, before touching stdin. Unlike the PreToolUse wrapper, a failed
# probe here is not answered with an "ask" decision — SessionEnd has nothing to
# gate, so it is answered with silence and exit 0.
if ! command -v "$BOUNDARY_BIN" >/dev/null 2>&1; then
	debug "boundary binary not found (looked for '$BOUNDARY_BIN'); exiting silently"
	drain_event
	exit 0
fi

# The binary exists — but does it have the exact leaf subcommand about to be
# exec'd? `hook sessionend --help` exits 0 only on a boundary that carries
# `hook sessionend`, and non-zero on one that predates it (whether it lacks the
# whole `hook` lane or just this subcommand).
if ! "$BOUNDARY_BIN" hook sessionend --help >/dev/null 2>&1; then
	debug "'$BOUNDARY_BIN' does not support 'hook sessionend'; exiting silently"
	drain_event
	exit 0
fi

debug "delegating to '$BOUNDARY_BIN' hook sessionend"

# Hand the process over. stdin (the SessionEnd event), stdout, stderr, and the
# whole environment pass straight through.
exec "$BOUNDARY_BIN" hook sessionend
