package governance

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// StaticPolicyRule defines a simple allow/deny/warn rule evaluated by Stage 2
// of the governance pipeline (governance/pipeline.go), before the full
// PolicyEval engine runs. A rule names a tool (exact, "*"/"" wildcard, or a
// path.Match glob), an action (allow/deny/warn/audit/escalate/require_approval),
// and an optional set of field matchers (Match plus Conditions). The launch
// policy surface is intentionally narrower than the PolicyEval DSL: the matcher
// set is fixed (see StaticPolicyMatch) and there is no boolean nesting — a rule
// fires only when the tool, transport, tenant/agent scope, AND every configured
// matcher all hold. It is loaded from YAML (governance/yaml_policy.go) or
// constructed programmatically and supplied via PipelineConfig.StaticPolicies.
type StaticPolicyRule struct {
	Name         string              `json:"name" yaml:"name"`
	Tool         string              `json:"tool" yaml:"tool"`
	Action       string              `json:"action" yaml:"action"` // allow, deny, warn, audit, escalate, require_approval
	Reason       string              `json:"reason,omitempty" yaml:"reason,omitempty"`
	Transport    string              `json:"transport,omitempty" yaml:"transport,omitempty"`
	DecisionMode DecisionMode        `json:"decision_mode,omitempty" yaml:"decision_mode,omitempty"`
	TenantScope  []string            `json:"tenant_scope,omitempty" yaml:"tenant_scope,omitempty"`
	AgentScope   []string            `json:"agent_scope,omitempty" yaml:"agent_scope,omitempty"`
	Match        *StaticPolicyMatch  `json:"match,omitempty" yaml:"match,omitempty"`
	Conditions   []StaticPolicyMatch `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	PolicyFile   string              `json:"policy_file,omitempty" yaml:"-"`
	Metadata     map[string]string   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// StaticPolicyMatch is the launch-grade matcher used by YAML static policies.
// It supports typed field checks against GovernanceRequest, including
// arguments.<name>. It remains intentionally smaller than the PolicyEval DSL.
type StaticPolicyMatch struct {
	Type            string   `json:"type,omitempty" yaml:"type,omitempty"`
	Field           string   `json:"field,omitempty" yaml:"field,omitempty"`
	Contains        string   `json:"contains,omitempty" yaml:"contains,omitempty"`
	Value           string   `json:"value,omitempty" yaml:"value,omitempty"`
	Values          []string `json:"values,omitempty" yaml:"values,omitempty"`
	Regex           string   `json:"regex,omitempty" yaml:"regex,omitempty"`
	CaseInsensitive bool     `json:"case_insensitive,omitempty" yaml:"case_insensitive,omitempty"`
}

// matchesRequest reports whether this rule fires for req. The rule matches only
// when ALL of the following hold: the tool pattern matches (toolMatches), the
// Transport (if set) equals the request transport case-insensitively, the
// request tenant is within TenantScope (if set), the request agent is within
// AgentScope (if set), and every configured matcher (Match prepended to
// Conditions) holds. A rule with no matchers and a satisfied scope matches on
// the tool/transport/scope gate alone. A nil request never matches.
func (r StaticPolicyRule) matchesRequest(req *GovernanceRequest) bool {
	if req == nil {
		return false
	}
	if !toolMatches(r.Tool, req.ToolName) {
		return false
	}
	if r.Transport != "" && !strings.EqualFold(r.Transport, string(req.Transport)) {
		return false
	}
	if len(r.TenantScope) > 0 && !stringInList(req.TenantID, r.TenantScope, true) {
		return false
	}
	if len(r.AgentScope) > 0 && !stringInList(req.AgentID, r.AgentScope, true) {
		return false
	}

	matches := r.Conditions
	if r.Match != nil {
		matches = append([]StaticPolicyMatch{*r.Match}, matches...)
	}
	if len(matches) == 0 {
		return true
	}

	for _, match := range matches {
		if !match.matches(req) {
			return false
		}
	}
	return true
}

// matches reports whether this single field matcher holds for req. An empty
// Type defaults to "contains". Supported types: transport_is, agent_in,
// agent_not_in, ast_class, contains, not_contains, equals, not_equals, regex.
//
// Missing data is fail-OPEN-to-non-match: field-comparison matchers (contains,
// not_contains, equals, not_equals, regex) return false when the matcher has no
// needle/pattern, no Field, or the referenced request field is absent — i.e.
// "the data isn't here, so this matcher does not assert a hit", which lets the
// rule fall through rather than fire on incomplete data.
//
// A malformed REGEX (a pattern that does not COMPILE) is treated differently: it
// is a malformed-policy fault, not missing data, and is fail-CLOSED. When the
// pattern's target Field IS present but the pattern cannot compile, matches
// returns true (treats the matcher as hit) so a deny/escalate/require_approval
// rule that depends on it cannot be silently bypassed by shipping an invalid
// pattern. (If the Field is absent or the pattern is empty, the missing-data
// rule above still applies and matches returns false.) YAML-loaded policies
// reject non-compiling regex at load (policyeval.ValidatePolicyV1YAML; see
// governance/yaml_policy_test.go), so this runtime guard only triggers for rules
// constructed programmatically, which bypass that load-time gate. For every
// VALID regex the behaviour is unchanged: the pattern is compiled (and cached,
// see compiledMatchRegex) and evaluated exactly as MatchString.
func (m StaticPolicyMatch) matches(req *GovernanceRequest) bool {
	matchType := strings.ToLower(strings.TrimSpace(m.Type))
	if matchType == "" {
		matchType = "contains"
	}

	switch matchType {
	case "transport_is":
		needle := firstNonEmpty(m.Value, m.Contains)
		return needle != "" && strings.EqualFold(needle, string(req.Transport))
	case "agent_in":
		return stringInList(req.AgentID, matchValues(m), true)
	case "agent_not_in":
		return !stringInList(req.AgentID, matchValues(m), true)
	case "ast_class":
		field := m.Field
		if field == "" {
			field = "arguments.sql_class"
		}
		got, ok := requestField(req, field)
		return ok && valueMatchesAny(got, matchValues(m), true)
	case "regex":
		// The regex pattern lives in Regex (the canonical field; YAML uses
		// `regex:`), falling back to Value/Contains for hand-built rules. It is
		// resolved here — not via the shared Contains/Value needle below — so a
		// pure-regex matcher reaches this case instead of being short-circuited
		// by the empty-needle guard.
		pattern := firstNonEmpty(m.Regex, m.Value, m.Contains)
		if pattern == "" || m.Field == "" {
			// Missing pattern or unspecified field: no data to assert on -> miss.
			return false
		}
		haystack, ok := requestField(req, m.Field)
		if !ok {
			return false
		}
		if m.CaseInsensitive {
			pattern = "(?i)" + pattern
		}
		re, err := compiledMatchRegex(pattern)
		if err != nil {
			// Fail closed: an uncompilable pattern is a malformed-policy fault,
			// not missing request data. Treat the matcher as hit so a gating
			// rule (deny/escalate/require_approval) is not silently bypassed.
			// See the matches doc comment for the full contract.
			return true
		}
		return re.MatchString(haystack)
	}

	needle := firstNonEmpty(m.Contains, m.Value)
	if needle == "" || m.Field == "" {
		return false
	}
	haystack, ok := requestField(req, m.Field)
	if !ok {
		return false
	}
	switch matchType {
	case "contains":
		return contains(haystack, needle, m.CaseInsensitive)
	case "not_contains":
		return !contains(haystack, needle, m.CaseInsensitive)
	case "equals":
		return equalString(haystack, needle, m.CaseInsensitive)
	case "not_equals":
		return !equalString(haystack, needle, m.CaseInsensitive)
	default:
		return false
	}
}

// staticRegexCache memoizes compiled static-policy regex patterns. Stage 2 runs
// on the hot path and the same rule set is evaluated for every request, so
// compiling once per distinct pattern avoids repeated regexp.Compile work. It
// mirrors the bounded cache in policyeval (getCompiledRegex). The cache stores
// only successfully compiled patterns; an uncompilable pattern is never cached
// and re-reports its error on each call so the fail-closed branch in matches
// stays deterministic.
var (
	staticRegexCache      sync.Map // pattern string -> *regexp.Regexp
	staticRegexCacheCount int64
	staticRegexCacheMu    sync.Mutex
)

const maxStaticRegexCacheSize = 1000

// compiledMatchRegex returns the compiled form of pattern, caching successful
// compilations. A non-compiling pattern returns the compile error (and is not
// cached); callers decide how to handle that fault.
func compiledMatchRegex(pattern string) (*regexp.Regexp, error) {
	if cached, ok := staticRegexCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	staticRegexCacheMu.Lock()
	if staticRegexCacheCount >= maxStaticRegexCacheSize {
		staticRegexCache.Range(func(key, _ any) bool {
			staticRegexCache.Delete(key)
			return true
		})
		staticRegexCacheCount = 0
	}
	staticRegexCacheCount++
	staticRegexCacheMu.Unlock()

	staticRegexCache.Store(pattern, compiled)
	return compiled, nil
}

// requestField resolves a matcher Field selector against req and reports whether
// the field was present. It maps friendly aliases (e.g. "tool", "risk_class",
// "arguments.sql") onto GovernanceRequest fields and arguments. The bool is
// false when the field is unknown or absent from req; callers treat that as
// "no data to match against".
func requestField(req *GovernanceRequest, field string) (string, bool) {
	if req == nil {
		return "", false
	}
	switch field {
	case "tool", "tool_name":
		return req.ToolName, true
	case "action":
		return req.Action, true
	case "agent_id":
		return req.AgentID, true
	case "tenant_id":
		return req.TenantID, true
	case "risk_class", "sql.class":
		if req.Arguments != nil {
			if value, ok := req.Arguments["sql_class"]; ok {
				return fmt.Sprint(value), true
			}
			if value, ok := req.Arguments["risk_class"]; ok {
				return fmt.Sprint(value), true
			}
		}
		return "", false
	case "transport":
		return string(req.Transport), true
	case "command":
		return req.Command, true
	case "code":
		return req.Code, true
	case "input.text", "arguments.text", "arguments.sql":
		key := strings.TrimPrefix(field, "arguments.")
		key = strings.TrimPrefix(key, "input.")
		if key == "text" && req.Arguments != nil {
			if value, ok := req.Arguments["sql"]; ok {
				return fmt.Sprint(value), true
			}
		}
		if req.Arguments != nil {
			value, ok := req.Arguments[key]
			if ok {
				return fmt.Sprint(value), true
			}
		}
	}

	if strings.HasPrefix(field, "arguments.") {
		key := strings.TrimPrefix(field, "arguments.")
		value, ok := req.Arguments[key]
		if !ok {
			return "", false
		}
		return fmt.Sprint(value), true
	}
	if strings.HasPrefix(field, "input.") {
		key := strings.TrimPrefix(field, "input.")
		value, ok := req.Arguments[key]
		if !ok {
			return "", false
		}
		return fmt.Sprint(value), true
	}
	return "", false
}

// firstNonEmpty returns the first argument whose trimmed form is non-empty, or
// "" if every argument is empty or whitespace.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// contains reports whether needle occurs in haystack, case-insensitively when
// insensitive is true.
func contains(haystack, needle string, insensitive bool) bool {
	if insensitive {
		return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
	}
	return strings.Contains(haystack, needle)
}

// equalString reports whether left and right are equal, case-insensitively when
// insensitive is true.
func equalString(left, right string, insensitive bool) bool {
	if insensitive {
		return strings.EqualFold(left, right)
	}
	return left == right
}

// matchValues collects the candidate values a multi-value matcher (agent_in,
// agent_not_in, ast_class) compares against: every entry in Values, plus the
// scalar Value and Contains when set. Order is Values, then Value, then Contains.
func matchValues(match StaticPolicyMatch) []string {
	values := append([]string{}, match.Values...)
	if match.Value != "" {
		values = append(values, match.Value)
	}
	if match.Contains != "" {
		values = append(values, match.Contains)
	}
	return values
}

// valueMatchesAny reports whether value equals any entry in values, using
// case-insensitive comparison when insensitive is true. An empty value can still
// match an empty candidate; callers that must reject empty values use
// stringInList.
func valueMatchesAny(value string, values []string, insensitive bool) bool {
	for _, candidate := range values {
		if equalString(value, candidate, insensitive) {
			return true
		}
	}
	return false
}

// stringInList reports whether value is non-empty AND equals some entry in
// values (case-insensitively when insensitive is true). An empty value is never
// in the list: this is what makes tenant/agent scoping fail closed for requests
// that carry no tenant or agent identity.
func stringInList(value string, values []string, insensitive bool) bool {
	if value == "" {
		return false
	}
	return valueMatchesAny(value, values, insensitive)
}
