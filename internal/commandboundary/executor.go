package commandboundary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

type ExecutionRequest struct {
	Command string
	Args    []string
	CWD     string
	Env     []string
}

type ExecutionResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
}

type Runner interface {
	Run(ctx context.Context, req ExecutionRequest) (ExecutionResult, error)
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, req ExecutionRequest) (ExecutionResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, req.Command, req.Args...) // #nosec G204 -- command and argv have been governed; no shell is invoked.
	cmd.Dir = req.CWD
	cmd.Env = governedEnv(req.Env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := ExecutionResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCodeFromError(err),
		Duration: time.Since(start),
	}
	if err != nil && len(result.Stderr) == 0 {
		result.Stderr = []byte(err.Error() + "\n")
	}
	return result, nil
}

type Executor struct {
	Pipeline   *governance.Pipeline
	Runner     Runner
	RecordPath string
	Env        []string
}

type RunResult struct {
	Classification Classification
	Decision       *governance.GovernanceDecision
	Record         CommandDecisionRecord
	RecordPath     string
	Stdout         []byte
	Stderr         []byte
	Executed       bool
	ExitCode       int
}

func (e Executor) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if len(req.Argv) == 0 {
		return nil, errors.New("command is required")
	}
	cwd := req.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
		req.CWD = cwd
	}
	recordPath := firstNonEmpty(req.RecordPath, e.RecordPath, DefaultDecisionRecordPath)
	if err := prepareRecordPath(recordPath); err != nil {
		return nil, err
	}

	classification, err := Classify(req.Argv)
	if err != nil {
		return nil, err
	}
	argvHash := HashArgv(req.Argv)
	governanceReq := BuildGovernanceRequest(req, classification, argvHash)
	pipeline := e.Pipeline
	if pipeline == nil {
		pipeline = NewDefaultPreviewPipeline()
	}
	decision, err := pipeline.Evaluate(ctx, governanceReq)
	if err != nil {
		decision = &governance.GovernanceDecision{
			RequestID:  governanceReq.RequestID,
			Action:     "deny",
			Reason:     fmt.Sprintf("governance pipeline error: %v", err),
			EnvelopeID: governanceReq.EnvelopeID,
		}
	}

	result := &RunResult{
		Classification: classification,
		Decision:       decision,
		RecordPath:     recordPath,
		ExitCode:       126,
	}
	if decision.Allowed() {
		runner := e.Runner
		if runner == nil {
			runner = OSRunner{}
		}
		execResult, err := runner.Run(ctx, ExecutionRequest{
			Command: req.Argv[0],
			Args:    append([]string(nil), req.Argv[1:]...),
			CWD:     cwd,
			Env:     append(append([]string(nil), e.Env...), req.Env...),
		})
		if err != nil {
			return nil, err
		}
		result.Executed = true
		result.Stdout = execResult.Stdout
		result.Stderr = execResult.Stderr
		result.ExitCode = execResult.ExitCode
	}

	record := CommandDecisionRecord{
		RecordType:    "command_decision",
		SchemaVersion: SchemaVersionDecision,
		RequestID:     decision.RequestID,
		EnvelopeID:    decision.EnvelopeID,
		Command:       classification.Command,
		ArgsHash:      argvHash,
		ArgsRedacted:  classification.ArgsRedacted,
		CWD:           cwd,
		Class:         classification.Class,
		Risk:          classification.Risk,
		Action:        decision.Action,
		Executed:      result.Executed,
		ExitCode:      result.ExitCode,
		Reason:        decision.Reason,
		MatchedRule:   decision.MatchedRule,
		PolicyID:      decision.PolicyID,
		Timestamp:     time.Now(),
	}
	if err := AppendDecisionRecord(recordPath, record); err != nil {
		return nil, err
	}
	result.Record = record
	return result, nil
}

func governedEnv(extra []string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "BOUNDARY_COMMAND_GOVERNED=true")
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
