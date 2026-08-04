package securegithub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

type captureDecisionSink struct {
	mu      sync.Mutex
	records []governance.DecisionRecordV1
}

func (s *captureDecisionSink) WriteDecision(_ context.Context, record governance.DecisionRecordV1) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

func (s *captureDecisionSink) latest(t *testing.T) governance.DecisionRecordV1 {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) == 0 {
		t.Fatal("no decision record captured")
	}
	return s.records[len(s.records)-1]
}

type captureForwardSink struct {
	mu     sync.Mutex
	events []MCPForwardEvent
}

type failFirstForwardSink struct {
	mu    sync.Mutex
	calls int
}

func (s *failFirstForwardSink) WriteForwardEvent(context.Context, MCPForwardEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls == 1 {
		return errors.New("forward evidence unavailable")
	}
	return nil
}

func (s *captureForwardSink) WriteForwardEvent(_ context.Context, event MCPForwardEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *captureForwardSink) latest(t *testing.T) MCPForwardEvent {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		t.Fatal("no forward event captured")
	}
	return s.events[len(s.events)-1]
}

func (s *captureForwardSink) snapshot() []MCPForwardEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]MCPForwardEvent{}, s.events...)
}

type upstreamCapture struct {
	mu          sync.Mutex
	calls       int
	toolCalls   []string
	authHeaders []string
	toolHeaders []string
	readFails   bool
}

func (u *upstreamCapture) handler(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	_ = json.Unmarshal(body, &msg)
	u.mu.Lock()
	u.calls++
	u.toolCalls = append(u.toolCalls, msg.Params.Name)
	u.authHeaders = append(u.authHeaders, req.Header.Get("Authorization"))
	u.toolHeaders = append(u.toolHeaders, req.Header.Get("X-MCP-Tools"))
	readFails := u.readFails
	u.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch msg.Method {
	case "initialize":
		w.Header().Set(mcpSessionHeader, "upstream-session")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"github-mcp","version":"test"}}}`))
	case "tools/list":
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"issue_read"},{"name":"create_or_update_file"},{"name":"delete_file"}]}}`))
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/call":
		if msg.Params.Name == GitHubIssueReadTool && readFails {
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"read failed"}],"isError":true}}`, msg.ID)
			return
		}
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"configured issue"}]}}`, msg.ID)
	default:
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":9,"result":{}}`))
	}
}

func (u *upstreamCapture) snapshot() (int, []string, []string, []string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.calls, append([]string{}, u.toolCalls...), append([]string{}, u.authHeaders...), append([]string{}, u.toolHeaders...)
}

func (u *upstreamCapture) setReadFails(value bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.readFails = value
}

