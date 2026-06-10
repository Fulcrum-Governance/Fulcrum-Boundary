package policyeval

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Evaluator provides thread-safe policy evaluation with a single yaml.v3 import
// for optional YAML schema parsing and no other infrastructure dependencies.
// It is designed to be embedded in MCP proxies, SDKs, and the Fulcrum server.
type Evaluator struct {
	// policies is the in-memory policy set, sorted by priority (highest first).
	policies []*Policy

	// strictErr is non-nil when strict mode is enabled and the most recently
	// loaded policy set failed validation. While set, Evaluate fails closed
	// (denies) instead of evaluating a known-invalid policy set. It is guarded
	// by mu alongside policies.
	strictErr error

	// mu protects concurrent access to policies and strictErr.
	mu sync.RWMutex

	// Configuration
	maxEvaluationTime    time.Duration
	logger               Logger
	externalCallsEnabled bool
	stopOnDeny           bool
	strictPolicies       bool
}

// NewEvaluator creates a new policy evaluator with the provided policies.
//
// By default invalid policies are not rejected here: a malformed rule (bad
// regex, unknown field, unsupported type) is skipped at evaluation time with a
// Warn-level log, and evaluation continues with the remaining rules. This keeps
// behavior backward-compatible but visible. To reject invalid policies up front
// and propagate the error, use NewEvaluatorStrict, or pass WithStrictPolicies
// (which makes this constructor fail closed — see WithStrictPolicies — without
// changing the signature). Embedders that cannot tolerate silent skips should
// call ValidateAllPolicies (or use the strict path) before serving traffic.
func NewEvaluator(policies []*Policy, opts ...Option) *Evaluator {
	e := &Evaluator{
		maxEvaluationTime:    10 * time.Millisecond,
		logger:               noopLogger{},
		externalCallsEnabled: false,
		stopOnDeny:           true,
	}

	for _, opt := range opts {
		opt(e)
	}

	e.UpdatePolicies(policies)
	return e
}

// NewEvaluatorStrict creates an evaluator and validates the provided policies
// (including compiling every regex pattern) before returning. If any policy is
// invalid it returns a nil-safe evaluator together with the validation error,
// so a typo'd deny rule fails loudly at construction instead of silently
// allowing requests at evaluation time. WithStrictPolicies is implied; passing
// it again is harmless.
//
// The returned evaluator is non-nil even on error so callers may inspect it,
// but when err != nil it holds the (rejected) policy set and fails closed
// (denies) on Evaluate until UpdatePoliciesStrict succeeds.
func NewEvaluatorStrict(policies []*Policy, opts ...Option) (*Evaluator, error) {
	e := &Evaluator{
		maxEvaluationTime:    10 * time.Millisecond,
		logger:               noopLogger{},
		externalCallsEnabled: false,
		stopOnDeny:           true,
		strictPolicies:       true,
	}

	for _, opt := range opts {
		opt(e)
	}
	e.strictPolicies = true // enforce regardless of option ordering

	return e, e.UpdatePoliciesStrict(policies)
}

// UpdatePolicies replaces the policy set with a new set.
// This is used for cache synchronization from the server.
// Policies are sorted by priority (highest first) for correct evaluation order.
//
// In strict mode (WithStrictPolicies) this validates the new set and, if it is
// invalid, logs the error and arms the fail-closed state so that subsequent
// Evaluate calls deny until a valid set is loaded. The signature is unchanged
// for backward compatibility; use UpdatePoliciesStrict to receive the error
// directly.
func (e *Evaluator) UpdatePolicies(policies []*Policy) {
	if err := e.UpdatePoliciesStrict(policies); err != nil {
		// Non-strict callers cannot receive this error; UpdatePoliciesStrict has
		// already logged it and armed the fail-closed state when strict.
		_ = err
	}
}

