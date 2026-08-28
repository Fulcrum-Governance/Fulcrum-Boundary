package hookboundary

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// decideString runs one event through Decide with a real sink under dir.
func decideString(t *testing.T, dir, event string) Result {
	t.Helper()
	return Decide(Config{Dir: dir, BoundaryVersion: "v-test"}, strings.NewReader(event))
}

// permissionOf decodes the hookSpecificOutput.permissionDecision from stdout.
// It returns "" when the hook emitted nothing (a silent allow).
func permissionOf(t *testing.T, stdout []byte) string {
	t.Helper()
	if len(stdout) == 0 {
		return ""
	}
	var decoded struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(stdout, &decoded); err != nil {
		t.Fatalf("stdout %q is not hook JSON: %v", stdout, err)
	}
	return decoded.HookSpecificOutput.PermissionDecision
}

// TestDecideRoutesAndMapsEveryVerdict is the routing plus BOU-3 mapping table
// end to end: a real event in, a permissionDecision and a record out.
func TestDecideRoutesAndMapsEveryVerdict(t *testing.T) {
	cases := []struct {
		name           string
		event          string
		wantRoute      Route
		wantVerdict    Verdict
		wantPermission string
		wantRecord     bool
	}{
		{
			name:           "bash observe allows silently",
			event:          `{"tool_name":"Bash","tool_input":{"command":"git status"}}`,
			wantRoute:      RouteCommand,
			wantVerdict:    VerdictAllow,
			wantPermission: "",
			wantRecord:     true,
		},
		{
			// warn surfaces a reason and asks; it never grants. A hook that
			// answered "allow" here would auto-approve a class it rates riskier
			// than the class it stays silent on.
			name:           "bash local write warns and asks with a reason",
			event:          `{"tool_name":"Bash","tool_input":{"command":"touch notes.txt"}}`,
			wantRoute:      RouteCommand,
			wantVerdict:    VerdictWarn,
			wantPermission: "ask",
			wantRecord:     true,
		},
		{
			name:           "bash repo mutation asks",
			event:          `{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}`,
			wantRoute:      RouteCommand,
			wantVerdict:    VerdictRequireApproval,
			wantPermission: "ask",
			wantRecord:     true,
		},
		{
			name:           "bash destructive mutation denies",
			event:          `{"tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}`,
			wantRoute:      RouteCommand,
			wantVerdict:    VerdictDeny,
			wantPermission: "deny",
			wantRecord:     true,
		},
		{
			name:           "compound line denies on its chained tail",
			event:          `{"tool_name":"Bash","tool_input":{"command":"git status && rm -rf ~"}}`,
			wantRoute:      RouteCommand,
			wantVerdict:    VerdictDeny,
			wantPermission: "deny",
			wantRecord:     true,
		},
		{
			name:           "undecomposable line asks rather than allowing",
			event:          `{"tool_name":"Bash","tool_input":{"command":"cat <<EOF"}}`,
			wantRoute:      RouteCommand,
			wantVerdict:    VerdictRequireApproval,
			wantPermission: "ask",
			wantRecord:     true,
		},
		{
			name:           "write to a secret-bearing path denies",
			event:          `{"tool_name":"Write","tool_input":{"file_path":"config/.env","content":"x"}}`,
			wantRoute:      RouteEdit,
			wantVerdict:    VerdictDeny,
			wantPermission: "deny",
			wantRecord:     true,
		},
		{
			name:           "edit to source asks",
			event:          `{"tool_name":"Edit","tool_input":{"file_path":"internal/thing.go"}}`,
			wantRoute:      RouteEdit,
			wantVerdict:    VerdictRequireApproval,
			wantPermission: "ask",
			wantRecord:     true,
		},
		{
			name:           "notebook edit routes on notebook_path",
			event:          `{"tool_name":"NotebookEdit","tool_input":{"notebook_path":"docs/tour.ipynb"}}`,
			wantRoute:      RouteEdit,
			wantVerdict:    VerdictAllow,
			wantPermission: "",
			wantRecord:     true,
		},
		{
			name:           "ungoverned tool allows silently with no record",
			event:          `{"tool_name":"Read","tool_input":{"file_path":"config/.env"}}`,
			wantRoute:      RouteNone,
			wantVerdict:    VerdictAllow,
			wantPermission: "",
			wantRecord:     false,
		},
		{
			name:           "mcp tool is not governed here",
			event:          `{"tool_name":"mcp__github__create_issue","tool_input":{"title":"x"}}`,
			wantRoute:      RouteNone,
			wantVerdict:    VerdictAllow,
			wantPermission: "",
			wantRecord:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			result := decideString(t, dir, tc.event)
			if result.Fault != nil {
				t.Fatalf("unexpected fault: %v", result.Fault)
			}
			if result.Route != tc.wantRoute {
				t.Fatalf("route = %q, want %q", result.Route, tc.wantRoute)
			}
			if result.Verdict != tc.wantVerdict {
				t.Fatalf("verdict = %q, want %q (reason %q)", result.Verdict, tc.wantVerdict, result.Reason)
			}
			if got := permissionOf(t, result.Stdout); got != tc.wantPermission {
				t.Fatalf("permissionDecision = %q, want %q", got, tc.wantPermission)
			}
			if tc.wantRecord {
				if result.Record == nil {
					t.Fatal("no decision record persisted for a decided event")
				}
				if result.Record.Action != string(tc.wantVerdict) {
					t.Fatalf("record action = %q, want %q", result.Record.Action, tc.wantVerdict)
				}
				assertRecordOnDisk(t, result)
			} else if result.Record != nil {
				t.Fatalf("ungoverned tool left a record: %#v", result.Record)
			}
		})
	}
}

