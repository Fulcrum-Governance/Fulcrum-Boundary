// Package adapterparity is the ADR-047 cross-adapter parity table (FUL-464).
// Every Boundary adapter runs the same cases through its real entry point and
// must surface the same verdict, and must not run the governed action unless
// the verdict allows it.
package adapterparity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// Input selects which request an adapter sends for a case.
type Input int

const (
	// Valid is a well-formed request for the adapter's parity tool.
	Valid Input = iota
	// Malformed is input the adapter cannot turn into a governance request.
	Malformed
)

// Outcome is what an adapter's caller observes.
type Outcome struct {
	// Action is the verdict the transport surfaced to its caller (header,
	// trailer, metadata or response field). Empty when it surfaced none.
	Action string
	// Executed counts calls that reached the governed downstream: the tool,
	// command, code, upstream server or allow confirmation.
	Executed int
	// Record is the decision record the adapter handed its caller, for
	// adapters that return one. Nil when the adapter returns none.
	Record *governance.DecisionRecordV1
}

// Adapter drives one transport through its governed entry point.
type Adapter struct {
	Name string
	// Tool is the GovernanceRequest.ToolName the adapter derives from its
	// Valid input. Interceptor cases register on it.
	Tool string
	// Informational adapters observe actions that already happened. They
	// report the verdict but never execute or forward anything.
	Informational bool
	// NotApplicable maps a case name to the reason the case cannot be
	// expressed for this adapter.
	NotApplicable map[string]string
	// Audit, when set, wraps the publisher the case pipeline writes to. An
	// adapter that reads its own decision event back hooks in here.
	Audit func(governance.AuditPublisher) governance.AuditPublisher
	Call  func(ctx context.Context, t *testing.T, p *governance.Pipeline, in Input) Outcome
}

// Case is one policy or fault scenario.
type Case struct {
	Name string
	// Pipeline builds the pipeline for an adapter whose parity tool is tool,
	// publishing decision records to audit. A nil result means the adapter is
	// wired with no pipeline.
	Pipeline func(tool string, audit governance.AuditPublisher) *governance.Pipeline
	Context  func() (context.Context, context.CancelFunc)
	Input    Input
	// Want is the verdict the caller must see. Empty means any blocked shape:
	// malformed input never reaches the pipeline, so there is no verdict.
	Want string
}

const indeterminate = governance.ActionCheckIndeterminate

// Cases is the shared table.
func Cases() []Case {
	return []Case{
		{Name: "allow", Pipeline: static(), Want: "allow"},
		{Name: "warn", Pipeline: static(rule("warn")), Want: "warn"},
		{Name: "deny", Pipeline: static(rule("deny")), Want: "deny"},
		{Name: "require_approval", Pipeline: static(rule("require_approval")), Want: "require_approval"},
		{Name: "escalate", Pipeline: static(rule("escalate")), Want: "escalate"},
		{Name: "unknown_static_rule_action", Pipeline: static(rule("Deny")), Want: indeterminate},
		{Name: "unknown_evaluator_action", Pipeline: evaluator(fixedEvaluator{decision: &policyeval.Decision{Action: policyeval.ActionType(97)}}), Want: indeterminate},
		{Name: "unknown_interceptor_action", Pipeline: interceptor(func(context.Context, *governance.GovernanceRequest) (*governance.InterceptorResult, error) {
			return &governance.InterceptorResult{Allowed: false, Action: "obliterate"}, nil
		}), Want: indeterminate},
		{Name: "unevaluable_condition", Pipeline: evaluator(policyeval.NewEvaluator([]*policyeval.Policy{badRegexDenyPolicy()})), Want: indeterminate},
		{Name: "invalid_policy_set", Pipeline: strictEvaluator(badRegexDenyPolicy()), Want: indeterminate},
		{Name: "evaluator_error", Pipeline: evaluator(fixedEvaluator{err: errors.New("policy engine unavailable")}), Want: indeterminate},
		{Name: "evaluator_nil_decision", Pipeline: evaluator(fixedEvaluator{}), Want: indeterminate},
		{Name: "evaluator_panic", Pipeline: evaluator(panicEvaluator{}), Want: indeterminate},
		{Name: "evaluator_timeout", Pipeline: evaluator(blockingEvaluator{}), Context: deadline, Want: indeterminate},
		{Name: "evaluator_canceled", Pipeline: evaluator(blockingEvaluator{}), Context: canceled, Want: indeterminate},
		{Name: "interceptor_error", Pipeline: interceptor(func(context.Context, *governance.GovernanceRequest) (*governance.InterceptorResult, error) {
			return nil, errors.New("sql classifier unavailable")
		}), Want: indeterminate},
		{Name: "interceptor_panic", Pipeline: interceptor(func(context.Context, *governance.GovernanceRequest) (*governance.InterceptorResult, error) {
			panic("interceptor bug")
		}), Want: indeterminate},
		{Name: "invalid_pipeline_config", Pipeline: func(_ string, audit governance.AuditPublisher) *governance.Pipeline {
			return governance.NewPipeline(governance.PipelineConfig{
				NonEnforcingTransports: []governance.NonEnforcingTransport{{Transport: governance.TransportMCP}},
			}, nil, nil, audit)
		}, Want: indeterminate},
		{Name: "nil_pipeline", Pipeline: func(string, governance.AuditPublisher) *governance.Pipeline { return nil }, Want: indeterminate},
		{Name: "malformed_input", Pipeline: static(), Input: Malformed},
	}
}

