package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Executor executes a governed CLI command without invoking a shell.
type Executor interface {
	Execute(ctx context.Context, req *governance.GovernanceRequest) (*governance.ToolResponse, error)
}

// OSExecutor executes parsed command segments through os/exec.
type OSExecutor struct {
	Env []string
}

// Execute runs the parsed pipe chain. It never invokes /bin/sh; each segment is
// executed as a command with explicit argv.
func (e OSExecutor) Execute(ctx context.Context, req *governance.GovernanceRequest) (*governance.ToolResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("governance request is required")
	}
	if len(req.PipeChain) == 0 {
		return nil, fmt.Errorf("cli request has no parsed pipe chain")
	}

	start := time.Now()
	input := req.Stdin
	var stderr bytes.Buffer
	exitCode := 0

	for _, segment := range req.PipeChain {
		cmd := exec.CommandContext(ctx, segment.Command, segment.Args...) // #nosec G204 -- command and argv are the governed CLI payload after policy evaluation; no shell is invoked.
		cmd.Env = governedEnv(req, e.Env)
		cmd.Stdin = bytes.NewReader(input)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		input = stdout.Bytes()
		if err != nil {
			exitCode = exitCodeFromError(err)
			if exitCode == 0 {
				return nil, err
			}
			break
		}
	}

	resp := &governance.ToolResponse{
		Content:     input,
		ContentType: "text/plain",
		ExitCode:    exitCode,
		Duration:    time.Since(start),
		Metadata: map[string]string{
			"cli_command": req.Command,
		},
	}
	if stderr.Len() > 0 {
		resp.Metadata["cli_stderr"] = stderr.String()
	}
	return resp, nil
}

func governedEnv(req *governance.GovernanceRequest, extra []string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"BOUNDARY_GOVERNED=true",
		"BOUNDARY_TRANSPORT=cli",
		"BOUNDARY_REQUEST_ID="+req.RequestID,
		"BOUNDARY_ENVELOPE_ID="+req.EnvelopeID,
		"BOUNDARY_TENANT_ID="+req.TenantID,
		"BOUNDARY_AGENT_ID="+req.AgentID,
	)
	return append(env, extra...)
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

func deniedResponse(decision *governance.GovernanceDecision) *governance.ToolResponse {
	reason := "denied by Boundary"
	if decision != nil && decision.Reason != "" {
		reason = decision.Reason
	}
	resp := &governance.ToolResponse{
		Content:     []byte(reason + "\n"),
		ContentType: "text/plain",
		ExitCode:    126,
		Metadata: map[string]string{
			"x-fulcrum-action": "deny",
			"cli_denied":       "true",
		},
	}
	if decision != nil {
		resp.Metadata["x-fulcrum-request-id"] = decision.RequestID
		resp.Metadata["x-fulcrum-envelope-id"] = decision.EnvelopeID
		if decision.MatchedRule != "" {
			resp.Metadata["x-fulcrum-rule"] = decision.MatchedRule
		}
	}
	return resp
}

func attachInspectionMetadata(resp *governance.ToolResponse, inspection *governance.ResponseInspection) {
	if resp == nil || inspection == nil || inspection.Safe {
		return
	}
	if resp.Metadata == nil {
		resp.Metadata = map[string]string{}
	}
	resp.Metadata["x-fulcrum-inspection-safe"] = strconv.FormatBool(inspection.Safe)
	if len(inspection.Concerns) > 0 {
		resp.Metadata["x-fulcrum-inspection-concerns"] = fmt.Sprintf("%v", inspection.Concerns)
	}
}