// TestDecideCompoundDenyNamesTheOffendingSegment is the BOU-5 wiring contract:
// a compound deny must tell the operator WHICH segment blocked the call, not
// just that something did, and it must not name the benign leading command as
// the offender.
func TestDecideCompoundDenyNamesTheOffendingSegment(t *testing.T) {
	dir := t.TempDir()
	result := decideString(t, dir, `{"tool_name":"Bash","tool_input":{"command":"git status && rm -rf ~"}}`)
	if result.Verdict != VerdictDeny {
		t.Fatalf("verdict = %q, want deny (reason %q)", result.Verdict, result.Reason)
	}
	for _, want := range []string{"rm -rf ~", "compound segments", "C4"} {
		if !strings.Contains(result.Reason, want) {
			t.Fatalf("reason %q does not carry %q", result.Reason, want)
		}
	}
	if result.Record == nil {
		t.Fatalf("no record; sink=%v", result.SinkError)
	}
	if result.Record.MatchedRule != "command-preview-posture:C4" {
		t.Fatalf("matched_rule = %q, want the offending segment's class", result.Record.MatchedRule)
	}
	if !strings.Contains(result.Record.Reason, "rm -rf ~") {
		t.Fatalf("record reason %q does not carry the offending segment", result.Record.Reason)
	}
	assertRecordOnDisk(t, result)
}

