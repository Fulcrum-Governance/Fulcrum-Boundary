package editboundary_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/adapterparity"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

const docsPatch = `diff --git a/docs/example.md b/docs/example.md
--- a/docs/example.md
+++ b/docs/example.md
@@ -1 +1,2 @@
 # Example
+More detail.
`

// TestApplyExecutorParity runs `boundary edit apply`'s executor through the
// ADR-047 parity table.
func TestApplyExecutorParity(t *testing.T) {
	adapterparity.Run(t, adapterparity.Adapter{
		Name: "editboundary",
		Tool: "edit.apply",
		NotApplicable: map[string]string{
			"nil_pipeline": "a nil Pipeline means the built-in preview policy, which `boundary edit apply` relies on",
		},
		Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
			applier := &countingApplier{}
			patch := docsPatch
			if in == adapterparity.Malformed {
				patch = "not a patch\n"
			}
			res, err := editboundary.ApplyExecutor{Pipeline: p, Applier: applier}.Apply(ctx, editboundary.ApplyRequest{
				Patch: []byte(patch), CWD: t.TempDir(), RecordPath: filepath.Join(t.TempDir(), "records.jsonl"),
			})
			out := adapterparity.Outcome{Executed: applier.calls}
			if err == nil {
				out.Action = res.Record.Action
			}
			return out
		},
	})
}

type countingApplier struct{ calls int }

func (a *countingApplier) Apply(string, []byte) (editboundary.ApplySummary, error) {
	a.calls++
	return editboundary.ApplySummary{}, nil
}
