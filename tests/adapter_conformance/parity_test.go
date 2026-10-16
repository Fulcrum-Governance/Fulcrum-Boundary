package adapter_conformance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/a2a"
	"github.com/fulcrum-governance/fulcrum-boundary/adapters/cli"
	"github.com/fulcrum-governance/fulcrum-boundary/adapters/codeexec"
	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
	"github.com/fulcrum-governance/fulcrum-boundary/adapters/mcp"
	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
	"github.com/fulcrum-governance/fulcrum-boundary/adapters/webhook"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/adapterparity"
)

// TestAdapterParity runs the shared ADR-047 table through every adapter in
// the root module. The gRPC adapter is its own module and runs the same
// table in adapters/grpc/parity_test.go.
func TestAdapterParity(t *testing.T) {
	adapterparity.Run(t,
		mcpParity(),
		secureGitHubParity(),
		a2aParity(),
		cliParity(),
		codeExecParity(),
		webhookParity(webhook.ModeExecution),
		webhookParity(webhook.ModeInformational),
		managedAgentsParity(),
	)
}

func mcpParity() adapterparity.Adapter {
	return adapterparity.Adapter{Name: "mcp", Tool: "parity_tool", Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
		upstream := &mcpUpstream{}
		body := `{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"parity_tool","arguments":{"x":1}}}`
		if in == adapterparity.Malformed {
			body = `{"jsonrpc":"2.0","id":"1","method":"tools/call","params":"not-an-object"}`
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)).WithContext(ctx)
		serveRecovering(t, mcp.NewGateway(p, upstream, "tenant-parity"), rec, req)
		return adapterparity.Outcome{Action: rec.Header().Get(governance.HeaderGovernanceAction), Executed: int(upstream.calls.Load())}
	}}
}

type mcpUpstream struct{ calls atomic.Int32 }

func (u *mcpUpstream) Forward(context.Context, []byte) (*governance.ToolResponse, error) {
	u.calls.Add(1)
	return &governance.ToolResponse{Content: []byte(`{"jsonrpc":"2.0","id":"1","result":{"content":[]}}`), ContentType: "application/json"}, nil
}

func secureGitHubParity() adapterparity.Adapter {
	return adapterparity.Adapter{Name: "securegithub", Tool: "github.get_issue", NotApplicable: map[string]string{
		"nil_pipeline": "NewAdapter installs the profile's own policy pipeline when given nil, so there is no unwired state",
	}, Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
		upstream := &gitHubUpstream{}
		var raw any = securegithub.ToolCall{ID: "1", ToolName: "get_issue", Arguments: map[string]any{
			"owner": securegithub.DefaultOwner, "repo": securegithub.DefaultRepo, "author_association": "MEMBER",
		}}
		if in == adapterparity.Malformed {
			raw = []byte(`{not json`)
		}
		res, err := securegithub.NewAdapter(securegithub.Config{}, p, upstream).GovernToolCall(ctx, raw)
		out := adapterparity.Outcome{Executed: int(upstream.calls.Load())}
		if err == nil && res != nil && res.DecisionRecord.RecordID != "" {
			out.Record = &res.DecisionRecord
		}
		switch {
		case err != nil || res == nil:
		case res.Response.Error != nil:
			out.Action, _ = res.Response.Error.Data["action"].(string)
		case res.Response.Result != nil:
			out.Action = res.Response.Result.Governance["action"]
		}
		return out
	}}
}

type gitHubUpstream struct{ calls atomic.Int32 }

func (u *gitHubUpstream) CallGitHub(context.Context, securegithub.ToolCall, securegithub.Envelope) (*securegithub.MCPResult, error) {
	u.calls.Add(1)
	return &securegithub.MCPResult{Content: []securegithub.MCPContent{{Type: "text", Text: "issue body"}}}, nil
}

func a2aParity() adapterparity.Adapter {
	return adapterparity.Adapter{Name: "a2a", Tool: "parity_tool", Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
		forwarder := &a2a.MemoryForwarder{}
		var raw any = a2a.TaskEnvelope{TaskID: "task-1", SenderAgentID: "agent-1", Receiver: "worker-1", Action: "parity_tool", Input: map[string]any{"text": "hello"}}
		if in == adapterparity.Malformed {
			raw = []byte(`{not json`)
		}
		resp, err := a2a.NewForwardingAdapter("tenant-parity", forwarder).GovernTask(ctx, raw, p)
		out := adapterparity.Outcome{Executed: len(forwarder.Snapshot())}
		if err == nil && resp != nil && resp.Governance != nil {
			out.Action = resp.Governance.Action
		}
		return out
	}}
}