// UpdatePoliciesStrict replaces the policy set and reports any validation error.
//
// The policies are always installed (sorted by priority) so the evaluator's
// loaded state is observable. The return value reflects validation:
//   - In strict mode, an invalid set returns the validation error AND arms the
//     fail-closed state (Evaluate denies until a valid set is loaded).
//   - In non-strict mode, validation still runs and the error is returned for
//     the caller to inspect, but the fail-closed state is NOT armed and
//     Evaluate keeps its default skip-with-Warn behavior.
//
// A nil/empty policy set is always valid (it allows everything by default).
func (e *Evaluator) UpdatePoliciesStrict(policies []*Policy) error {
	// Sort by priority descending (higher priority = evaluated first)
	sorted := make([]*Policy, len(policies))
	copy(sorted, policies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	validationErr := validatePolicySet(sorted)

	e.mu.Lock()
	e.policies = sorted
	if e.strictPolicies {
		e.strictErr = validationErr
	} else {
		e.strictErr = nil
	}
	e.mu.Unlock()

	if validationErr != nil && e.strictPolicies {
		e.logger.Warn("strict policy validation failed; evaluator will fail closed (deny) until a valid policy set is loaded",
			Field{Key: "error", Value: validationErr.Error()})
	}

	return validationErr
}

// ValidateAllPolicies validates every currently-loaded policy, including
// compiling each regex pattern. It returns the first validation error, or nil
// if the whole set is valid. Embedders can call this after NewEvaluator (or
// after UpdatePolicies) to detect a typo'd rule that would otherwise be
// silently skipped at evaluation time. It does not change the loaded set or the
// fail-closed state.
func (e *Evaluator) ValidateAllPolicies() error {
	e.mu.RLock()
	policies := e.policies
	e.mu.RUnlock()
	return validatePolicySet(policies)
}

// validatePolicySet validates each policy in the set, returning the first error.
// A nil or empty set is valid.
func validatePolicySet(policies []*Policy) error {
	for _, policy := range policies {
		if err := ValidatePolicy(policy); err != nil {
			id := "<nil>"
			if policy != nil {
				id = policy.PolicyId
			}
			return fmt.Errorf("policy %q invalid: %w", id, err)
		}
	}
	return nil
}

// Policies returns a copy of the current policy set.
func (e *Evaluator) Policies() []*Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Policy, len(e.policies))
	copy(result, e.policies)
	return result
}

// PolicyCount returns the number of loaded policies.
func (e *Evaluator) PolicyCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.policies)
}

