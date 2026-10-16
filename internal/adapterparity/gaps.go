package adapterparity

// KnownGaps lists rows that fail on main, keyed "adapter/case". Each entry is
// deleted by the change that fixes it.
var KnownGaps = gaps(
	group("a missing pipeline is reported as a policy deny, not check_indeterminate",
		"a2a/nil_pipeline",
		"cli/nil_pipeline",
		"codeexec/nil_pipeline",
		"grpc/nil_pipeline",
	),
	group("a missing pipeline panics the handler and the caller sees no verdict",
		"mcp/nil_pipeline",
	),
	group("a missing pipeline returns an error body with no verdict header",
		"webhook-execution/nil_pipeline",
		"webhook-informational/nil_pipeline",
	),
	group("a missing pipeline returns an error and sends no confirmation",
		"managedagents/nil_pipeline",
	),
)

type gapGroup struct {
	reason string
	rows   []string
}

func group(reason string, rows ...string) gapGroup { return gapGroup{reason, rows} }

func gaps(groups ...gapGroup) map[string]string {
	out := map[string]string{}
	for _, g := range groups {
		for _, row := range g.rows {
			out[row] = g.reason
		}
	}
	return out
}
