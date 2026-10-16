package adapterparity

// KnownGaps lists rows that fail on main, keyed "adapter/case". Each entry is
// deleted by the change that fixes it.
var KnownGaps = gaps(
	group("blocked responses always report x-fulcrum-action=deny",
		blockedRows("cli")...,
	),
	group("blocked responses always report x-fulcrum-action=deny",
		blockedRows("codeexec")...,
	),
	group("a missing pipeline is reported as a policy deny, not check_indeterminate",
		"a2a/nil_pipeline",
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

func blockedRows(adapter string) []string {
	var rows []string
	for _, c := range Cases() {
		if c.Name == "unknown_static_rule_action" {
			continue
		}
		if c.Want != "" && c.Want != "allow" && c.Want != "warn" && c.Want != "deny" {
			rows = append(rows, adapter+"/"+c.Name)
		}
	}
	return rows
}

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