func TestMCPRouteLifecycleAndWriteAfterTaintDenial(t *testing.T) {
	upstream := &upstreamCapture{}
	server := httptest.NewServer(http.HandlerFunc(upstream.handler))
	defer server.Close()
	decisions := &captureDecisionSink{}
	forwardEvents := &captureForwardSink{}
	route, err := NewMCPRoute(MCPRouteConfig{
		UpstreamURL: server.URL, Token: "test-token-never-returned",
		SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
		TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
		AgentID: "claude-code", TenantID: "test", DecisionSink: decisions, ForwardSink: forwardEvents,
	})
	if err != nil {
		t.Fatal(err)
	}

	init := postMCP(t, route, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"claude-code","version":"test"}}}`, nil)
	if init.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, body = %s", init.Code, init.Body.String())
	}
	sessionID := init.Header().Get(mcpSessionHeader)
	if sessionID == "" || sessionID == "upstream-session" {
		t.Fatalf("Boundary did not mint an independent local session: %q", sessionID)
	}

	notification := postMCP(t, route, sessionID, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
	if notification.Code != http.StatusAccepted || notification.Body.Len() != 0 {
		t.Fatalf("notification status/body = %d/%q", notification.Code, notification.Body.String())
	}

	tools := postMCP(t, route, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, nil)
	if strings.Contains(tools.Body.String(), "delete_file") || !strings.Contains(tools.Body.String(), GitHubIssueReadTool) || !strings.Contains(tools.Body.String(), GitHubProtectedWriteTool) {
		t.Fatalf("tools/list was not filtered: %s", tools.Body.String())
	}
	ping := postMCP(t, route, sessionID, `{"jsonrpc":"2.0","id":20,"method":"ping","params":{}}`, nil)
	if ping.Code != http.StatusOK {
		t.Fatalf("ping status = %d", ping.Code)
	}
	beforeUnknown, _, _, _ := upstream.snapshot()
	unknownMethod := postMCP(t, route, sessionID, `{"jsonrpc":"2.0","id":21,"method":"github/custom","params":{}}`, nil)
	if !strings.Contains(unknownMethod.Body.String(), `"code":-32601`) {
		t.Fatalf("custom method was not rejected: %s", unknownMethod.Body.String())
	}
	unknownNotification := postMCP(t, route, sessionID, `{"jsonrpc":"2.0","method":"notifications/custom","params":{}}`, nil)
	if unknownNotification.Code != http.StatusAccepted || unknownNotification.Body.Len() != 0 {
		t.Fatalf("custom notification rejection = %d/%q", unknownNotification.Code, unknownNotification.Body.String())
	}
	afterUnknown, _, _, _ := upstream.snapshot()
	if afterUnknown != beforeUnknown {
		t.Fatalf("unknown methods reached upstream: before=%d after=%d", beforeUnknown, afterUnknown)
	}

	before, _, _, _ := upstream.snapshot()
	disallowedSource := postMCP(t, route, sessionID, toolCall(30, GitHubIssueReadTool, map[string]any{
		"method": "get", "owner": "other-org", "repo": "untrusted-issues", "issue_number": 17,
	}), nil)
	assertDenied(t, disallowedSource, "secure-github-source-allowlist")
	after, _, _, _ := upstream.snapshot()
	if after != before {
		t.Fatalf("disallowed source reached upstream: before=%d after=%d", before, after)
	}
	beforeFractional, _, _, _ := upstream.snapshot()
	fractionalSource := postMCP(t, route, sessionID, toolCall(301, GitHubIssueReadTool, map[string]any{
		"method": "get", "owner": "public-org", "repo": "untrusted-issues", "issue_number": 17.5,
	}), nil)
	assertDenied(t, fractionalSource, "secure-github-source-allowlist")
	afterFractional, _, _, _ := upstream.snapshot()
	if afterFractional != beforeFractional {
		t.Fatalf("fractional source issue reached upstream: before=%d after=%d", beforeFractional, afterFractional)
	}

	upstream.setReadFails(true)
	readFailure := postMCP(t, route, sessionID, toolCall(31, GitHubIssueReadTool, configuredRead()), nil)
	if !strings.Contains(readFailure.Body.String(), `"isError":true`) {
		t.Fatalf("upstream read failure was not relayed: %s", readFailure.Body.String())
	}
	writeBeforeTaint := postMCP(t, route, sessionID, toolCall(32, GitHubProtectedWriteTool, configuredWrite()), nil)
	assertDenied(t, writeBeforeTaint, "secure-github-read-before-write")

	upstream.setReadFails(false)
	readSuccess := postMCP(t, route, sessionID, toolCall(33, GitHubIssueReadTool, configuredRead()), nil)
	if readSuccess.Code != http.StatusOK || !strings.Contains(readSuccess.Body.String(), "configured issue") {
		t.Fatalf("configured issue read failed: %d %s", readSuccess.Code, readSuccess.Body.String())
	}
	beforeWrite, _, _, _ := upstream.snapshot()
	deniedWrite := postMCP(t, route, sessionID, toolCall(34, GitHubProtectedWriteTool, configuredWrite()), nil)
	assertDenied(t, deniedWrite, "secure-github-write-after-taint")
	afterWrite, toolCalls, authHeaders, toolHeaders := upstream.snapshot()
	if afterWrite != beforeWrite {
		t.Fatalf("protected write reached upstream: before=%d after=%d", beforeWrite, afterWrite)
	}
	for _, tool := range toolCalls {
		if tool == GitHubProtectedWriteTool {
			t.Fatal("protected mutation appeared in independent upstream capture")
		}
	}
	for _, auth := range authHeaders {
		if auth != "Bearer test-token-never-returned" {
			t.Fatalf("upstream authorization = %q", auth)
		}
	}
	for _, toolsHeader := range toolHeaders {
		if toolsHeader != GitHubIssueReadTool+","+GitHubProtectedWriteTool {
			t.Fatalf("upstream X-MCP-Tools = %q", toolsHeader)
		}
	}
	if strings.Contains(deniedWrite.Body.String(), "test-token-never-returned") {
		t.Fatal("upstream token leaked into downstream denial")
	}

	record := decisions.latest(t)
	if record.Action != "deny" || record.MatchedRule != "secure-github-write-after-taint" || record.ExecutionClaim == nil || record.ExecutionClaim.UpstreamCalled {
		t.Fatalf("incomplete deny record: %+v", record)
	}
	if err := governance.VerifyDecisionRecord(record, nil, "", ""); err != nil {
		t.Fatalf("deny decision record does not verify: %v", err)
	}
	canary := forwardEvents.latest(t)
	if canary.RequestID != recordRequestID(t, deniedWrite.Body.Bytes()) || canary.Forwarded || !canary.Mutation || canary.Outcome != "blocked_before_forward" {
		t.Fatalf("independent canary does not prove no-forward: %+v", canary)
	}
	var sawRead, sawBlockedMutation bool
	for _, event := range forwardEvents.snapshot() {
		if event.Tool == GitHubIssueReadTool && event.Outcome == "forwarded_success" && event.Forwarded && !event.Mutation {
			sawRead = true
		}
		if event.Tool == GitHubProtectedWriteTool && event.Outcome == "blocked_before_forward" && !event.Forwarded && event.Mutation {
			sawBlockedMutation = true
		}
		if event.Mutation && event.Forwarded {
			t.Fatalf("forward log contains a mutation request: %+v", event)
		}
	}
	if !sawRead || !sawBlockedMutation {
		t.Fatalf("forward log lacks correlated read/block events: %+v", forwardEvents.snapshot())
	}

	disallowedSink := postMCP(t, route, sessionID, toolCall(35, GitHubProtectedWriteTool, map[string]any{
		"owner": "private-org", "repo": "protected-repo", "branch": "dev", "path": "README.md", "content": "x", "message": "x",
	}), nil)
	assertDenied(t, disallowedSink, "secure-github-sink-allowlist")
	unknownTool := postMCP(t, route, sessionID, toolCall(36, "delete_file", map[string]any{}), nil)
	assertDenied(t, unknownTool, "secure-github-tool-allowlist")
	beforeSlashTool, _, _, _ := upstream.snapshot()
	slashTool := postMCP(t, route, sessionID, toolCall(37, "delete/file", map[string]any{}), nil)
	assertDenied(t, slashTool, "secure-github-tool-allowlist")
	afterSlashTool, _, _, _ := upstream.snapshot()
	if afterSlashTool != beforeSlashTool {
		t.Fatalf("slash-containing unknown tool reached upstream: before=%d after=%d", beforeSlashTool, afterSlashTool)
	}
	slashRecord := decisions.latest(t)
	slashCanary := forwardEvents.latest(t)
	if slashRecord.MatchedRule != "secure-github-tool-allowlist" || slashCanary.RequestID != recordRequestID(t, slashTool.Body.Bytes()) ||
		slashCanary.Tool != "delete/file" || slashCanary.Forwarded || slashCanary.Outcome != "blocked_before_forward" {
		t.Fatalf("slash-containing tool lacks canonical deny evidence: record=%+v canary=%+v", slashRecord, slashCanary)
	}
	if err := governance.VerifyDecisionRecord(slashRecord, nil, "", ""); err != nil {
		t.Fatalf("slash-containing tool decision record does not verify: %v", err)
	}
}

func TestMCPRouteIssueReadJSONAndSSEOutcomes(t *testing.T) {
	type outcome struct {
		name    string
		result  func(json.RawMessage) string
		tainted bool
	}
	outcomes := []outcome{
		{
			name: "success",
			result: func(id json.RawMessage) string {
				return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"configured issue"}]}}`, id)
			},
			tainted: true,
		},
		{
			name: "tool-is-error",
			result: func(id json.RawMessage) string {
				return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"read failed"}],"isError":true}}`, id)
			},
		},
		{
			name: "json-rpc-error",
			result: func(id json.RawMessage) string {
				return fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"error":{"code":-32000,"message":"read failed"}}`, id)
			},
		},
		{
			name: "mismatched-id",
			result: func(json.RawMessage) string {
				return `{"jsonrpc":"2.0","id":999,"result":{"content":[{"type":"text","text":"wrong request"}]}}`
			},
		},
	}
	for _, format := range []string{"application/json; charset=utf-8", "text/event-stream; charset=utf-8"} {
		for _, test := range outcomes {
			name := strings.Split(format, ";")[0] + "/" + test.name
			t.Run(name, func(t *testing.T) {
				forwardEvents := &captureForwardSink{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					body, _ := io.ReadAll(req.Body)
					var msg mcpWireMessage
					_ = json.Unmarshal(body, &msg)
					w.Header().Set("Content-Type", format)
					w.Header().Set("Cache-Control", "no-cache, no-transform")
					var response string
					switch msg.Method {
					case "initialize":
						response = fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"github-mcp","version":"test"}}}`, msg.ID)
					case "tools/list":
						response = fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"tools":[{"name":"issue_read"},{"name":"create_or_update_file"},{"name":"delete_file"}]}}`, msg.ID)
					case "tools/call":
						response = test.result(msg.ID)
					}
					if baseMediaType(format) == "text/event-stream" {
						_, _ = fmt.Fprintf(w, "event: message\nid: upstream-event\ndata: %s\n\n", response)
						return
					}
					_, _ = io.WriteString(w, response)
				}))
				defer server.Close()
				route, err := NewMCPRoute(MCPRouteConfig{
					UpstreamURL: server.URL, Token: "fake", SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
					TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
					DecisionSink: &captureDecisionSink{}, ForwardSink: forwardEvents,
				})
				if err != nil {
					t.Fatal(err)
				}
				init := postMCP(t, route, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`, nil)
				sessionID := init.Header().Get(mcpSessionHeader)
				if sessionID == "" || baseMediaType(init.Header().Get("Content-Type")) != baseMediaType(format) {
					t.Fatalf("initialize format/session = %q/%q", init.Header().Get("Content-Type"), sessionID)
				}
				tools := postMCP(t, route, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, nil)
				if baseMediaType(tools.Header().Get("Content-Type")) != baseMediaType(format) || strings.Contains(tools.Body.String(), "delete_file") {
					t.Fatalf("filtered tools response = %q/%s", tools.Header().Get("Content-Type"), tools.Body.String())
				}
				read := postMCP(t, route, sessionID, toolCall(3, GitHubIssueReadTool, configuredRead()), nil)
				if baseMediaType(read.Header().Get("Content-Type")) != baseMediaType(format) || read.Header().Get("Cache-Control") != "no-cache, no-transform" {
					t.Fatalf("read headers were not preserved: %v", read.Header())
				}
				expectedReadBody := test.result(json.RawMessage("3"))
				if baseMediaType(format) == "text/event-stream" {
					expectedReadBody = "event: message\nid: upstream-event\ndata: " + expectedReadBody + "\n\n"
				}
				if read.Body.String() != expectedReadBody {
					t.Fatalf("read body was not preserved:\nwant %q\n got %q", expectedReadBody, read.Body.String())
				}
				write := postMCP(t, route, sessionID, toolCall(4, GitHubProtectedWriteTool, configuredWrite()), nil)
				if test.tainted {
					assertDenied(t, write, "secure-github-write-after-taint")
					var sawRead bool
					for _, event := range forwardEvents.snapshot() {
						if event.Tool == GitHubIssueReadTool && event.Outcome == "forwarded_success" && event.SessionID == sessionID {
							sawRead = true
						}
					}
					if !sawRead {
						t.Fatalf("successful %s read lacks correlated forward evidence", format)
					}
				} else {
					assertDenied(t, write, "secure-github-read-before-write")
					for _, event := range forwardEvents.snapshot() {
						if event.Tool == GitHubIssueReadTool && event.Outcome == "forwarded_success" {
							t.Fatalf("unsuccessful read emitted success evidence: %+v", event)
						}
					}
				}
			})
		}
	}
}

func TestMCPRouteProtectedWriteVerdictComesFromCanonicalPipeline(t *testing.T) {
	decisions := &captureDecisionSink{}
	forwardEvents := &captureForwardSink{}
	route, err := NewMCPRoute(MCPRouteConfig{
		UpstreamURL: "http://upstream.invalid", Token: "fake", SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
		TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
		DecisionSink: decisions, ForwardSink: forwardEvents,
	})
	if err != nil {
		t.Fatal(err)
	}
	route.sessions["pipeline-session"] = &mcpRouteSession{tainted: true}
	governed, err := route.governTool(context.Background(), "pipeline-session", GitHubProtectedWriteTool, configuredWrite())
	if err != nil {
		t.Fatal(err)
	}
	if governed.decision.Allowed() || governed.decision.MatchedRule != "secure-github-write-after-taint" {
		t.Fatalf("canonical pipeline verdict = %+v", governed.decision)
	}
	if governed.record.MatchedRule != governed.decision.MatchedRule || governed.record.PolicyBundleHash != "secure-github-mcp-v1" {
		t.Fatalf("canonical pipeline decision record = %+v", governed.record)
	}
	if route.auditor.Len() != 0 {
		t.Fatalf("canonical pipeline audit capture was not consumed: %d", route.auditor.Len())
	}
}

func TestMCPRouteMalformedOrAmbiguousSSEDoesNotTaint(t *testing.T) {
	responses := map[string]string{
		"malformed": "event: message\ndata: not-json\n\n",
		"ambiguous": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":3,\"result\":{}}\n\n" +
			"event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":3,\"result\":{}}\n\n",
	}
	for name, response := range responses {
		t.Run(name, func(t *testing.T) {
			forwardEvents := &captureForwardSink{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, response)
			}))
			defer server.Close()
			route, err := NewMCPRoute(MCPRouteConfig{
				UpstreamURL: server.URL, Token: "fake", SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
				TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
				DecisionSink: &captureDecisionSink{}, ForwardSink: forwardEvents,
			})
			if err != nil {
				t.Fatal(err)
			}
			route.sessions["local-session"] = &mcpRouteSession{}
			read := postMCP(t, route, "local-session", toolCall(3, GitHubIssueReadTool, configuredRead()), nil)
			if read.Header().Get("Content-Type") != "text/event-stream" || read.Body.String() != response {
				t.Fatalf("SSE response was not preserved: %q/%q", read.Header().Get("Content-Type"), read.Body.String())
			}
			write := postMCP(t, route, "local-session", toolCall(4, GitHubProtectedWriteTool, configuredWrite()), nil)
			assertDenied(t, write, "secure-github-read-before-write")
			for _, event := range forwardEvents.snapshot() {
				if event.Tool == GitHubIssueReadTool && event.Outcome == "forwarded_success" {
					t.Fatalf("invalid SSE tainted the session: %+v", event)
				}
			}
		})
	}
}

func TestMCPRouteEvidenceFailureRetainsValidatedReadTaint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		var msg mcpWireMessage
		_ = json.Unmarshal(body, &msg)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"configured issue"}]}}`, msg.ID)
	}))
	defer server.Close()
	route, err := NewMCPRoute(MCPRouteConfig{
		UpstreamURL: server.URL, Token: "fake", SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
		TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
		DecisionSink: &captureDecisionSink{}, ForwardSink: &failFirstForwardSink{},
	})
	if err != nil {
		t.Fatal(err)
	}
	route.sessions["local-session"] = &mcpRouteSession{}
	read := postMCP(t, route, "local-session", toolCall(3, GitHubIssueReadTool, configuredRead()), nil)
	if !strings.Contains(read.Body.String(), "Boundary forwarder evidence unavailable") || strings.Contains(read.Body.String(), "configured issue") {
		t.Fatalf("evidence failure did not fail closed: %s", read.Body.String())
	}
	write := postMCP(t, route, "local-session", toolCall(4, GitHubProtectedWriteTool, configuredWrite()), nil)
	assertDenied(t, write, "secure-github-write-after-taint")
}

