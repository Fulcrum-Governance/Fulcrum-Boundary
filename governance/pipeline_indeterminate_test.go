package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// panicEvaluator panics inside Evaluate; the pipeline boundary must recover it
// and classify the failure as CHECK_INDETERMINATE / panic (ADR-047).
type panicEvaluator struct{}

func (panicEvaluator) Evaluate(context.Context, *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	panic("evaluator exploded")
}

// nilDecisionEvaluator returns (nil, nil) — an invalid required-check result.
type nilDecisionEvaluator struct{}

func (nilDecisionEvaluator) Evaluate(context.Context, *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	return nil, nil
}

// weirdActionEvaluator returns a decision whose action is outside the
// GovernanceDecision verdict vocabulary — an unknown required-check result.
type weirdActionEvaluator struct{}

func (weirdActionEvaluator) Evaluate(context.Context, *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	return &policyeval.Decision{Action: policyeval.ActionType(97)}, nil
}

// panicTrustChecker panics inside CheckAgentState.
type panicTrustChecker struct{}

func (panicTrustChecker) CheckAgentState(context.Context, string) (TrustState, error) {
	panic("trust store exploded")
}

// panicTrustBackend panics inside RecordDecision (the deferred trust update).
type panicTrustBackend struct {
	programmableTrustBackend
}

func (b *panicTrustBackend) RecordDecision(context.Context, *GovernanceRequest, *GovernanceDecision) (TrustDecisionUpdate, error) {
	panic("trust update exploded")
}

// downstreamStub models an adapter's forwarding gate: the governed tool is
// invoked only when the returned decision allows it — the same check every
// in-repo adapter performs on decision.Allowed().
type downstreamStub struct{ calls int }

func (s *downstreamStub) forward(d *GovernanceDecision) {
	if d.Allowed() {
		s.calls++
	}
}

// allTransports enumerates every transport the pipeline serves; with the
// ADR-047 defaults every one of them is execution-capable (enforcing).
var allTransports = []TransportType{
	TransportMCP,
	TransportManagedAgents,
	TransportCLI,
	TransportCodeExec,
	TransportGRPC,
	TransportA2A,
	TransportWebhook,
}