func cliParity() adapterparity.Adapter {
	return adapterparity.Adapter{Name: "cli", Tool: "parity_tool", Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
		executor := &countingExecutor{}
		input := cli.CommandInput{Command: "parity_tool --flag"}
		if in == adapterparity.Malformed {
			input.Command = `parity_tool "unterminated`
		}
		resp, err := cli.NewAdapterWithExecutor("tenant-parity", executor).GovernCommand(ctx, input, p)
		out := adapterparity.Outcome{Executed: int(executor.calls.Load())}
		if err == nil && resp != nil {
			out.Action = resp.Metadata["x-fulcrum-action"]
		}
		return out
	}}
}

func codeExecParity() adapterparity.Adapter {
	return adapterparity.Adapter{Name: "codeexec", Tool: "code_exec", Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
		executor := &countingExecutor{}
		input := codeexec.CodeExecInput{Code: "print('ok')", Language: "python"}
		if in == adapterparity.Malformed {
			input = codeexec.CodeExecInput{Code: "", Language: "python"}
		}
		resp, err := codeexec.NewAdapterWithExecutor("tenant-parity", executor, codeexec.LocalProcessBoundary("parity")).GovernCode(ctx, input, p)
		out := adapterparity.Outcome{Executed: int(executor.calls.Load())}
		if err == nil && resp != nil {
			out.Action = resp.Metadata["x-fulcrum-action"]
		}
		return out
	}}
}

type countingExecutor struct{ calls atomic.Int32 }

func (e *countingExecutor) Execute(context.Context, *governance.GovernanceRequest) (*governance.ToolResponse, error) {
	e.calls.Add(1)
	return &governance.ToolResponse{Content: []byte("ok\n"), ContentType: "text/plain", Metadata: map[string]string{}}, nil
}

func webhookParity(mode webhook.Mode) adapterparity.Adapter {
	return adapterparity.Adapter{Name: "webhook-" + string(mode), Tool: "parity_tool", Informational: mode == webhook.ModeInformational,
		Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
			var forwarded atomic.Int32
			downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				forwarded.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer downstream.Close()
			body := `{"tool":"parity_tool","agent_id":"agent-1","tenant_id":"tenant-parity","arguments":{"x":1}}`
			if in == adapterparity.Malformed {
				body = `{"tool":""}`
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)).WithContext(ctx)
			serveRecovering(t, webhook.HandlerWithConfig(p, webhook.HandlerConfig{Mode: mode, ForwardURL: downstream.URL}), rec, req)
			return adapterparity.Outcome{Action: rec.Header().Get(governance.HeaderGovernanceAction), Executed: int(forwarded.Load())}
		}}
}

func managedAgentsParity() adapterparity.Adapter {
	return adapterparity.Adapter{Name: "managedagents", Tool: "parity_tool", Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
		forwarder := &managedagents.MemoryForwarder{}
		event := managedagents.Event{ID: "evt-1", Type: managedagents.EventAgentToolUse, AgentID: "agent-1", SessionID: "sess-1", ToolName: "parity_tool"}
		if in == adapterparity.Malformed {
			event = managedagents.Event{ID: "evt-1", Type: "session.status_idle", SessionID: "sess-1"}
		}
		resolver := &managedagents.ToolResolver{Adapter: managedagents.NewProxyAdapter("tenant-parity", forwarder), Pipeline: p, Forwarder: forwarder}
		confirmation, _, err := resolver.Resolve(ctx, event)
		out := adapterparity.Outcome{}
		for _, c := range forwarder.Confirmations {
			if c.Result == managedagents.ConfirmationAllow {
				out.Executed++
			}
		}
		if err == nil && confirmation.Governance != nil {
			out.Action = confirmation.Governance.Action
		}
		return out
	}}
}

// serveRecovering logs a handler panic the way net/http would absorb it: the
// connection drops and the caller sees no verdict, which the table reports.
func serveRecovering(t *testing.T, h http.Handler, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	defer func() {
		if v := recover(); v != nil {
			t.Logf("handler panicked: %v", v)
		}
	}()
	h.ServeHTTP(w, r)
}