func TestMCPRouteUpstreamOutageTimeoutAndCancellationDoNotTaint(t *testing.T) {
	tests := []struct {
		name   string
		client *http.Client
		server http.Handler
		cancel bool
	}{
		{name: "outage", client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}},
		{name: "timeout", server: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { time.Sleep(100 * time.Millisecond) })},
		{name: "cancellation", server: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { time.Sleep(100 * time.Millisecond) }), cancel: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstreamURL := "http://upstream.invalid"
			var server *httptest.Server
			if test.server != nil {
				server = httptest.NewServer(test.server)
				defer server.Close()
				upstreamURL = server.URL
			}
			route, err := NewMCPRoute(MCPRouteConfig{
				UpstreamURL: upstreamURL, Token: "fake", HTTPClient: test.client, Timeout: 15 * time.Millisecond,
				SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
				TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
				DecisionSink: &captureDecisionSink{}, ForwardSink: &captureForwardSink{},
			})
			if err != nil {
				t.Fatal(err)
			}
			route.sessions["local-session"] = &mcpRouteSession{}
			ctx := context.Background()
			if test.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			read := postMCP(t, route, "local-session", toolCall(1, GitHubIssueReadTool, configuredRead()), ctx)
			if !strings.Contains(read.Body.String(), "upstream unavailable") {
				t.Fatalf("expected fail-closed upstream error, got %s", read.Body.String())
			}
			write := postMCP(t, route, "local-session", toolCall(2, GitHubProtectedWriteTool, configuredWrite()), nil)
			assertDenied(t, write, "secure-github-read-before-write")
		})
	}
}