// Evaluate evaluates all applicable policies against the request context.
// Returns a Decision indicating whether the action should be allowed, denied, or escalated.
func (e *Evaluator) Evaluate(ctx context.Context, req *EvaluationRequest) (*Decision, error) {
	if req == nil {
		return nil, fmt.Errorf("evaluation request is nil")
	}

	startTime := time.Now()
	evalCtx := req.ToProtoContext()

	e.mu.RLock()
	policies := e.policies
	strictErr := e.strictErr
	e.mu.RUnlock()

	// Strict mode fail-closed: if the loaded policy set failed validation we
	// refuse to evaluate it (a partially-skipped invalid set could allow a
	// request a corrected policy would deny) and deny instead.
	if strictErr != nil {
		e.logger.Warn("denying request: strict policy validation failed",
			Field{Key: "error", Value: strictErr.Error()})
		return &Decision{
			Action:               ActionDeny,
			Reason:               fmt.Sprintf("strict policy validation failed: %v", strictErr),
			EvaluationDurationMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	var allActions []*PolicyAction
	var matchedRules []*RuleMatch
	var matchedPolicy *Policy
	finalAction := ActionAllow
	var escalationReason string

policyLoop:
	for _, policy := range policies {
		// Skip non-active policies
		if policy.Status != PolicyStatus_POLICY_STATUS_ACTIVE {
			continue
		}

		// Check if policy applies to this context
		if !e.policyApplies(policy, evalCtx) {
			continue
		}

		// Evaluate the policy
		decision, err := e.evaluatePolicy(ctx, policy, evalCtx)
		if err != nil {
			// Surface the skip at Warn (visible on a default slog logger): a
			// faulting policy is silently dropped here, which can turn a typo'd
			// deny rule into an allow. Strict mode (WithStrictPolicies) rejects
			// such policies up front instead.
			e.logger.Warn("policy evaluation error; policy skipped (request may be allowed by default)",
				Field{Key: "policy_id", Value: policy.PolicyId},
				Field{Key: "error", Value: err.Error()})
			continue
		}

		// Collect matched rules and actions
		matchedRules = append(matchedRules, decision.MatchedRules...)
		allActions = append(allActions, decision.Actions...)

		// Update final action based on precedence
		switch decision.Action {
		case ActionDeny:
			finalAction = ActionDeny
			matchedPolicy = policy
			if e.stopOnDeny {
				break policyLoop
			}
		case ActionEscalate:
			if finalAction != ActionDeny {
				finalAction = ActionEscalate
				escalationReason = decision.EscalationReason
				if matchedPolicy == nil {
					matchedPolicy = policy
				}
			}
		case ActionRequireApproval:
			if finalAction != ActionDeny && finalAction != ActionEscalate {
				finalAction = ActionRequireApproval
				if matchedPolicy == nil {
					matchedPolicy = policy
				}
			}
		case ActionWarn:
			if finalAction == ActionAllow {
				finalAction = ActionWarn
				if matchedPolicy == nil {
					matchedPolicy = policy
				}
			}
		}
	}

	duration := time.Since(startTime)

	// Warn if evaluation took too long
	if duration > e.maxEvaluationTime {
		e.logger.Warn("policy evaluation exceeded time limit",
			Field{Key: "duration_ms", Value: duration.Milliseconds()},
			Field{Key: "limit_ms", Value: e.maxEvaluationTime.Milliseconds()})
	}

	reason := fmt.Sprintf("Evaluated %d policies, %d rules matched", len(policies), len(matchedRules))
	if len(matchedRules) == 0 {
		reason = "No rules matched - action allowed by default"
	}

	return &Decision{
		Action:               finalAction,
		MatchedPolicy:        matchedPolicy,
		MatchedRules:         matchedRules,
		Actions:              allActions,
		Reason:               reason,
		EvaluationDurationMs: duration.Milliseconds(),
		EscalationReason:     escalationReason,
	}, nil
}

// EvaluatePolicy evaluates a single policy against the context.
func (e *Evaluator) EvaluatePolicy(ctx context.Context, policy *Policy, req *EvaluationRequest) (*Decision, error) {
	if policy == nil {
		return nil, fmt.Errorf("policy is nil")
	}
	if req == nil {
		return nil, fmt.Errorf("evaluation request is nil")
	}

	return e.evaluatePolicy(ctx, policy, req.ToProtoContext())
}

// evaluatePolicy is the internal implementation.
func (e *Evaluator) evaluatePolicy(ctx context.Context, policy *Policy, evalCtx *EvaluationContext) (*Decision, error) {
	startTime := time.Now()

	// Validate inputs
	if policy.Status != PolicyStatus_POLICY_STATUS_ACTIVE {
		return &Decision{
			Action: ActionAllow,
			Reason: fmt.Sprintf("Policy %s is not active (status: %s)", policy.PolicyId, policy.Status),
		}, nil
	}

	// Evaluate rules in order
	var matchedRules []*RuleMatch
	var actions []*PolicyAction
	finalAction := ActionAllow
	var escalationReason string

	for _, rule := range policy.Rules {
		if !rule.Enabled {
			continue
		}

		// Evaluate all conditions in the rule
		ruleMatches, escalate, escReason, err := e.evaluateRule(ctx, rule, evalCtx)
		if err != nil {
			// Surface the skip at Warn (visible on a default slog logger). A
			// condition fault (invalid regex, unknown field, unsupported type)
			// drops this rule, so a typo'd deny rule silently never fires and
			// the request can be allowed by default. Strict mode
			// (WithStrictPolicies / NewEvaluatorStrict) rejects such rules up
			// front instead of skipping them here.
			e.logger.Warn("rule evaluation error; rule skipped (a deny rule may silently not fire)",
				Field{Key: "rule_id", Value: rule.RuleId},
				Field{Key: "error", Value: err.Error()})
			continue
		}

		// Handle escalation (semantic condition requires phone-home)
		if escalate {
			finalAction = ActionEscalate
			escalationReason = escReason
			matchedRules = append(matchedRules, &RuleMatch{
				RuleID:   rule.RuleId,
				RuleName: rule.Name,
				Priority: rule.Priority,
			})
			break // Escalation takes priority
		}

		if ruleMatches {
			matchedRules = append(matchedRules, &RuleMatch{
				RuleID:   rule.RuleId,
				RuleName: rule.Name,
				Priority: rule.Priority,
			})

			actions = append(actions, rule.Actions...)

			// Determine action from rule actions
			for _, action := range rule.Actions {
				switch action.ActionType {
				case PolicyActionType_ACTION_TYPE_DENY:
					finalAction = ActionDeny
				case PolicyActionType_ACTION_TYPE_WARN:
					if finalAction == ActionAllow {
						finalAction = ActionWarn
					}
				case PolicyActionType_ACTION_TYPE_REQUIRE_APPROVAL:
					if finalAction != ActionDeny {
						finalAction = ActionRequireApproval
					}
				}

				// Stop if terminal action
				if action.Terminal {
					goto done
				}
			}
		}
	}

done:
	duration := time.Since(startTime)

	reason := "No rules matched"
	if len(matchedRules) > 0 {
		reason = fmt.Sprintf("%d rule(s) matched", len(matchedRules))
	}

	return &Decision{
		Action:               finalAction,
		MatchedPolicy:        policy,
		MatchedRules:         matchedRules,
		Actions:              actions,
		Reason:               reason,
		EvaluationDurationMs: duration.Milliseconds(),
		EscalationReason:     escalationReason,
	}, nil
}

// evaluateRule evaluates all conditions in a rule.
// Returns (matches, needsEscalation, escalationReason, error).
func (e *Evaluator) evaluateRule(ctx context.Context, rule *PolicyRule, evalCtx *EvaluationContext) (matches, needsEscalation bool, escalationReason string, err error) {
	if len(rule.Conditions) == 0 {
		// Rule with no conditions always matches
		return true, false, "", nil
	}

	// All conditions must match (implicit AND)
	for _, condition := range rule.Conditions {
		// Semantic conditions require escalation (phone home to server with LLM)
		if condition.ConditionType == ConditionType_CONDITION_TYPE_SEMANTIC {
			return false, true, fmt.Sprintf("rule %s has semantic condition requiring LLM evaluation", rule.RuleId), nil
		}

		// External calls may be disabled
		if condition.ConditionType == ConditionType_CONDITION_TYPE_EXTERNAL_CALL && !e.externalCallsEnabled {
			return false, true, fmt.Sprintf("rule %s has external call condition (disabled in this context)", rule.RuleId), nil
		}

		matches, err := EvaluateCondition(condition, evalCtx, e.externalCallsEnabled)
		if err != nil {
			return false, false, "", err
		}
		if !matches {
			return false, false, "", nil // Short-circuit on first non-match
		}
	}

	return true, false, "", nil
}

// policyApplies checks if a policy applies to the given evaluation context.
func (e *Evaluator) policyApplies(policy *Policy, ctx *EvaluationContext) bool {
	if policy.Scope == nil {
		return true // No scope means applies to everything
	}

	scope := policy.Scope

	// Check if applies to all
	if scope.ApplyToAll {
		return true
	}

	// Check workflow
	if len(scope.WorkflowIds) > 0 {
		found := false
		for _, wf := range scope.WorkflowIds {
			if wf == ctx.WorkflowId {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check phase
	if len(scope.Phases) > 0 {
		found := false
		for _, phase := range scope.Phases {
			if phase == ctx.Phase {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check roles
	if len(scope.Roles) > 0 {
		userRoleSet := make(map[string]struct{}, len(ctx.UserRoles))
		for _, userRole := range ctx.UserRoles {
			userRoleSet[userRole] = struct{}{}
		}
		found := false
		for _, role := range scope.Roles {
			if _, exists := userRoleSet[role]; exists {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check models
	if len(scope.ModelIds) > 0 {
		found := false
		for _, model := range scope.ModelIds {
			if model == ctx.ModelId {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tools
	if len(scope.ToolNames) > 0 {
		ctxToolSet := make(map[string]struct{}, len(ctx.ToolNames))
		for _, ctxTool := range ctx.ToolNames {
			ctxToolSet[ctxTool] = struct{}{}
		}
		found := false
		for _, tool := range scope.ToolNames {
			if _, exists := ctxToolSet[tool]; exists {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// ValidatePolicy validates a policy definition for correctness.
func ValidatePolicy(policy *Policy) error {
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}

	if policy.PolicyId == "" {
		return fmt.Errorf("policy_id is required")
	}

	if policy.TenantId == "" {
		return fmt.Errorf("tenant_id is required")
	}

	if len(policy.Rules) == 0 {
		return fmt.Errorf("policy must have at least one rule")
	}

	for i, rule := range policy.Rules {
		if err := ValidateRule(rule); err != nil {
			return fmt.Errorf("rule %d (%s) invalid: %w", i, rule.RuleId, err)
		}
	}

	return nil
}

// ValidateRule validates a policy rule for correctness.
func ValidateRule(rule *PolicyRule) error {
	if rule == nil {
		return fmt.Errorf("rule is nil")
	}

	if rule.RuleId == "" {
		return fmt.Errorf("rule_id is required")
	}

	if len(rule.Actions) == 0 {
		return fmt.Errorf("rule must have at least one action")
	}

	for i, condition := range rule.Conditions {
		if err := ValidateCondition(condition); err != nil {
			return fmt.Errorf("condition %d invalid: %w", i, err)
		}
	}

	return nil
}

// ValidateCondition validates a condition for correctness.
func ValidateCondition(condition *PolicyCondition) error {
	if condition == nil {
		return fmt.Errorf("condition is nil")
	}

	// Logical conditions must have nested conditions
	if condition.ConditionType == ConditionType_CONDITION_TYPE_LOGICAL {
		if len(condition.NestedConditions) == 0 {
			return fmt.Errorf("logical condition must have nested conditions")
		}
		for i, nested := range condition.NestedConditions {
			if err := ValidateCondition(nested); err != nil {
				return fmt.Errorf("nested condition %d invalid: %w", i, err)
			}
		}
		return nil
	}

	// Non-logical conditions must reference a field (except external call, whose
	// Field names a key in the HTTP response body rather than a context field).
	// For everything else the field must be a well-formed, resolvable path: a
	// typo'd field never matches at runtime, which would silently weaken a deny
	// rule, so we reject it here.
	if condition.ConditionType != ConditionType_CONDITION_TYPE_EXTERNAL_CALL {
		if condition.Field == "" {
			return fmt.Errorf("condition field is required")
		}
		if err := validateFieldPath(condition.Field); err != nil {
			return fmt.Errorf("condition field invalid: %w", err)
		}
	}

	// IN/NOT_IN conditions must have values list
	if condition.Operator == ConditionOperator_CONDITION_OPERATOR_IN ||
		condition.Operator == ConditionOperator_CONDITION_OPERATOR_NOT_IN {
		if len(condition.Values) == 0 {
			return fmt.Errorf("IN/NOT_IN conditions require values list")
		}
	}

	// REGEX conditions must carry a string value that compiles. We compile via
	// the same cache the evaluator uses, so a pattern that validates here is the
	// exact pattern that will run at evaluation time (and the cache is warmed as
	// a side effect). Catching a bad pattern here is what prevents the
	// "typo in a deny rule => rule silently skipped => request allowed" failure.
	if condition.ConditionType == ConditionType_CONDITION_TYPE_REGEX {
		strVal, ok := condition.Value.(*PolicyCondition_StringValue)
		if !ok {
			return fmt.Errorf("regex condition requires a string value")
		}
		if strVal.StringValue == "" {
			return fmt.Errorf("regex condition requires a non-empty pattern")
		}
		if _, err := getCompiledRegex(strVal.StringValue); err != nil {
			return fmt.Errorf("invalid regex pattern %q: %w", strVal.StringValue, err)
		}
	}

	return nil
}
