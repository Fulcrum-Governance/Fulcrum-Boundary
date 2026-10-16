package commandboundary_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/adapterparity"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
)

// TestExecutorParity runs `boundary command run`'s executor through the
// ADR-047 parity table.
func TestExecutorParity(t *testing.T) {
	adapterparity.Run(t, adapterparity.Adapter{
		Name: "commandboundary",
		Tool: "ls",
		NotApplicable: map[string]string{
			"nil_pipeline": "a nil Pipeline means the built-in preview policy, which `boundary command run` relies on",
		},
		Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
			runner := &countingRunner{}
			argv := []string{"ls", "-la"}
			if in == adapterparity.Malformed {
				argv = nil
			}
			res, err := commandboundary.Executor{Pipeline: p, Runner: runner}.Run(ctx, commandboundary.RunRequest{
				Argv: argv, CWD: t.TempDir(), RecordPath: filepath.Join(t.TempDir(), "records.jsonl"),
			})
			out := adapterparity.Outcome{Executed: runner.calls}
			if err == nil {
				out.Action = res.Record.Action
			}
			return out
		},
	})
}

type countingRunner struct{ calls int }

func (r *countingRunner) Run(context.Context, commandboundary.ExecutionRequest) (commandboundary.ExecutionResult, error) {
	r.calls++
	return commandboundary.ExecutionResult{}, nil
}