// TestPipeline_CheckIndeterminate_FailureMatrix is the ADR-047 contract test:
// every failure category, on every execution-capable transport, must produce
// ActionCheckIndeterminate — never allow, never a substantive deny — with the
// machine-readable category carried on the decision and the audit record, and
// zero downstream execution.
func TestPipeline_CheckIndeterminate_FailureMatrix(t *testing.T) {
	cases := []struct {
		name      string
		category  FailureCategory
		stage     string
		evaluator PolicyEvaluator
		trust     TrustChecker
		cfg       PipelineConfig
		agentID   string
	}{
		{
			name:      "unavailable",
			category:  FailureUnavailable,
			stage:     CheckStagePolicyEval,
			evaluator: &errorEvaluator{err: errors.New("evaluator unavailable")},
		},
		{
			name:      "timeout",
			category:  FailureTimeout,
			stage:     CheckStagePolicyEval,
			evaluator: &errorEvaluator{err: fmt.Errorf("eval: %w", context.DeadlineExceeded)},
		},
		{
			name:      "canceled",
			category:  FailureCanceled,
			stage:     CheckStagePolicyEval,
			evaluator: &errorEvaluator{err: fmt.Errorf("eval: %w", context.Canceled)},
		},
		{
			name:      "panic in evaluator",
			category:  FailurePanic,
			stage:     CheckStagePolicyEval,
			evaluator: panicEvaluator{},
		},
		{
			name:      "nil evaluator result",
			category:  FailureInvalidResult,
			stage:     CheckStagePolicyEval,
			evaluator: nilDecisionEvaluator{},
		},
		{
			name:      "unknown evaluator action",
			category:  FailureInvalidResult,
			stage:     CheckStagePolicyEval,
			evaluator: weirdActionEvaluator{},
		},
		{
			name:     "stale snapshot",
			category: FailureStaleSnapshot,
			stage:    CheckStagePolicyEval,
			evaluator: &errorEvaluator{err: NewCheckError(FailureStaleSnapshot,
				errors.New("policy snapshot age 120s exceeds ttl 30s"))},
		},
		{
			name:     "missing identity",
			category: FailureMissingIdentity,
			stage:    CheckStageIdentity,
			cfg:      PipelineConfig{RequireAgentID: true},
			agentID:  "",
		},
		{
			name:     "trust lookup unavailable",
			category: FailureUnavailable,
			stage:    CheckStageTrust,
			trust:    &mockTrustChecker{err: errors.New("redis down")},
			agentID:  "agent-1",
		},
		{
			name:     "panic in trust lookup",
			category: FailurePanic,
			stage:    CheckStageTrust,
			trust:    panicTrustChecker{},
			agentID:  "agent-1",
		},
		{
			name:     "trust update unavailable",
			category: FailureUnavailable,
			stage:    CheckStageTrustUpdate,
			trust: &programmableTrustBackend{
				checkState: TrustStateTrusted,
				recordErr:  errors.New("trust store unreachable"),
			},
			agentID: "agent-1",
		},
		{
			name:     "panic in trust update",
			category: FailurePanic,
			stage:    CheckStageTrustUpdate,
			trust: &panicTrustBackend{programmableTrustBackend{
				checkState: TrustStateTrusted,
			}},
			agentID: "agent-1",
		},
	}

	for _, tc := range cases {
		for _, transport := range allTransports {
			t.Run(fmt.Sprintf("%s/%s", tc.name, transport), func(t *testing.T) {
				auditor := &collectingAuditor{}
				p := NewPipeline(tc.cfg, tc.trust, tc.evaluator, auditor)
				downstream := &downstreamStub{}

				req := &GovernanceRequest{
					ToolName:  "read_file",
					Transport: transport,
					AgentID:   tc.agentID,
					TenantID:  "tenant-1",
				}
				d, err := p.Evaluate(context.Background(), req)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				// The classification is CHECK_INDETERMINATE — it must never
				// read as allow or as a substantive policy deny.
				if d.Action != ActionCheckIndeterminate {
					t.Fatalf("action = %q, want %q", d.Action, ActionCheckIndeterminate)
				}
				if d.Allowed() {
					t.Fatal("check_indeterminate must not allow execution")
				}

				// Zero downstream execution.
				downstream.forward(d)
				if downstream.calls != 0 {
					t.Fatal("indeterminate decision reached the downstream tool")
				}

				// The decision carries the ADR-047 context.
				if d.Check == nil {
					t.Fatal("decision.Check is nil; failure context not carried")
				}
				if d.Check.Category != tc.category {
					t.Errorf("check category = %q, want %q", d.Check.Category, tc.category)
				}
				if d.Check.Stage != tc.stage {
					t.Errorf("check stage = %q, want %q", d.Check.Stage, tc.stage)
				}

				// The emitted record keeps the classification plus safe context.
				events := auditor.Events()
				if len(events) != 1 {
					t.Fatalf("expected 1 audit event, got %d", len(events))
				}
				e := events[0]
				if e.Action != ActionCheckIndeterminate {
					t.Errorf("audit action = %q, want %q", e.Action, ActionCheckIndeterminate)
				}
				if e.Action == "allow" || e.Action == "deny" {
					t.Errorf("audit action %q mislabels the infrastructure failure", e.Action)
				}
				if e.Check == nil || e.Check.Category != tc.category {
					t.Errorf("audit check = %+v, want category %q", e.Check, tc.category)
				}
				if e.RequestID == "" || e.RequestHash == "" {
					t.Error("audit record must retain request id and canonical action digest")
				}
			})
		}
	}
}

