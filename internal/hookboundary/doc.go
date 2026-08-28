// Package hookboundary decides Claude Code PreToolUse tool calls in-process and
// records the verdict.
//
// It is the binary-native replacement for the POSIX shell hook at
// integrations/claude-code/pretooluse-boundary.sh: the same routed surfaces and
// the same deny contract, without the `boundary` + `jq` subprocess fan-out, plus
// a hash-verifiable decision record for every decided event.
//
// # What it governs
//
// Exactly the tool calls Claude Code routes to it. A tool that is not wired into
// the hook matcher, a command a subprocess runs on its own, or direct shell use
// outside Claude Code never reaches this package and is a BYPASS. Routed
// interception is the boundary; nothing here claims total coverage of what an
// agent can do.
//
// Routing (RouteFor):
//
//   - Bash / Shell    -> Command Boundary (commandboundary.ClassifyLine)
//   - Edit / Write / MultiEdit / NotebookEdit
//     -> Edit Boundary (editboundary.InspectPatch on a synthesized one-file diff)
//   - everything else -> allowed silently, with NO decision record
//
// Command Boundary and Edit Boundary are delivered PREVIEWS. Their verdicts are
// preview-grade and their posture may change.
//
// # Verdict mapping
//
// Boundary's verdict maps onto Claude Code's PreToolUse permissionDecision:
//
//	deny             -> "deny"  (plus the legacy {"decision":"block"} keys)
//	require_approval -> "ask"
//	warn             -> "ask" with a permissionDecisionReason the user sees
//	allow            -> no stdout at all (silent allow, exit 0)
//
// This package never emits "allow". That value GRANTS — it bypasses the host's
// own permission prompt — so a gate that answered it would be more permissive
// than no gate at all for that call. See PermissionFor.
//
// # Fail mode
//
// A Boundary deny is a DECISION and always blocks. An internal fault — an
// unreadable or malformed event, a tool_input with nothing to classify, a
// classifier error — is not a decision. Faults resolve through FailMode, whose
// default is FailModeAsk: the operator is asked rather than silently allowed or
// hard-blocked. BOUNDARY_HOOK_FAILMODE=open restores the shell hook's permissive
// posture; =closed denies.
//
// A record-sink write failure is treated separately and never resolves to a
// silent allow, whatever the fail mode: see Sink and decide.go.
//
// # Records
//
// Every decided event — allow, warn, ask, and deny alike — produces a
// governance.DecisionRecordV1, persisted BEFORE the decision reaches stdout.
// Unmatched tools produce no record because nothing was decided.
//
// Records are hash-verifiable for INTEGRITY (boundary verify-record recomputes
// decision_hash). They are not authenticity, not proof the verdict was correct,
// and not proof the action was executed or prevented. execution_claim on these
// records reports upstream_called=false / executed=false: this package decides
// before execution and never runs the command or writes the file. That is a
// self-report about the hook, not corroboration that nothing else ran.
//
// # Self-protection
//
// Both routes deny writes to the governance control surfaces in
// editboundary.ControlSurfacePaths — the files that decide HOW this agent is
// governed, and the evidence of what was governed. The edit route gets it from
// Edit Boundary's E7 classes; the command route applies the same path shapes to
// the segments whose class writes a file (see commandControlSurface). It has to
// be both: a self-protection that denies `.claude/settings.json` as an Edit tool
// call while permitting `cp evil .claude/settings.json` as a Bash call is not a
// self-protection. It is a set of SHAPES, not an inventory of every way
// governance could be disabled.
//
// # Honest gaps carried forward from the shell hook
//
//   - Compound lines are decomposed, but not by a real shell. ClassifyBashLine
//     splits a Bash line into simple commands and classifies each one, so the
//     shell hook's leading-command gap is CLOSED here: `git status && rm -rf ~`
//     denies, `cat x > important.db` is a write rather than a read, and
//     `find . -exec rm -rf {} +` denies. What remains is the decomposer's own
//     modelling limit — heredocs, process substitution, `eval`, unbalanced
//     quotes, and nesting past commandboundary.MaxLineDepth are not modelled.
//     Those lines come back undecomposable and escalate to the user; they are
//     never allowed, but they are also not classified, so no class claim is made
//     about them. Argument-position execution outside the modelled shapes
//     (`watch`, `parallel`, an interpreter's inline program) leaves its payload
//     unclassified with the line still reported decomposed.
//   - Edit route is path-shape based. SynthesizeEditPatch names the target path
//     in a one-file diff with no content hunk, so content-only edit classes are
//     not asserted. The path itself IS asserted: an absolute target is resolved
//     against the project root, so a write outside it denies as E7 and a write
//     inside it keeps the class its position earns. Both paths are canonicalized
//     as far as the filesystem resolves them, so a symlink out of the project is
//     judged where the write lands — but that is a point-in-time answer, and a
//     symlink swapped after the check is not caught. Case is not folded, so on a
//     case-insensitive filesystem a case-flipped root is not matched.
//   - Redaction is pattern-based. commandboundary.RedactArgs and
//     editboundary's secret-path redaction run before anything is persisted, and
//     cover secret-looking flags and paths plus inline scheme://user:password@host
//     URL credentials, but pattern matching is not a guarantee that no
//     secret-looking argument survives into a record or into the reason handed
//     back to the model.
package hookboundary
