package hookboundary

import (
	"encoding/json"
	"strings"
)

// Verdict is Boundary's pre-execution verdict for a routed PreToolUse call. The
// four values are exactly the recommended actions Command Boundary and Edit
// Boundary produce, so a classifier verdict maps across without translation.
type Verdict string

const (
	// VerdictAllow permits the tool call and says nothing.
	VerdictAllow Verdict = "allow"
	// VerdictWarn permits the tool call but surfaces a reason to the user.
	VerdictWarn Verdict = "warn"
	// VerdictRequireApproval asks the user before the tool call runs.
	VerdictRequireApproval Verdict = "require_approval"
	// VerdictDeny blocks the tool call before it runs.
	VerdictDeny Verdict = "deny"
)

// ParseVerdict converts a classifier's recommended_action into a Verdict. It
// reports false for an unrecognized value so the caller can treat it as a fault
// rather than guessing a posture.
func ParseVerdict(raw string) (Verdict, bool) {
	switch Verdict(strings.TrimSpace(raw)) {
	case VerdictAllow:
		return VerdictAllow, true
	case VerdictWarn:
		return VerdictWarn, true
	case VerdictRequireApproval:
		return VerdictRequireApproval, true
	case VerdictDeny:
		return VerdictDeny, true
	default:
		return "", false
	}
}

// AtLeastAsStrict returns whichever of v and other blocks more. It is used when
// a fault would otherwise weaken an already-computed verdict: a record-sink
// failure escalates an allow to an ask, but it must never soften a deny.
func (v Verdict) AtLeastAsStrict(other Verdict) Verdict {
	if verdictStrength(other) > verdictStrength(v) {
		return other
	}
	return v
}

func verdictStrength(v Verdict) int {
	switch v {
	case VerdictAllow:
		return 0
	case VerdictWarn:
		return 1
	case VerdictRequireApproval:
		return 2
	case VerdictDeny:
		return 3
	default:
		// An unrecognized verdict is treated as the fail-mode default so it
		// can never rank below a real allow.
		return 2
	}
}

// PermissionDecision is the value Claude Code reads from
// hookSpecificOutput.permissionDecision.
type PermissionDecision string

const (
	// PermissionAllow lets the tool call proceed, BYPASSING the host's own
	// permission prompt. It is the vocabulary's only granting value, and this
	// package never emits it: see PermissionFor.
	PermissionAllow PermissionDecision = "allow"
	// PermissionAsk prompts the user before the tool call proceeds.
	PermissionAsk PermissionDecision = "ask"
	// PermissionDeny stops the tool call before it runs.
	PermissionDeny PermissionDecision = "deny"
)

// PermissionFor maps a Boundary verdict onto Claude Code's permissionDecision.
// It also reports whether the hook emits any stdout at all: a plain allow is
// silent, matching the shell hook's quiet happy path, so the second return is
// false for VerdictAllow and true for every other verdict.
//
// The hook NEVER emits PermissionAllow. In Claude Code's vocabulary "allow" does
// not mean "Boundary has no objection" — it means "skip the permission system",
// which is a grant the host would not otherwise have made. A gate that answers
// "allow" to the classes it rates riskiest while staying silent on the safest
// class is strictly more permissive than no gate at all, so warn maps to "ask"
// rather than to a grant: the reason still reaches the user, and the host's own
// permission flow still runs.
//
//	allow            -> no stdout at all (silent; the host decides as usual)
//	warn             -> "ask", carrying the warning as the reason
//	require_approval -> "ask"
//	deny             -> "deny"
func PermissionFor(v Verdict) (PermissionDecision, bool) {
	switch v {
	case VerdictAllow:
		return PermissionAllow, false
	case VerdictWarn:
		return PermissionAsk, true
	case VerdictDeny:
		return PermissionDeny, true
	case VerdictRequireApproval:
		return PermissionAsk, true
	default:
		// Unrecognized verdicts escalate rather than guess.
		return PermissionAsk, true
	}
}

// hookSpecificOutput is Claude Code's current PreToolUse decision shape.
type hookSpecificOutput struct {
	HookEventName            string             `json:"hookEventName"`
	PermissionDecision       PermissionDecision `json:"permissionDecision"`
	PermissionDecisionReason string             `json:"permissionDecisionReason"`
}

// output is the JSON object the hook writes to stdout. On a deny it carries BOTH
// the legacy decision/reason keys and the current hookSpecificOutput keys, so
// older and newer Claude Code clients both block the call. The JSON — not the
// exit code — is what drives the decision; the hook always exits 0.
type output struct {
	Decision           string              `json:"decision,omitempty"`
	Reason             string              `json:"reason,omitempty"`
	HookSpecificOutput *hookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// BuildOutput renders the hook's stdout bytes for a verdict and reason. It
// returns nil for a silent allow (no stdout, exit 0).
//
// A deny emits the dual-shape object described on output: the legacy
// {"decision":"block","reason":...} keys alongside
// {"hookSpecificOutput":{"permissionDecision":"deny",...}}. Both keys carry the
// same verdict, so emitting both is safe on either client version.
func BuildOutput(v Verdict, reason string) []byte {
	decision, emit := PermissionFor(v)
	if !emit {
		return nil
	}
	out := output{
		HookSpecificOutput: &hookSpecificOutput{
			HookEventName:            HookEventName,
			PermissionDecision:       decision,
			PermissionDecisionReason: reason,
		},
	}
	if decision == PermissionDeny {
		out.Decision = "block"
		out.Reason = reason
	}
	// The struct holds only strings, so json.Marshal cannot fail here.
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return append(encoded, '\n')
}
