package hookboundary

import (
	"encoding/json"
	"testing"
)

func TestParseVerdictAcceptsClassifierActionsOnly(t *testing.T) {
	for raw, want := range map[string]Verdict{
		"allow":            VerdictAllow,
		"warn":             VerdictWarn,
		"require_approval": VerdictRequireApproval,
		"deny":             VerdictDeny,
		" deny ":           VerdictDeny,
	} {
		got, ok := ParseVerdict(raw)
		if !ok || got != want {
			t.Fatalf("ParseVerdict(%q) = %q,%v; want %q,true", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"", "escalate", "block", "DENY", "proved"} {
		if got, ok := ParseVerdict(raw); ok {
			t.Fatalf("ParseVerdict(%q) = %q,true; want not-ok so the caller can fault", raw, got)
		}
	}
}

func TestVerdictAtLeastAsStrictNeverSoftensADecision(t *testing.T) {
	cases := []struct {
		have, other, want Verdict
	}{
		{VerdictAllow, VerdictRequireApproval, VerdictRequireApproval},
		{VerdictWarn, VerdictRequireApproval, VerdictRequireApproval},
		{VerdictRequireApproval, VerdictRequireApproval, VerdictRequireApproval},
		{VerdictDeny, VerdictRequireApproval, VerdictDeny},
		{VerdictDeny, VerdictAllow, VerdictDeny},
	}
	for _, tc := range cases {
		if got := tc.have.AtLeastAsStrict(tc.other); got != tc.want {
			t.Fatalf("%q.AtLeastAsStrict(%q) = %q, want %q", tc.have, tc.other, got, tc.want)
		}
	}
}

// TestPermissionForMapsEveryVerdict is the BOU-3 mapping table.
func TestPermissionForMapsEveryVerdict(t *testing.T) {
	cases := []struct {
		verdict    Verdict
		want       PermissionDecision
		wantOutput bool
	}{
		{VerdictDeny, PermissionDeny, true},
		{VerdictRequireApproval, PermissionAsk, true},
		// warn is riskier than allow, so it may not resolve to the vocabulary's
		// only granting value: it asks, carrying the warning as the reason.
		{VerdictWarn, PermissionAsk, true},
		{VerdictAllow, PermissionAllow, false},
		{Verdict("nonsense"), PermissionAsk, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.verdict), func(t *testing.T) {
			got, emit := PermissionFor(tc.verdict)
			if got != tc.want || emit != tc.wantOutput {
				t.Fatalf("PermissionFor(%q) = %q,%v; want %q,%v", tc.verdict, got, emit, tc.want, tc.wantOutput)
			}
		})
	}
}

// TestHookNeverEmitsAGrant is the permission-widening guard. "allow" is the only
// permissionDecision that GRANTS — it bypasses the host's own permission prompt —
// so a hook that emits it is strictly more permissive than no hook at all for
// that call. Boundary's job here is to gate, never to approve on the operator's
// behalf, so no verdict may put "allow" on stdout.
func TestHookNeverEmitsAGrant(t *testing.T) {
	for _, verdict := range []Verdict{
		VerdictAllow, VerdictWarn, VerdictRequireApproval, VerdictDeny, Verdict("nonsense"),
	} {
		t.Run(string(verdict), func(t *testing.T) {
			out := BuildOutput(verdict, "because")
			if out == nil {
				return // A silent allow grants nothing: the host decides as usual.
			}
			var decoded struct {
				HookSpecificOutput struct {
					PermissionDecision PermissionDecision `json:"permissionDecision"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal(out, &decoded); err != nil {
				t.Fatalf("unmarshal %q: %v", out, err)
			}
			if decoded.HookSpecificOutput.PermissionDecision == PermissionAllow {
				t.Fatalf("verdict %q emitted an explicit grant: %s", verdict, out)
			}
		})
	}
}

func TestBuildOutputIsSilentOnAllow(t *testing.T) {
	if out := BuildOutput(VerdictAllow, "anything"); out != nil {
		t.Fatalf("BuildOutput(allow) = %q, want no stdout at all", out)
	}
}

// TestBuildOutputDenyCarriesBothDenyShapes pins the deny contract: the legacy
// keys and the current hookSpecificOutput keys ship together so older and newer
// Claude Code clients both block.
func TestBuildOutputDenyCarriesBothDenyShapes(t *testing.T) {
	const reason = "Fulcrum Boundary denied this command."
	var decoded struct {
		Decision           string `json:"decision"`
		Reason             string `json:"reason"`
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	out := BuildOutput(VerdictDeny, reason)
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if decoded.Decision != "block" || decoded.Reason != reason {
		t.Fatalf("legacy deny keys = %q/%q", decoded.Decision, decoded.Reason)
	}
	if decoded.HookSpecificOutput.HookEventName != HookEventName {
		t.Fatalf("hookEventName = %q", decoded.HookSpecificOutput.HookEventName)
	}
	if decoded.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("permissionDecision = %q", decoded.HookSpecificOutput.PermissionDecision)
	}
	if decoded.HookSpecificOutput.PermissionDecisionReason != reason {
		t.Fatalf("permissionDecisionReason = %q", decoded.HookSpecificOutput.PermissionDecisionReason)
	}
}

// TestBuildOutputNonDenyOmitsTheLegacyBlockKeys guards against an ask or a warn
// reading as a block on a client that only understands the legacy shape.
func TestBuildOutputNonDenyOmitsTheLegacyBlockKeys(t *testing.T) {
	for _, tc := range []struct {
		verdict Verdict
		want    string
	}{
		{VerdictRequireApproval, "ask"},
		{VerdictWarn, "ask"},
	} {
		t.Run(string(tc.verdict), func(t *testing.T) {
			var decoded map[string]any
			if err := json.Unmarshal(BuildOutput(tc.verdict, "because"), &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if _, ok := decoded["decision"]; ok {
				t.Fatalf("%q emitted a legacy decision key: %v", tc.verdict, decoded)
			}
			if _, ok := decoded["reason"]; ok {
				t.Fatalf("%q emitted a legacy reason key: %v", tc.verdict, decoded)
			}
			hook, ok := decoded["hookSpecificOutput"].(map[string]any)
			if !ok {
				t.Fatalf("missing hookSpecificOutput: %v", decoded)
			}
			if hook["permissionDecision"] != tc.want {
				t.Fatalf("permissionDecision = %v, want %q", hook["permissionDecision"], tc.want)
			}
			if hook["permissionDecisionReason"] != "because" {
				t.Fatalf("reason = %v, want it surfaced to the user", hook["permissionDecisionReason"])
			}
		})
	}
}

func TestParseFailModeDefaultsToAsk(t *testing.T) {
	for raw, want := range map[string]FailMode{
		"open":     FailModeOpen,
		"OPEN":     FailModeOpen,
		" closed ": FailModeClosed,
		"ask":      FailModeAsk,
		"":         FailModeAsk,
		"nonsense": FailModeAsk,
	} {
		if got := ParseFailMode(raw); got != want {
			t.Fatalf("ParseFailMode(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestFailModeVerdict(t *testing.T) {
	for mode, want := range map[FailMode]Verdict{
		FailModeOpen:   VerdictAllow,
		FailModeClosed: VerdictDeny,
		FailModeAsk:    VerdictRequireApproval,
		FailMode(""):   VerdictRequireApproval,
	} {
		if got := mode.Verdict(); got != want {
			t.Fatalf("FailMode(%q).Verdict() = %q, want %q", mode, got, want)
		}
	}
}

func TestConfigFromEnvReadsTheHookKnobs(t *testing.T) {
	env := map[string]string{
		EnvFailMode:  "closed",
		EnvAgentID:   "agent-7",
		EnvRecordDir: " /tmp/records ",
	}
	cfg := ConfigFromEnv(func(key string) string { return env[key] }, "v9.9.9")
	if cfg.FailMode != FailModeClosed {
		t.Fatalf("fail mode = %q", cfg.FailMode)
	}
	if cfg.AgentID != "agent-7" {
		t.Fatalf("agent id = %q", cfg.AgentID)
	}
	if cfg.Dir != "/tmp/records" {
		t.Fatalf("dir = %q", cfg.Dir)
	}
	if cfg.BoundaryVersion != "v9.9.9" {
		t.Fatalf("version = %q", cfg.BoundaryVersion)
	}
}

func TestConfigWithDefaultsFillsEveryField(t *testing.T) {
	cfg := Config{}.withDefaults()
	if cfg.Dir != DefaultRecordDir || cfg.FailMode != FailModeAsk ||
		cfg.AgentID != DefaultAgentID || cfg.TenantID != DefaultTenantID || cfg.Now == nil {
		t.Fatalf("defaults = %#v", cfg)
	}
}
