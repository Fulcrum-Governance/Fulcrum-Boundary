package redteam

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	mode := strings.TrimSpace(opts.Mode)
	if mode == "" {
		mode = ModeFixture
	}
	if mode != ModeFixture {
		return nil, fmt.Errorf("redteam mode %q is not supported; only %q runs without live system access", mode, ModeFixture)
	}

	packID := strings.TrimSpace(opts.PackID)
	if packID == "" {
		packID = DefaultPackID
	}
	pack, ok := findPack(packID)
	if !ok {
		return nil, fmt.Errorf("unknown redteam pack %q", packID)
	}
	if pack.Status != PackStatusImplemented {
		return nil, fmt.Errorf("redteam pack %q is a %s: %s", pack.ID, pack.Status, pack.StubReason)
	}
	if len(pack.Scenarios) == 0 {
		return nil, fmt.Errorf("redteam pack %q has no fixture scenarios", pack.ID)
	}

	result := &RunResult{
		SchemaVersion:      SchemaVersion,
		Mode:               mode,
		PackID:             pack.ID,
		PackName:           pack.Name,
		Status:             ResultPassed,
		Passed:             true,
		MutatesLiveSystems: false,
		RealSecretsUsed:    false,
		Results:            make([]ScenarioResult, 0, len(pack.Scenarios)),
	}

	for _, scenario := range pack.Scenarios {
		scenarioResult, err := runScenario(ctx, pack, scenario, mode)
		if err != nil {
			return nil, err
		}
		if !scenarioResult.Passed {
			result.Passed = false
			result.Status = ResultFailed
		}
		result.Results = append(result.Results, scenarioResult)
	}
	return result, nil
}

func runScenario(ctx context.Context, pack Pack, scenario Scenario, mode string) (ScenarioResult, error) {
	auditor := &captureAuditPublisher{}
	req := scenario.Request
	if req.Arguments != nil {
		req.Arguments = cloneArguments(req.Arguments)
	}
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: scenario.Policies,
		GatewayVersion: "redteam-fixture",
		BuildDigest:    "fixture-only",
	}, nil, nil, auditor)

	decision, err := pipeline.Evaluate(ctx, &req)
	if err != nil {
		return ScenarioResult{}, fmt.Errorf("run redteam scenario %q: %w", scenario.ID, err)
	}
	event, ok := auditor.LastDecisionEvent()
	if !ok {
		return ScenarioResult{}, fmt.Errorf("run redteam scenario %q: no decision record emitted", scenario.ID)
	}
	record := governance.BuildDecisionRecord(event)
	passed := decision.Action == scenario.ExpectedAction
	status := ResultPassed
	if !passed {
		status = ResultFailed
	}
	return ScenarioResult{
		PackID:         pack.ID,
		ScenarioID:     scenario.ID,
		Name:           scenario.Name,
		Mode:           mode,
		Status:         status,
		FixtureOnly:    scenario.FixtureOnly,
		NoLiveMutation: scenario.NoLiveMutation,
		ExpectedAction: scenario.ExpectedAction,
		ActualAction:   decision.Action,
		Passed:         passed,
		Reason:         decision.Reason,
		MatchedRule:    decision.MatchedRule,
		DecisionRecord: record,
	}, nil
}

func cloneArguments(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

type captureAuditPublisher struct {
	mu     sync.Mutex
	events []governance.AuditEvent
}

func (p *captureAuditPublisher) Publish(_ context.Context, event governance.AuditEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *captureAuditPublisher) LastDecisionEvent() (governance.AuditEvent, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := len(p.events) - 1; i >= 0; i-- {
		if p.events[i].EventType == "" || p.events[i].EventType == "governance_decision" {
			return p.events[i], true
		}
	}
	return governance.AuditEvent{}, false
}
