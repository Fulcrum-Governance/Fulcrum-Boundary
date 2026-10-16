package adapterparity

// KnownGaps lists rows that fail on main, keyed "adapter/case". Each entry is
// deleted by the change that fixes it.
var KnownGaps = map[string]string{
	// The route passes the upstream result through unchanged, so a warn looks
	// like an allow to the caller. The route's own profile has deny rules
	// only, so warn cannot happen there today.
	"securegithub-mcp-route/warn": "forwarded result carries no verdict",
}