// TestDecideUndecomposableLineIsEscalatedNotAllowed pins that a line Boundary
// could not decompose is escalated as a DECISION (not a fault) and recorded
// with a request hash, and that the reason says why.
func TestDecideUndecomposableLineIsEscalatedNotAllowed(t *testing.T) {
	for _, line := range []string{"cat <<EOF", "eval \\\"$PAYLOAD\\\"", "diff <(ls) <(ls -a)"} {
		t.Run(line, func(t *testing.T) {
			dir := t.TempDir()
			result := decideString(t, dir,
				`{"tool_name":"Bash","tool_input":{"command":"`+line+`"}}`)
			if result.Fault != nil {
				t.Fatalf("undecomposable line reported as a fault: %v", result.Fault)
			}
			if result.Verdict != VerdictRequireApproval {
				t.Fatalf("verdict = %q, want require_approval (reason %q)", result.Verdict, result.Reason)
			}
			if got := permissionOf(t, result.Stdout); got != "ask" {
				t.Fatalf("permissionDecision = %q, want ask", got)
			}
			if !strings.Contains(result.Reason, "could not be safely decomposed") {
				t.Fatalf("reason %q does not explain the escalation", result.Reason)
			}
			// The reason is read by a human under a prompt: it must not carry a
			// placeholder standing in for a command Boundary never decomposed.
			if strings.Contains(result.Reason, "(no decomposed command segment)") {
				t.Fatalf("reason %q shows an empty-segment placeholder", result.Reason)
			}
			if result.Record == nil {
				t.Fatalf("no record; sink=%v", result.SinkError)
			}
			if result.Record.RequestHash == "" || result.Record.EventType == EventTypeParseRejected {
				t.Fatalf("record = %#v, want a decided event bound to a request", result.Record)
			}
			assertRecordOnDisk(t, result)
		})
	}
}

// TestDecideAllowsUngovernedToolsWithoutTouchingDisk pins that an ungoverned
// tool leaves no artifact at all — no directory, no empty log.
func TestDecideAllowsUngovernedToolsWithoutTouchingDisk(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "hook")
	result := decideString(t, dir, `{"tool_name":"Grep","tool_input":{"pattern":"x"}}`)
	if result.Stdout != nil {
		t.Fatalf("stdout = %q, want a silent allow", result.Stdout)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("record directory was created for an ungoverned tool: %v", err)
	}
}

// assertRecordOnDisk re-reads the persisted single-record file and verifies it
// the way `boundary verify-record` does, through the same governance functions.
func assertRecordOnDisk(t *testing.T, result Result) {
	t.Helper()
	body, err := os.ReadFile(result.Paths.Record)
	if err != nil {
		t.Fatalf("read record %s: %v", result.Paths.Record, err)
	}
	var record governance.DecisionRecordV1
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatalf("record is not a single JSON object: %v", err)
	}
	if !governance.SupportedDecisionRecordSchemaVersion(record.SchemaVersion) {
		t.Fatalf("schema_version = %q is not verifiable by this build", record.SchemaVersion)
	}
	if err := governance.VerifyDecisionRecord(record, nil, "", ""); err != nil {
		t.Fatalf("verify-record equivalent failed: %v", err)
	}
	if record.DecisionHash != result.Record.DecisionHash || record.RecordID != result.Record.RecordID {
		t.Fatalf("on-disk record differs from the returned one: %q/%q vs %q/%q",
			record.DecisionHash, record.RecordID, result.Record.DecisionHash, result.Record.RecordID)
	}
}

// TestDecideRecordCarriesHookRouteContext pins the fields that make a hook
// record identifiable and honest.
func TestDecideRecordCarriesHookRouteContext(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Dir: dir, BoundaryVersion: "v1.2.3", AgentID: "agent-9"}
	result := Decide(cfg, strings.NewReader(
		`{"hook_event_name":"PreToolUse","session_id":"sess-abc","cwd":"/repo",`+
			`"tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}`))
	if result.Record == nil {
		t.Fatalf("no record; fault=%v sink=%v", result.Fault, result.SinkError)
	}
	record := *result.Record

	if record.SchemaVersion != governance.DecisionRecordSchemaV2 {
		t.Fatalf("schema_version = %q, want the additive route-context version %q",
			record.SchemaVersion, governance.DecisionRecordSchemaV2)
	}
	if record.Adapter != TransportHook {
		t.Fatalf("adapter = %q, want %q", record.Adapter, TransportHook)
	}
	if record.AdapterID != AdapterID || record.RouteID != RoutePrefix+"Bash" {
		t.Fatalf("route context = %q/%q", record.AdapterID, record.RouteID)
	}
	if record.Tool != "Bash" {
		t.Fatalf("tool = %q", record.Tool)
	}
	if record.AgentID != "agent-9" || record.TenantID != DefaultTenantID {
		t.Fatalf("identity = %q/%q", record.AgentID, record.TenantID)
	}
	if record.BoundaryVersion != "v1.2.3" {
		t.Fatalf("boundary_version = %q", record.BoundaryVersion)
	}
	if record.DecisionMode != governance.DecisionModeDeterministic {
		t.Fatalf("decision_mode = %q, want deterministic", record.DecisionMode)
	}
	if record.RequestHash == "" || record.RawShapeHash != "" {
		t.Fatalf("hashes = request %q raw-shape %q; a decided event binds to a request",
			record.RequestHash, record.RawShapeHash)
	}
	if record.MatchedRule != "command-preview-posture:C4" {
		t.Fatalf("matched_rule = %q", record.MatchedRule)
	}
	if record.ExecutionClaim == nil ||
		record.ExecutionClaim.UpstreamCalled || record.ExecutionClaim.Executed ||
		record.ExecutionClaim.Source != ExecutionClaimSource {
		t.Fatalf("execution_claim = %#v, want a pre-execution self-report", record.ExecutionClaim)
	}
	if !strings.HasPrefix(record.TraceID, "sess-abc#") {
		t.Fatalf("trace_id = %q, want the session id then the sequence", record.TraceID)
	}
}