// TestPipeline_CheckIndeterminate_WebhookExecutionBlocksByDefault pins the
// ADR-047 B02/B07 fix: TransportWebhook is an execution-capable transport, so
// it is enforcing by default and an evaluator failure blocks (previously the
// webhook row failed open and could be forwarded).
func TestPipeline_CheckIndeterminate_WebhookExecutionBlocksByDefault(t *testing.T) {
	ev := &errorEvaluator{err: errors.New("evaluator unavailable")}
	p := NewPipeline(PipelineConfig{}, nil, ev, nil)
	downstream := &downstreamStub{}

	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "deploy",
		Transport: TransportWebhook,
		TenantID:  "t1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionCheckIndeterminate {
		t.Fatalf("webhook execution mode must block on evaluator error; got %q", d.Action)
	}
	downstream.forward(d)
	if downstream.calls != 0 {
		t.Fatal("webhook execution mode forwarded on evaluator error")
	}
}

// TestPipeline_CheckIndeterminate_NonEnforcingTransport_RecordsWouldHaveBlocked
// pins the ADR-047 §5 exception: a transport explicitly declared in
// NonEnforcingTransports is a declared non-enforcing surface — it may
// continue after a check failure only because it cannot deny, and the record
// MUST carry the would-have-blocked CHECK_INDETERMINATE context rather than an
// ordinary allow.
func TestPipeline_CheckIndeterminate_NonEnforcingTransport_RecordsWouldHaveBlocked(t *testing.T) {
	auditor := &collectingAuditor{}
	ev := &errorEvaluator{err: errors.New("evaluator unavailable")}
	// Webhook is the declared non-enforcing exception; everything else —
	// MCP included — still enforces.
	cfg := PipelineConfig{NonEnforcingTransports: []NonEnforcingTransport{
		{Transport: TransportWebhook, Reason: "informational webhook sink; cannot block upstream"},
	}}
	p := NewPipeline(cfg, nil, ev, auditor)

	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "notify",
		Transport: TransportWebhook,
		TenantID:  "t1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Fatalf("non-enforcing transport should continue; got %q", d.Action)
	}
	if d.Check == nil {
		t.Fatal("non-enforcing continuation must record the would-have-blocked check failure")
	}
	if d.Check.Category != FailureUnavailable || d.Check.Stage != CheckStagePolicyEval {
		t.Errorf("check = %+v, want policy_eval/unavailable", d.Check)
	}

	events := auditor.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Action != "allow" {
		t.Errorf("non-enforcing audit action = %q, want allow with check context", events[0].Action)
	}
	if events[0].Check == nil || events[0].Check.Category != FailureUnavailable {
		t.Errorf("audit must record the would-have-blocked check; got %+v", events[0].Check)
	}
}

// TestPipeline_Config_InvalidNonEnforcingDeclarationRejected pins the
// ADR-047 §6 ruling on the inverted model: a non-enforcing declaration that
// does not name its transport or record its reason is not an acceptable
// production policy. It is rejected at config validation and — because an
// invalid configuration cannot distinguish allow from deny — every decision
// is CHECK_INDETERMINATE / missing_config on every transport.
func TestPipeline_Config_InvalidNonEnforcingDeclarationRejected(t *testing.T) {
	cfg := PipelineConfig{NonEnforcingTransports: []NonEnforcingTransport{
		{Transport: TransportWebhook}, // no reason — ambiguous exemption
	}}
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidNonEnforcingTransport) {
		t.Fatalf("Validate() = %v, want ErrInvalidNonEnforcingTransport", err)
	}

	auditor := &collectingAuditor{}
	p := NewPipeline(cfg, nil, nil, auditor)
	if p.ConfigError() == nil {
		t.Fatal("expected ConfigError to report the rejected config")
	}
	downstream := &downstreamStub{}

	for _, transport := range allTransports {
		t.Run(string(transport), func(t *testing.T) {
			d, err := p.Evaluate(context.Background(), &GovernanceRequest{
				ToolName:  "read_file",
				Transport: transport,
				AgentID:   "agent-1",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Action != ActionCheckIndeterminate {
				t.Fatalf("action = %q, want %q on invalid config", d.Action, ActionCheckIndeterminate)
			}
			if d.Check == nil || d.Check.Category != FailureMissingConfig {
				t.Fatalf("check = %+v, want missing_config", d.Check)
			}
			downstream.forward(d)
		})
	}
	if downstream.calls != 0 {
		t.Fatal("invalid config allowed downstream execution")
	}
	if got := len(auditor.Events()); got != len(allTransports) {
		t.Fatalf("expected one audit record per blocked evaluation, got %d", got)
	}
}

// TestPipeline_Config_ValidateNonEnforcingListAccepted confirms the
// validation boundary: nil, empty, and well-formed populated lists are all
// valid — only malformed entries are rejected.
func TestPipeline_Config_ValidateNonEnforcingListAccepted(t *testing.T) {
	for _, cfg := range []PipelineConfig{
		{},
		{NonEnforcingTransports: []NonEnforcingTransport{}},
		{NonEnforcingTransports: []NonEnforcingTransport{{Transport: TransportWebhook, Reason: "informational only"}}},
		{NonEnforcingTransports: []NonEnforcingTransport{
			{Transport: TransportWebhook, Reason: "informational only"},
			{Transport: TransportCLI, Reason: "interactive session; evaluator outage must not brick it"},
		}},
	} {
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v, want nil", cfg, err)
		}
	}
}

