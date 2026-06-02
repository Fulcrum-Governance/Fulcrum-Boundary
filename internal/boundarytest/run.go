package boundarytest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

var fixedDoesNotProve = []string{
	"boundary test reports policy verdicts for routed request fixtures; it does not prove production route enforcement.",
	"boundary test does not prove a deployment removed every direct or unrouted path to the same tool.",
	"boundary test does not prove the verdict was globally correct; it proves only that the local policy bundle produced the expected decision for the supplied fixture.",
}

func Run(opts Options) (*Result, error) {
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		path = ".boundary/tests"
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read test directory %s: %w", path, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(path, name))
		}
	}
	sort.Strings(files)

	result := &Result{
		SchemaVersion:       SchemaVersion,
		Status:              "pass",
		Path:                path,
		RequiresCredentials: false,
		RequiresNetwork:     false,
		MutatesLiveSystems:  false,
		DoesNotProve:        fixedDoesNotProve,
	}
	for _, file := range files {
		caseResult := runCase(file)
		result.Cases = append(result.Cases, caseResult)
	}
	if len(files) == 0 {
		result.Cases = append(result.Cases, CaseResult{
			Name:         filepath.Base(path),
			Status:       "fail",
			ActualAction: "case_parse_error",
			Error:        fmt.Sprintf("%s: no YAML test case files found", path),
		})
	}

	result.Summary.Total = len(result.Cases)
	for _, c := range result.Cases {
		if c.Status == "pass" {
			result.Summary.Passed++
		} else {
			result.Summary.Failed++
		}
	}
	if result.Summary.Failed > 0 {
		result.Status = "fail"
	}
	return result, nil
}

func runCase(path string) CaseResult {
	name := caseNameFromPath(path)
	body, err := os.ReadFile(path)
	if err != nil {
		return CaseResult{Name: name, Status: "fail", ActualAction: "case_parse_error", Error: fmt.Sprintf("read case: %v", err)}
	}
	var c testCase
	if err := yaml.Unmarshal(body, &c); err != nil {
		return CaseResult{Name: name, Status: "fail", ActualAction: "case_parse_error", Error: fmt.Sprintf("parse case: %v", err)}
	}
	if strings.TrimSpace(c.Name) != "" {
		name = c.Name
	}

	expected := strings.ToLower(strings.TrimSpace(c.Expect.Action))
	base := CaseResult{Name: name, ExpectedAction: expected}
	if err := validateCase(c); err != nil {
		base.Status = "fail"
		base.ActualAction = "case_parse_error"
		base.Error = err.Error()
		return base
	}

	policyDir := resolveRelative(filepath.Dir(path), c.Policies)
	policies, err := governance.LoadStaticPoliciesFromDir(policyDir)
	if err != nil {
		if expected == "parse_rejection" {
			base.Status = "pass"
			base.ActualAction = "parse_rejection"
			base.Error = err.Error()
			return base
		}
		base.Status = "fail"
		base.ActualAction = "policy_load_error"
		base.Error = err.Error()
		return base
	}
	if expected == "parse_rejection" {
		base.Status = "fail"
		base.ActualAction = "policy_loaded"
		base.Error = "expected policy parse rejection, but policy bundle loaded"
		return base
	}

	req := c.Request.toGovernanceRequest()
	pipeline := governance.NewPipeline(governance.PipelineConfig{StaticPolicies: policies}, nil, nil, nil)
	decision, err := pipeline.Evaluate(context.Background(), req)
	if err != nil {
		base.Status = "fail"
		base.ActualAction = "policy_eval_error"
		base.Error = err.Error()
		return base
	}

	base.ActualAction = decision.Action
	base.Reason = decision.Reason
	base.MatchedRule = decision.MatchedRule
	base.PolicyFile = decision.PolicyFile
	if decision.Action != expected {
		base.Status = "fail"
		base.Error = fmt.Sprintf("expected %s, got %s", expected, decision.Action)
		return base
	}
	if needle := strings.TrimSpace(c.Expect.ReasonContains); needle != "" && !strings.Contains(decision.Reason, needle) {
		base.Status = "fail"
		base.Error = fmt.Sprintf("expected reason to contain %q, got %q", needle, decision.Reason)
		return base
	}
	base.Status = "pass"
	return base
}

func validateCase(c testCase) error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("case name is required")
	}
	if strings.TrimSpace(c.Policies) == "" {
		return fmt.Errorf("policies path is required")
	}
	if strings.TrimSpace(c.Expect.Action) == "" {
		return fmt.Errorf("expect.action is required")
	}
	switch strings.ToLower(strings.TrimSpace(c.Expect.Action)) {
	case "allow", "deny", "warn", "escalate", "require_approval", "parse_rejection":
	default:
		return fmt.Errorf("unsupported expect.action %q", c.Expect.Action)
	}
	if strings.ToLower(strings.TrimSpace(c.Expect.Action)) != "parse_rejection" && strings.TrimSpace(firstNonEmpty(c.Request.ToolName, c.Request.Tool)) == "" {
		return fmt.Errorf("request.tool_name is required")
	}
	return nil
}

func (r requestFixture) toGovernanceRequest() *governance.GovernanceRequest {
	toolName := firstNonEmpty(r.ToolName, r.Tool)
	return &governance.GovernanceRequest{
		RequestID: r.RequestID,
		Transport: r.Transport,
		AgentID:   r.AgentID,
		TenantID:  r.TenantID,
		ToolName:  toolName,
		Action:    r.Action,
		Arguments: r.Arguments,
		Command:   r.Command,
		Code:      r.Code,
		Language:  r.Language,
		TraceID:   r.TraceID,
		BudgetKey: r.BudgetKey,
	}
}

func resolveRelative(base, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(base, value))
}

func caseNameFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