// TestDecideSequenceAdvancesPerDecision pins that a process deciding several
// events distinguishes them, and that the trace id carries the session.
func TestDecideSequenceAdvancesPerDecision(t *testing.T) {
	dir := t.TempDir()
	event := `{"session_id":"sess-seq","tool_name":"Bash","tool_input":{"command":"git status"}}`
	first := decideString(t, dir, event)
	second := decideString(t, dir, event)
	if first.Sequence == 0 || second.Sequence <= first.Sequence {
		t.Fatalf("sequences = %d then %d, want strictly increasing", first.Sequence, second.Sequence)
	}
	if first.Record.TraceID == second.Record.TraceID {
		t.Fatalf("both records share trace_id %q", first.Record.TraceID)
	}
	for _, r := range []Result{first, second} {
		if !strings.HasPrefix(r.Record.TraceID, "sess-seq#") {
			t.Fatalf("trace_id = %q", r.Record.TraceID)
		}
	}
}

func TestDecideTraceIDMarksAMissingSession(t *testing.T) {
	dir := t.TempDir()
	result := decideString(t, dir, `{"tool_name":"Bash","tool_input":{"command":"git status"}}`)
	if !strings.HasPrefix(result.Record.TraceID, noSessionLabel+"#") {
		t.Fatalf("trace_id = %q, want the explicit no-session label", result.Record.TraceID)
	}
}

// TestDecideRedactsSecretsBeforePersisting covers the redaction contract: a
// secret-looking argument, a secret-bearing path, and tool_input file content
// must none of them reach the record or the log.
func TestDecideRedactsSecretsBeforePersisting(t *testing.T) {
	cases := []struct {
		name    string
		event   string
		absent  []string
		present string
	}{
		{
			name:    "secret command argument",
			event:   `{"tool_name":"Bash","tool_input":{"command":"curl --token hunter2SuperSecret https://example.invalid"}}`,
			absent:  []string{"hunter2SuperSecret"},
			present: "[redacted]",
		},
		{
			name:    "secret-bearing edit path",
			event:   `{"tool_name":"Write","tool_input":{"file_path":"deploy/.env.production","content":"API_KEY=live"}}`,
			absent:  []string{"deploy/.env.production", "API_KEY=live"},
			present: "[redacted-secret-path]",
		},
		{
			name:    "write content never lands in a record",
			event:   `{"tool_name":"Write","tool_input":{"file_path":"internal/thing.go","content":"TOTALLY_UNIQUE_CONTENT_MARKER"}}`,
			absent:  []string{"TOTALLY_UNIQUE_CONTENT_MARKER"},
			present: "internal/thing.go",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			result := decideString(t, dir, tc.event)
			if result.Record == nil {
				t.Fatalf("no record; fault=%v", result.Fault)
			}
			recordBody, err := os.ReadFile(result.Paths.Record)
			if err != nil {
				t.Fatalf("read record: %v", err)
			}
			logBody, err := os.ReadFile(result.Paths.Log)
			if err != nil {
				t.Fatalf("read log: %v", err)
			}
			for _, artifact := range []string{string(recordBody), string(logBody), string(result.Stdout), result.Reason} {
				for _, secret := range tc.absent {
					if strings.Contains(artifact, secret) {
						t.Fatalf("artifact leaked %q:\n%s", secret, artifact)
					}
				}
			}
			if !strings.Contains(result.Reason, tc.present) {
				t.Fatalf("reason %q missing the expected redacted marker %q", result.Reason, tc.present)
			}
		})
	}
}