// Run runs every case against every adapter. A row listed in KnownGaps must
// still fail; once it passes, Run fails until the entry is deleted.
func Run(t *testing.T, adapters ...Adapter) {
	t.Helper()
	for _, a := range adapters {
		for _, c := range Cases() {
			t.Run(a.Name+"/"+c.Name, func(t *testing.T) {
				if reason, ok := a.NotApplicable[c.Name]; ok {
					t.Skip("not applicable: " + reason)
				}
				ctx, cancel := context.Background(), context.CancelFunc(func() {})
				if c.Context != nil {
					ctx, cancel = c.Context()
				}
				defer cancel()
				audit := &recorder{}
				var publisher governance.AuditPublisher = audit
				if a.Audit != nil {
					publisher = a.Audit(audit)
				}
				p := c.Pipeline(a.Tool, publisher)
				problems := check(a, c, a.Call(ctx, t, p, c.Input))
				if p != nil && c.Input == Valid {
					problems = append(problems, checkRecord(c, audit.events())...)
				}

				key := a.Name + "/" + c.Name
				if reason, ok := KnownGaps[key]; ok {
					if len(problems) == 0 {
						t.Fatalf("%s passes now; delete it from KnownGaps", key)
					}
					t.Skipf("known gap: %s (%s)", reason, strings.Join(problems, "; "))
				}
				for _, p := range problems {
					t.Error(p)
				}
			})
		}
	}
}

func check(a Adapter, c Case, got Outcome) []string {
	var problems []string
	if c.Want == "" {
		if got.Action == "allow" || got.Action == "warn" {
			problems = append(problems, fmt.Sprintf("action = %q, want a blocked shape", got.Action))
		}
	} else if got.Action != c.Want {
		problems = append(problems, fmt.Sprintf("action = %q, want %q", got.Action, c.Want))
	}
	wantExecuted := 0
	if !a.Informational && (c.Want == "allow" || c.Want == "warn") {
		wantExecuted = 1
	}
	if got.Executed != wantExecuted {
		problems = append(problems, fmt.Sprintf("executed = %d, want %d", got.Executed, wantExecuted))
	}
	if got.Record != nil && c.Want != "" {
		if got.Record.Action != c.Want {
			problems = append(problems, fmt.Sprintf("returned record action = %q, want %q", got.Record.Action, c.Want))
		}
		if c.Want == indeterminate && got.Record.Check == nil {
			problems = append(problems, "returned check_indeterminate record carries no check context")
		}
	}
	return problems
}