// TestPipeline_RequireAudit_NilAuditorIsConfigError pins the second half of
// the audit contract: on a surface that declares its records must be
// delivered, a nil auditor is a configuration error, not a silent no-op.
func TestPipeline_RequireAudit_NilAuditorIsConfigError(t *testing.T) {
	p := NewPipeline(PipelineConfig{RequireAudit: true}, nil, nil, nil)
	if p.ConfigError() == nil {
		t.Fatal("RequireAudit with nil auditor must be a config error")
	}
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "read_file",
		Transport: TransportMCP,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionCheckIndeterminate {
		t.Fatalf("action = %q, want %q", d.Action, ActionCheckIndeterminate)
	}
	if d.Check == nil || d.Check.Category != FailureMissingConfig {
		t.Fatalf("check = %+v, want missing_config", d.Check)
	}

	// The same configuration with a real auditor is not an error.
	ok := NewPipeline(PipelineConfig{RequireAudit: true}, nil, nil, &collectingAuditor{})
	if ok.ConfigError() != nil {
		t.Fatalf("RequireAudit with an auditor must not be a config error: %v", ok.ConfigError())
	}
}

// TestPipeline_AuditDeliveryFailure_ExposesDegraded covers ADR-047 B09: an
// audit delivery failure never authorizes and never changes the decision, but
// the surface MUST expose a degraded state. The publisher reports failures
// through CheckedAuditPublisher; Degraded() flips on failure and clears on
// recovery while AuditFailures accumulates.
func TestPipeline_AuditDeliveryFailure_ExposesDegraded(t *testing.T) {
	pub := &failingCheckedPublisher{failRemaining: 2}
	p := NewPipeline(PipelineConfig{}, nil, nil, pub)
	req := &GovernanceRequest{ToolName: "read_file", Transport: TransportMCP, AgentID: "a"}

	// Outage: two failed publishes. The decision is unchanged (audit delivery
	// failure never authorizes nor blocks), the counter accumulates, and the
	// surface reports degraded.
	d, err := p.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Fatalf("audit failure must not change the decision; got %q", d.Action)
	}
	if !p.Degraded() {
		t.Fatal("Degraded() must be true after a failed publish")
	}
	if got := p.AuditFailures(); got != 1 {
		t.Fatalf("AuditFailures = %d, want 1", got)
	}

	if _, err := p.Evaluate(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Degraded() {
		t.Fatal("Degraded() must stay true while the publisher is down")
	}
	if got := p.AuditFailures(); got != 2 {
		t.Fatalf("AuditFailures = %d, want 2", got)
	}

	// Recovery: the next publish succeeds, Degraded() clears, the counter
	// keeps the outage history.
	d, err = p.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Fatalf("decision after recovery = %q, want allow", d.Action)
	}
	if p.Degraded() {
		t.Fatal("Degraded() must clear after a successful publish")
	}
	if got := p.AuditFailures(); got != 2 {
		t.Fatalf("AuditFailures = %d after recovery, want 2", got)
	}
}