// TestDecideFailModeMatrix covers every fault input against every fail mode.
func TestDecideFailModeMatrix(t *testing.T) {
	faults := map[string]string{
		"empty stdin":           ``,
		"malformed json":        `{"tool_name":"Bash"`,
		"no tool name":          `{"tool_input":{"command":"git status"}}`,
		"wrong hook event":      `{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"git status"}}`,
		"bash without command":  `{"tool_name":"Bash","tool_input":{}}`,
		"edit without path":     `{"tool_name":"Edit","tool_input":{"old_string":"a"}}`,
		"tool input not object": `{"tool_name":"Bash","tool_input":"git status"}`,
	}
	modes := []struct {
		raw            string
		wantVerdict    Verdict
		wantPermission string
	}{
		{"", VerdictRequireApproval, "ask"},
		{"ask", VerdictRequireApproval, "ask"},
		{"nonsense", VerdictRequireApproval, "ask"},
		{"open", VerdictAllow, ""},
		{"closed", VerdictDeny, "deny"},
	}

	for faultName, event := range faults {
		for _, mode := range modes {
			t.Run(faultName+"/"+mode.raw, func(t *testing.T) {
				dir := t.TempDir()
				cfg := Config{Dir: dir, FailMode: ParseFailMode(mode.raw), BoundaryVersion: "v-test"}
				result := Decide(cfg, strings.NewReader(event))
				if result.Fault == nil {
					t.Fatalf("expected a fault, got verdict %q", result.Verdict)
				}
				if result.Verdict != mode.wantVerdict {
					t.Fatalf("verdict = %q, want %q (reason %q)", result.Verdict, mode.wantVerdict, result.Reason)
				}
				if got := permissionOf(t, result.Stdout); got != mode.wantPermission {
					t.Fatalf("permissionDecision = %q, want %q", got, mode.wantPermission)
				}
				// A fault is still a decided event: it must leave a record.
				if result.Record == nil {
					t.Fatalf("fault left no decision record; sink=%v", result.SinkError)
				}
				if result.Record.EventType != EventTypeParseRejected {
					t.Fatalf("event_type = %q, want %q", result.Record.EventType, EventTypeParseRejected)
				}
				if result.Record.RawShapeHash == "" || result.Record.RequestHash != "" {
					t.Fatalf("fault record hashes = raw %q request %q; want the observed shape only",
						result.Record.RawShapeHash, result.Record.RequestHash)
				}
				assertRecordOnDisk(t, result)
			})
		}
	}
}

// TestDecideFaultRecordNamesNoRouteWithoutAToolName pins that a record never
// asserts a route the request did not travel.
func TestDecideFaultRecordNamesNoRouteWithoutAToolName(t *testing.T) {
	dir := t.TempDir()
	result := decideString(t, dir, `{"tool_input":{"command":"git status"}}`)
	if result.Record == nil {
		t.Fatalf("no record; sink=%v", result.SinkError)
	}
	if result.Record.RouteID != "" {
		t.Fatalf("route_id = %q, want it empty when no tool was named", result.Record.RouteID)
	}
	if result.Record.AdapterID != AdapterID {
		t.Fatalf("adapter_id = %q, want the hook that parsed the event", result.Record.AdapterID)
	}
}