// checkRecord requires one decision record per governed request, naming the
// verdict the caller saw and, for check_indeterminate, the failed check.
func checkRecord(c Case, events []governance.AuditEvent) []string {
	if len(events) != 1 {
		return []string{fmt.Sprintf("decision records = %d, want 1", len(events))}
	}
	var problems []string
	if events[0].Action != c.Want {
		problems = append(problems, fmt.Sprintf("record action = %q, want %q", events[0].Action, c.Want))
	}
	if c.Want == indeterminate && events[0].Check == nil {
		problems = append(problems, "check_indeterminate record carries no check context")
	}
	return problems
}

type recorder struct {
	mu   sync.Mutex
	list []governance.AuditEvent
}

func (r *recorder) Publish(_ context.Context, e governance.AuditEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = append(r.list, e)
}

func (r *recorder) events() []governance.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]governance.AuditEvent(nil), r.list...)
}

func rule(action string) governance.StaticPolicyRule {
	return governance.StaticPolicyRule{Name: "parity-" + action, Tool: "*", Action: action, Reason: "parity " + action}
}

type pipelineFunc = func(tool string, audit governance.AuditPublisher) *governance.Pipeline

func static(rules ...governance.StaticPolicyRule) pipelineFunc {
	return func(_ string, audit governance.AuditPublisher) *governance.Pipeline {
		return governance.NewPipeline(governance.PipelineConfig{StaticPolicies: rules}, nil, nil, audit)
	}
}

func evaluator(e governance.PolicyEvaluator) pipelineFunc {
	return func(_ string, audit governance.AuditPublisher) *governance.Pipeline {
		return governance.NewPipeline(governance.PipelineConfig{}, nil, e, audit)
	}
}

func strictEvaluator(policies ...*policyeval.Policy) pipelineFunc {
	return func(_ string, audit governance.AuditPublisher) *governance.Pipeline {
		e, _ := policyeval.NewEvaluatorStrict(policies)
		return governance.NewPipeline(governance.PipelineConfig{}, nil, e, audit)
	}
}

func interceptor(fn governance.Interceptor) pipelineFunc {
	return func(tool string, audit governance.AuditPublisher) *governance.Pipeline {
		p := governance.NewPipeline(governance.PipelineConfig{}, nil, nil, audit)
		p.RegisterInterceptor(tool, fn)
		return p
	}
}

func deadline() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 20*time.Millisecond)
}

func canceled() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx, cancel
}

func badRegexDenyPolicy() *policyeval.Policy {
	return &policyeval.Policy{
		PolicyId: "parity-bad-regex",
		TenantId: "tenant-parity",
		Name:     "parity bad regex",
		Status:   policyeval.PolicyStatus_POLICY_STATUS_ACTIVE,
		Rules: []*policyeval.PolicyRule{{
			RuleId:  "r-bad-regex",
			Name:    "deny on an uncompilable regex",
			Enabled: true,
			Conditions: []*policyeval.PolicyCondition{{
				ConditionType: policyeval.ConditionType_CONDITION_TYPE_REGEX,
				Field:         "input.text",
				Operator:      policyeval.ConditionOperator_CONDITION_OPERATOR_MATCHES,
				Value:         &policyeval.PolicyCondition_StringValue{StringValue: "[invalid"},
			}},
			Actions: []*policyeval.PolicyAction{{ActionType: policyeval.PolicyActionType_ACTION_TYPE_DENY, Message: "should deny on match"}},
		}},
	}
}

type fixedEvaluator struct {
	decision *policyeval.Decision
	err      error
}

func (e fixedEvaluator) Evaluate(context.Context, *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	return e.decision, e.err
}

type panicEvaluator struct{}

func (panicEvaluator) Evaluate(context.Context, *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	panic("evaluator bug")
}

type blockingEvaluator struct{}

func (blockingEvaluator) Evaluate(ctx context.Context, _ *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