// TestPipeline_AuditPublisherPanic_DegradedNotFatal proves a panicking
// publisher cannot crash the calling adapter or change the decision: the
// panic is recovered at the publish boundary, counted, and exposes degraded
// state.
func TestPipeline_AuditPublisherPanic_DegradedNotFatal(t *testing.T) {
	p := NewPipeline(PipelineConfig{}, nil, nil, panickingPublisher{})
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "read_file",
		Transport: TransportMCP,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Fatalf("publisher panic must not change the decision; got %q", d.Action)
	}
	if !p.Degraded() {
		t.Fatal("Degraded() must be true after a publisher panic")
	}
	if got := p.AuditFailures(); got != 1 {
		t.Fatalf("AuditFailures = %d, want 1", got)
	}
}

// TestPipeline_DryRun_CheckIndeterminate_RecordsWouldHaveBlocked extends the
// dry-run contract to the new classification: dry-run is an explicit
// non-enforcing mode, so an indeterminate (would-have-blocked) outcome is
// converted to allow for the caller while the audit record keeps the real
// CHECK_INDETERMINATE action.
func TestPipeline_DryRun_CheckIndeterminate_RecordsWouldHaveBlocked(t *testing.T) {
	auditor := &collectingAuditor{}
	ev := &errorEvaluator{err: errors.New("evaluator unavailable")}
	p := NewPipeline(PipelineConfig{DryRun: true}, nil, ev, auditor)

	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "read_file",
		Transport: TransportMCP,
		TenantID:  "t1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Fatalf("dry-run caller action = %q, want allow", d.Action)
	}
	if !d.DryRun {
		t.Error("expected decision.DryRun=true")
	}
	if !strings.HasPrefix(d.Reason, "DRY-RUN would block: ") {
		t.Errorf("expected DRY-RUN would-block prefix, got %q", d.Reason)
	}
	if d.Check == nil {
		t.Error("caller-visible decision must keep the check classification")
	}

	events := auditor.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Action != ActionCheckIndeterminate {
		t.Errorf("audit must record check_indeterminate, got %q", events[0].Action)
	}
	if events[0].Check == nil || events[0].Check.Category != FailureUnavailable {
		t.Errorf("audit check = %+v, want unavailable", events[0].Check)
	}
}

// TestPipeline_CheckIndeterminate_InterceptorError pins Stage 3 under
// ADR-047: an interceptor failure is a required-check failure, not a
// substantive deny.
func TestPipeline_CheckIndeterminate_InterceptorError(t *testing.T) {
	p := NewPipeline(PipelineConfig{}, nil, nil, nil)
	p.RegisterInterceptor("bad_tool", func(_ context.Context, _ *GovernanceRequest) (*InterceptorResult, error) {
		return nil, errors.New("interceptor crashed")
	})
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "bad_tool",
		Transport: TransportMCP,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionCheckIndeterminate {
		t.Fatalf("action = %q, want %q", d.Action, ActionCheckIndeterminate)
	}
	if d.Check == nil || d.Check.Stage != CheckStageInterceptor || d.Check.Category != FailureUnavailable {
		t.Fatalf("check = %+v, want interceptor/unavailable", d.Check)
	}
}

// TestPipeline_CheckIndeterminate_InvalidInterceptorResult covers a blocking
// interceptor result whose action is outside the verdict vocabulary: that is
// an invalid_result check failure, not an adoptable verdict.
func TestPipeline_CheckIndeterminate_InvalidInterceptorResult(t *testing.T) {
	p := NewPipeline(PipelineConfig{}, nil, nil, nil)
	p.RegisterInterceptor("odd_tool", func(_ context.Context, _ *GovernanceRequest) (*InterceptorResult, error) {
		return &InterceptorResult{Allowed: false, Action: "bogus-verdict"}, nil
	})
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "odd_tool",
		Transport: TransportMCP,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionCheckIndeterminate {
		t.Fatalf("action = %q, want %q", d.Action, ActionCheckIndeterminate)
	}
	if d.Check == nil || d.Check.Category != FailureInvalidResult {
		t.Fatalf("check = %+v, want invalid_result", d.Check)
	}
}