// TestDecideFaultReasonNamesTheFailModeKnob keeps an unexpected prompt
// self-explaining.
func TestDecideFaultReasonNamesTheFailModeKnob(t *testing.T) {
	dir := t.TempDir()
	result := decideString(t, dir, `{"tool_name":`)
	if !strings.Contains(result.Reason, EnvFailMode) {
		t.Fatalf("reason %q does not name %s", result.Reason, EnvFailMode)
	}
}

// TestDecideDegradesWhenTheRecordCannotBePersisted is the sink-failure
// contract: never a silent allow, and never a softened deny.
func TestDecideDegradesWhenTheRecordCannotBePersisted(t *testing.T) {
	cases := []struct {
		name           string
		event          string
		failMode       FailMode
		wantVerdict    Verdict
		wantPermission string
	}{
		{
			name:           "allow escalates to ask",
			event:          `{"tool_name":"Bash","tool_input":{"command":"git status"}}`,
			failMode:       FailModeAsk,
			wantVerdict:    VerdictRequireApproval,
			wantPermission: "ask",
		},
		{
			name:           "deny stays deny",
			event:          `{"tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}`,
			failMode:       FailModeAsk,
			wantVerdict:    VerdictDeny,
			wantPermission: "deny",
		},
		{
			name:           "fail-open still never silently allows an unrecorded call",
			event:          `{"tool_name":"Bash","tool_input":{"command":"git status"}}`,
			failMode:       FailModeOpen,
			wantVerdict:    VerdictRequireApproval,
			wantPermission: "ask",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A regular file where the record directory belongs makes every
			// write fail, with no stubbing of the sink itself.
			root := t.TempDir()
			blocked := filepath.Join(root, "hook")
			if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
				t.Fatalf("seed blocker: %v", err)
			}
			cfg := Config{Dir: blocked, FailMode: tc.failMode, BoundaryVersion: "v-test"}
			result := Decide(cfg, strings.NewReader(tc.event))
			if result.SinkError == nil {
				t.Fatal("expected a sink error")
			}
			if result.Record != nil {
				t.Fatal("a failed sink must not report a persisted record")
			}
			if result.Verdict != tc.wantVerdict {
				t.Fatalf("verdict = %q, want %q", result.Verdict, tc.wantVerdict)
			}
			if got := permissionOf(t, result.Stdout); got != tc.wantPermission {
				t.Fatalf("permissionDecision = %q, want %q", got, tc.wantPermission)
			}
			if !strings.Contains(result.Reason, "could not persist the decision record") {
				t.Fatalf("reason %q does not explain the degradation", result.Reason)
			}
		})
	}
}

// TestDecideDegradationReasonOmitsTheRecordPath keeps the operator's record
// location out of the text handed back to the model and written into every later
// record: the agent learns that recording failed, not where records live.
func TestDecideDegradationReasonOmitsTheRecordPath(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "UNIQUE-RECORD-DIR-MARKER")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	result := Decide(Config{Dir: blocked, BoundaryVersion: "v-test"},
		strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git status"}}`))
	if result.SinkError == nil {
		t.Fatal("expected a sink error")
	}
	for _, artifact := range []string{result.Reason, string(result.Stdout)} {
		if strings.Contains(artifact, "UNIQUE-RECORD-DIR-MARKER") || strings.Contains(artifact, root) {
			t.Fatalf("degradation text leaked the record path:\n%s", artifact)
		}
	}
	if !strings.Contains(result.SinkError.Error(), blocked) {
		t.Fatalf("SinkError = %v, want the full path for diagnostics", result.SinkError)
	}
}