func TestMCPRouteRejectsMissingSessionAndNonLocalOrigin(t *testing.T) {
	route, err := NewMCPRoute(MCPRouteConfig{
		UpstreamURL: "http://upstream.invalid", Token: "fake",
		SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
		TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
		DecisionSink: &captureDecisionSink{}, ForwardSink: &captureForwardSink{},
	})
	if err != nil {
		t.Fatal(err)
	}
	missing := postMCP(t, route, "", `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, nil)
	if !strings.Contains(missing.Body.String(), "missing Boundary MCP session") {
		t.Fatalf("missing session was not rejected: %s", missing.Body.String())
	}
	origin := postMCP(t, route, "session", `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, nil, "https://attacker.example")
	if origin.Code != http.StatusForbidden {
		t.Fatalf("non-local Origin status = %d", origin.Code)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func configuredRead() map[string]any {
	return map[string]any{"method": "get", "owner": "public-org", "repo": "untrusted-issues", "issue_number": 17}
}

func configuredWrite() map[string]any {
	return map[string]any{
		"owner": "private-org", "repo": "protected-repo", "branch": "main",
		"path": "protected.txt", "content": "never forwarded", "message": "attempt protected write",
	}
}

func toolCall(id int, name string, args map[string]any) string {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	return string(body)
}

func postMCP(t *testing.T, handler http.Handler, sessionID, body string, ctx context.Context, origins ...string) *httptest.ResponseRecorder {
	t.Helper()
	if ctx == nil {
		ctx = context.Background()
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:8080/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		req.Header.Set(mcpSessionHeader, sessionID)
	}
	if len(origins) > 0 {
		req.Header.Set("Origin", origins[0])
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func assertDenied(t *testing.T, response *httptest.ResponseRecorder, matchedRule string) {
	t.Helper()
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"code":-32001`) || !strings.Contains(response.Body.String(), matchedRule) {
		t.Fatalf("expected %s denial, got status=%d body=%s", matchedRule, response.Code, response.Body.String())
	}
}

func recordRequestID(t *testing.T, body []byte) string {
	t.Helper()
	var response struct {
		Error struct {
			Data struct {
				RequestID string `json:"request_id"`
			} `json:"data"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	return response.Error.Data.RequestID
}