// TestPipeline_CheckIndeterminate_TrustUpdateFailure_PreservesDenyVerdict
// proves the classification never rewrites a substantive outcome: when the
// decision is already a policy deny, a deferred trust-update failure is
// recorded on the check context but the verdict stays deny.
func TestPipeline_CheckIndeterminate_TrustUpdateFailure_PreservesDenyVerdict(t *testing.T) {
	backend := &programmableTrustBackend{
		checkState: TrustStateTrusted,
		recordErr:  errors.New("trust store unreachable"),
	}
	cfg := PipelineConfig{
		StaticPolicies: []StaticPolicyRule{
			{Name: "block-rm", Tool: "rm", Action: "deny", Reason: "destructive command"},
		},
	}
	p := NewPipeline(cfg, backend, nil, nil)
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "rm",
		Transport: TransportMCP,
		AgentID:   "agent-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "deny" {
		t.Fatalf("substantive deny must not be rewritten; got %q", d.Action)
	}
	if d.Check == nil || d.Check.Stage != CheckStageTrustUpdate {
		t.Fatalf("failed trust update must still be recorded; check = %+v", d.Check)
	}
}

// TestPipeline_CheckIndeterminate_RecordCarriesSafeContext builds the
// decision record from the emitted event and asserts the ADR-047 §3 evidence
// contract: the classification, stage, class, category, request id, and the
// canonical action digest (request_hash) are present, and the record is not
// labeled allow or a substantive deny.
func TestPipeline_CheckIndeterminate_RecordCarriesSafeContext(t *testing.T) {
	auditor := &collectingAuditor{}
	ev := &errorEvaluator{err: fmt.Errorf("eval: %w", context.DeadlineExceeded)}
	p := NewPipeline(PipelineConfig{GatewayVersion: "test-v"}, nil, ev, auditor)

	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "read_file",
		Transport: TransportGRPC,
		AgentID:   "agent-9",
		TenantID:  "tenant-9",
		TraceID:   "trace-9",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := auditor.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	record := BuildDecisionRecord(events[0])
	if record.Action != ActionCheckIndeterminate {
		t.Fatalf("record action = %q, want %q", record.Action, ActionCheckIndeterminate)
	}
	if record.Check == nil {
		t.Fatal("record carries no check context")
	}
	if record.Check.Category != FailureTimeout ||
		record.Check.Stage != CheckStagePolicyEval ||
		record.Check.Class != CheckClassPolicy {
		t.Errorf("record check = %+v, want policy_eval/policy/timeout", record.Check)
	}
	if record.RequestID == "" && events[0].RequestID == "" {
		t.Error("record must carry the request identifier")
	}
	if events[0].RequestID == "" || events[0].RequestID != d.RequestID {
		t.Error("record must carry the request identifier")
	}
	if record.AgentID != "agent-9" || record.TenantID != "tenant-9" {
		t.Error("record must carry tenant/agent identifiers when present")
	}
	if record.Adapter != TransportGRPC {
		t.Errorf("record must carry the transport; got %q", record.Adapter)
	}
	if record.RequestHash == "" {
		t.Error("record must carry the canonical action digest (request_hash)")
	}
	if d.Allowed() {
		t.Error("indeterminate decision must not allow")
	}
}

// failingCheckedPublisher implements AuditPublisher plus the optional
// CheckedAuditPublisher reporting seam, so tests can drive the pipeline's
// degraded-state accounting. It fails failRemaining publishes, then succeeds.
type failingCheckedPublisher struct {
	failRemaining int
	calls         int
}

func (p *failingCheckedPublisher) Publish(context.Context, AuditEvent) {
	p.calls++
}

func (p *failingCheckedPublisher) PublishChecked(context.Context, AuditEvent) error {
	p.calls++
	if p.failRemaining > 0 {
		p.failRemaining--
		return errors.New("audit sink unreachable")
	}
	return nil
}

// panickingPublisher panics inside Publish.
type panickingPublisher struct{}

func (panickingPublisher) Publish(context.Context, AuditEvent) {
	panic("publisher exploded")
}