// TestDecideCommandRouteDeniesGovernanceControlSurfaces is the self-protection
// property, checked on the route that did not have it. The edit route denies
// `.claude/settings.json` as E7; the command route reached the same files through
// `cp`, `mv`, and shell redirection and let them through, so an agent could
// rewrite the gate its own next tool call passes through.
func TestDecideCommandRouteDeniesGovernanceControlSurfaces(t *testing.T) {
	for _, command := range []string{
		"cp /tmp/evil .claude/settings.json",
		"cp /tmp/evil .claude/settings.local.json",
		"mv /tmp/evil.json .claude/settings.json",
		"cp /tmp/evil.sh .claude/hooks/pretooluse.sh",
		"touch .boundary/hook/decision-records.jsonl",
		"cat /tmp/evil.json > .claude/settings.json",
		"ls -R / > /Users/me/proj/.claude/settings.json",
		"cp /tmp/evil.sh integrations/claude-code/pretooluse-boundary.sh",
		"git status && cp /tmp/evil .claude/settings.json",
		"cp /tmp/evil .CLAUDE/Settings.json",
	} {
		t.Run(command, func(t *testing.T) {
			dir := t.TempDir()
			event, err := json.Marshal(map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]string{"command": command},
			})
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			result := decideString(t, dir, string(event))
			if result.Fault != nil {
				t.Fatalf("unexpected fault: %v", result.Fault)
			}
			if result.Verdict != VerdictDeny {
				t.Fatalf("verdict = %q, want deny (reason %q)", result.Verdict, result.Reason)
			}
			if got := permissionOf(t, result.Stdout); got != "deny" {
				t.Fatalf("permissionDecision = %q, want deny", got)
			}
			if result.Record == nil || result.Record.Action != "deny" {
				t.Fatalf("record did not carry the enforced deny: %#v", result.Record)
			}
		})
	}
}

// The guard is about WRITE classes. Reading, listing, or verifying a control
// surface keeps the verdict its own class earns, so the fix does not brick an
// operator inspecting their own decision records.
func TestDecideCommandRouteLeavesControlSurfaceReadsAlone(t *testing.T) {
	for _, tc := range []struct {
		command string
		want    Verdict
	}{
		{"cat .claude/settings.json", VerdictAllow},
		{"ls .boundary/hook/records", VerdictAllow},
		{"cp notes.txt backup.txt", VerdictWarn},
		// Argument position is not analyzed, so a control surface named as the
		// SOURCE of a write is refused too. Pinned so the over-refusal is a
		// known cost rather than a surprise.
		{"cp .claude/settings.json backup.json", VerdictDeny},
	} {
		t.Run(tc.command, func(t *testing.T) {
			dir := t.TempDir()
			event, err := json.Marshal(map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]string{"command": tc.command},
			})
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			result := decideString(t, dir, string(event))
			if result.Verdict != tc.want {
				t.Fatalf("verdict = %q, want %q (reason %q)", result.Verdict, tc.want, result.Reason)
			}
		})
	}
}

// TestDecideEditRouteDeniesAnAbsoluteTargetOutsideTheProject is finding-1 end to
// end: Claude Code's edit tools always pass ABSOLUTE paths, so a write outside
// the project is the normal shape of an escape, not an edge case.
func TestDecideEditRouteDeniesAnAbsoluteTargetOutsideTheProject(t *testing.T) {
	for _, path := range []string{
		"/etc/passwd",
		"/usr/local/bin/boundary",
		"/home/me/.zshrc",
	} {
		t.Run(path, func(t *testing.T) {
			dir := t.TempDir()
			event, err := json.Marshal(map[string]any{
				"tool_name":  "Write",
				"tool_input": map[string]string{"file_path": path},
			})
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			cfg := Config{Dir: dir, ProjectRoot: "/home/me/proj", BoundaryVersion: "v-test"}
			result := Decide(cfg, strings.NewReader(string(event)))
			if result.Verdict != VerdictDeny {
				t.Fatalf("verdict = %q, want deny (reason %q)", result.Verdict, result.Reason)
			}
			if got := permissionOf(t, result.Stdout); got != "deny" {
				t.Fatalf("permissionDecision = %q, want deny", got)
			}
		})
	}
}

