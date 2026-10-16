package securegithub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/adapterparity"
)

// TestMCPRouteParity runs the stateful Secure GitHub MCP route through the
// ADR-047 parity table. The route builds its own pipeline, so the driver swaps
// in the case pipeline and tees decision events into the route's capture.
func TestMCPRouteParity(t *testing.T) {
	var capture *mcpAuditCapture
	adapterparity.Run(t, adapterparity.Adapter{
		Name: "securegithub-mcp-route",
		Tool: "github." + GitHubIssueReadTool,
		NotApplicable: map[string]string{
			"nil_pipeline": "NewMCPRoute always builds the profile pipeline, so there is no unwired state",
		},
		Audit: func(rec governance.AuditPublisher) governance.AuditPublisher {
			capture = newMCPAuditCapture()
			return teeAudit{rec, capture}
		},
		Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
			var toolCalls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var msg struct {
					ID json.RawMessage `json:"id"`
				}
				_ = json.NewDecoder(req.Body).Decode(&msg)
				toolCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(msg.ID) + `,"result":{"content":[{"type":"text","text":"issue"}]}}`))
			}))
			defer upstream.Close()

			route, err := NewMCPRoute(MCPRouteConfig{
				UpstreamURL: upstream.URL, Token: "fake", SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
				TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
				DecisionSink: &captureDecisionSink{}, ForwardSink: &captureForwardSink{},
			})
			if err != nil {
				t.Fatal(err)
			}
			route.pipeline, route.auditor = p, capture
			route.sessions["parity"] = &mcpRouteSession{}

			params := `{"name":"` + GitHubIssueReadTool + `","arguments":{"method":"get","owner":"public-org","repo":"untrusted-issues","issue_number":17}}`
			if in == adapterparity.Malformed {
				params = `{"name":""}`
			}
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":`+params+`}`)).WithContext(ctx)
			req.Header.Set(mcpSessionHeader, "parity")
			rec := httptest.NewRecorder()
			route.ServeHTTP(rec, req)

			var resp struct {
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Data map[string]any `json:"data"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("route response %q: %v", rec.Body.String(), err)
			}
			out := adapterparity.Outcome{Executed: int(toolCalls.Load())}
			switch {
			case resp.Error != nil:
				out.Action, _ = resp.Error.Data["action"].(string)
			case len(resp.Result) > 0:
				// A forwarded result carries no verdict field. The caller sees
				// the call run, which is an allow.
				out.Action = "allow"
			}
			return out
		},
	})
}

type teeAudit []governance.AuditPublisher

func (t teeAudit) Publish(ctx context.Context, e governance.AuditEvent) {
	for _, p := range t {
		p.Publish(ctx, e)
	}
}