// The same route must keep classifying an absolute target INSIDE the project by
// its position in the project, including the leading-component classes the old
// leading-slash strip destroyed.
func TestDecideEditRouteClassifiesAnAbsoluteTargetInsideTheProject(t *testing.T) {
	for _, tc := range []struct {
		path string
		want Verdict
	}{
		{"/home/me/proj/.git/hooks/pre-commit", VerdictDeny},
		{"/home/me/proj/.claude/settings.json", VerdictDeny},
		{"/home/me/proj/config/.env", VerdictDeny},
		{"/home/me/proj/internal/thing.go", VerdictRequireApproval},
		{"/home/me/proj/docs/README.md", VerdictAllow},
	} {
		t.Run(tc.path, func(t *testing.T) {
			dir := t.TempDir()
			event, err := json.Marshal(map[string]any{
				"tool_name":  "Write",
				"tool_input": map[string]string{"file_path": tc.path},
			})
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			cfg := Config{Dir: dir, ProjectRoot: "/home/me/proj", BoundaryVersion: "v-test"}
			result := Decide(cfg, strings.NewReader(string(event)))
			if result.Verdict != tc.want {
				t.Fatalf("verdict = %q, want %q (reason %q)", result.Verdict, tc.want, result.Reason)
			}
		})
	}
}

// The project root is Boundary's own, never the event's: a producer that could
// choose the root could relativize any path into scope.
func TestDecideEditRouteIgnoresTheEventCWD(t *testing.T) {
	dir := t.TempDir()
	event := `{"tool_name":"Write","cwd":"/","tool_input":{"file_path":"/etc/passwd"}}`
	cfg := Config{Dir: dir, ProjectRoot: "/home/me/proj", BoundaryVersion: "v-test"}
	result := Decide(cfg, strings.NewReader(event))
	if result.Verdict != VerdictDeny {
		t.Fatalf("verdict = %q, want deny; the event's cwd must not widen scope (reason %q)",
			result.Verdict, result.Reason)
	}
}

// TestDecideRecordBindsToItsGovernedRequest checks the request_hash cross-check
// verify-record performs with --request, using the same governance function.
func TestDecideRecordBindsToItsGovernedRequest(t *testing.T) {
	request := &governance.GovernanceRequest{
		RequestID: "req-1",
		Transport: TransportHook,
		ToolName:  "rm",
		Action:    "C4",
	}
	record := buildRecord(recordInput{
		Config:  Config{}.withDefaults(),
		Request: request,
		Tool:    "Bash",
		Verdict: VerdictDeny,
		Reason:  "denied",
	})
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if err := governance.VerifyDecisionRecord(record, raw, "", ""); err != nil {
		t.Fatalf("record does not bind to its request: %v", err)
	}
}

func TestDecideClipsAnOverlongReason(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("averylongargument ", 200)
	result := decideString(t, dir,
		`{"tool_name":"Bash","tool_input":{"command":"rm -rf `+long+`"}}`)
	if result.Record == nil {
		t.Fatalf("no record; fault=%v", result.Fault)
	}
	if got := len([]rune(result.Record.Reason)); got > maxReasonLen {
		t.Fatalf("record reason is %d runes, want at most %d", got, maxReasonLen)
	}
	if !strings.HasSuffix(result.Record.Reason, "...") {
		t.Fatalf("clipped reason must mark the cut: %q", result.Record.Reason)
	}
}

func TestSanitizeLabelBoundsUntrustedInput(t *testing.T) {
	if got := sanitizeLabel("../../etc/passwd\n\x00", 128); strings.ContainsAny(got, "\n\x00") {
		t.Fatalf("sanitizeLabel kept control characters: %q", got)
	}
	if got := sanitizeLabel(strings.Repeat("a", 500), 10); len(got) != 10 {
		t.Fatalf("sanitizeLabel length = %d, want 10", len(got))
	}
	if got := sanitizeLabel("   ", 10); got != "" {
		t.Fatalf("sanitizeLabel(blank) = %q", got)
	}
}
