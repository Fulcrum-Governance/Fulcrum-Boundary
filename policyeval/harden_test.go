package policyeval

import (
	"context"
	"testing"
)

// getDebugs returns a copy of the Debug-level messages captured by mockLogger.
func (m *mockLogger) getDebugs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.debugs))
	copy(out, m.debugs)
	return out
}

// regexCondition builds a REGEX condition over input.text with the given pattern.
func regexCondition(pattern string) *PolicyCondition {
	return &PolicyCondition{
		ConditionType: ConditionType_CONDITION_TYPE_REGEX,
		Field:         "input.text",
		Operator:      ConditionOperator_CONDITION_OPERATOR_MATCHES,
		Value:         &PolicyCondition_StringValue{StringValue: pattern},
	}
}

// badRegexDenyPolicy is the canonical "typo in a deny rule" fixture: a deny rule
// whose only condition is an uncompilable regex.
func badRegexDenyPolicy() *Policy {
	return newTestPolicy("bad-regex", "tenant1", 100, PolicyStatus_POLICY_STATUS_ACTIVE, []*PolicyRule{
		newTestRule("r-bad", true, []*PolicyCondition{
			regexCondition("[invalid"), // missing closing bracket — never compiles
		}, []*PolicyAction{
			newDenyAction(false, "should deny on match"),
		}),
	})
}

// --- (a) ValidateCondition compiles regex patterns ---

func TestValidateCondition_RejectsInvalidRegex(t *testing.T) {
	cond := regexCondition("[invalid") // unterminated character class

	err := ValidateCondition(cond)
	if err == nil {
		t.Fatal("expected ValidateCondition to reject an uncompilable regex, got nil")
	}
	if !contains(err.Error(), "invalid regex") {
		t.Errorf("expected error to mention invalid regex, got %q", err.Error())
	}
}

func TestValidateCondition_AcceptsValidRegex(t *testing.T) {
	cond := regexCondition(`^drop\s+table`) // compiles fine

	if err := ValidateCondition(cond); err != nil {
		t.Errorf("expected a valid regex to pass validation, got %v", err)
	}
}

func TestValidateCondition_RejectsRegexWithNonStringValue(t *testing.T) {
	cond := &PolicyCondition{
		ConditionType: ConditionType_CONDITION_TYPE_REGEX,
		Field:         "input.text",
		Operator:      ConditionOperator_CONDITION_OPERATOR_MATCHES,
		Value:         &PolicyCondition_IntValue{IntValue: 7}, // wrong value kind
	}

	err := ValidateCondition(cond)
	if err == nil {
		t.Fatal("expected ValidateCondition to reject a REGEX condition with a non-string value")
	}
	if !contains(err.Error(), "string value") {
		t.Errorf("expected error to mention string value, got %q", err.Error())
	}
}

func TestValidateCondition_RejectsEmptyRegexPattern(t *testing.T) {
	cond := &PolicyCondition{
		ConditionType: ConditionType_CONDITION_TYPE_REGEX,
		Field:         "input.text",
		Operator:      ConditionOperator_CONDITION_OPERATOR_MATCHES,
		Value:         &PolicyCondition_StringValue{StringValue: ""},
	}

	if err := ValidateCondition(cond); err == nil {
		t.Fatal("expected ValidateCondition to reject an empty regex pattern")
	}
}

func TestValidateCondition_RejectsInvalidRegexNestedInLogical(t *testing.T) {
	// The bad regex is buried inside an AND/OR tree; validation must recurse.
	cond := &PolicyCondition{
		ConditionType:   ConditionType_CONDITION_TYPE_LOGICAL,
		LogicalOperator: LogicalOperator_LOGICAL_OPERATOR_AND,
		NestedConditions: []*PolicyCondition{
			newFieldMatchCondition("user.id", "u1", ConditionOperator_CONDITION_OPERATOR_EQUALS),
			{
				ConditionType:   ConditionType_CONDITION_TYPE_LOGICAL,
				LogicalOperator: LogicalOperator_LOGICAL_OPERATOR_OR,
				NestedConditions: []*PolicyCondition{
					regexCondition("(unclosed"),
				},
			},
		},
	}

	err := ValidateCondition(cond)
	if err == nil {
		t.Fatal("expected nested invalid regex to be rejected")
	}
	if !contains(err.Error(), "invalid regex") {
		t.Errorf("expected error to mention invalid regex, got %q", err.Error())
	}
}

// --- (a') ValidateCondition validates field paths (bad-field) ---

func TestValidateCondition_RejectsUnknownField(t *testing.T) {
	cond := newFieldMatchCondition("user.idd", "x", ConditionOperator_CONDITION_OPERATOR_EQUALS) // typo

	err := ValidateCondition(cond)
	if err == nil {
		t.Fatal("expected ValidateCondition to reject an unknown field path")
	}
	if !contains(err.Error(), "unknown field") {
		t.Errorf("expected error to mention unknown field, got %q", err.Error())
	}
}

func TestValidateCondition_RejectsMalformedFieldPath(t *testing.T) {
	cond := newFieldMatchCondition("inputtext", "x", ConditionOperator_CONDITION_OPERATOR_EQUALS) // no dot

	if err := ValidateCondition(cond); err == nil {
		t.Fatal("expected ValidateCondition to reject a malformed field path")
	}
}

func TestValidateCondition_AcceptsAttributeField(t *testing.T) {
	cond := newFieldMatchCondition("attribute.custom_metric", "x", ConditionOperator_CONDITION_OPERATOR_EQUALS)

	if err := ValidateCondition(cond); err != nil {
		t.Errorf("expected attribute.* field to pass validation, got %v", err)
	}
}

// --- (b) strict mode returns an error instead of silently allowing ---

func TestStrictMode_RejectsBadRegexPolicy(t *testing.T) {
	e, err := NewEvaluatorStrict([]*Policy{badRegexDenyPolicy()})
	if err == nil {
		t.Fatal("expected NewEvaluatorStrict to return an error for a bad-regex deny rule")
	}
	if !contains(err.Error(), "invalid regex") {
		t.Errorf("expected error to mention invalid regex, got %q", err.Error())
	}

	// And the evaluator must fail closed: the request the (broken) deny rule was
	// meant to block is DENIED, not silently allowed.
	decision, evalErr := e.Evaluate(context.Background(), &EvaluationRequest{
		TenantID:  "tenant1",
		InputText: "anything",
	})
	if evalErr != nil {
		t.Fatalf("unexpected eval error: %v", evalErr)
	}
	if decision.Action != ActionDeny {
		t.Errorf("strict mode must fail closed (deny) on an invalid policy set, got %v", decision.Action)
	}
}

func TestStrictMode_RejectsBadFieldPolicy(t *testing.T) {
	policy := newTestPolicy("bad-field", "tenant1", 100, PolicyStatus_POLICY_STATUS_ACTIVE, []*PolicyRule{
		newTestRule("r1", true, []*PolicyCondition{
			newFieldMatchCondition("user.idd", "blocked", ConditionOperator_CONDITION_OPERATOR_EQUALS),
		}, []*PolicyAction{
			newDenyAction(false, "block typo'd user field"),
		}),
	})

	_, err := NewEvaluatorStrict([]*Policy{policy})
	if err == nil {
		t.Fatal("expected NewEvaluatorStrict to return an error for an unknown-field deny rule")
	}
	if !contains(err.Error(), "unknown field") {
		t.Errorf("expected error to mention unknown field, got %q", err.Error())
	}
}

func TestStrictMode_OptionOnPlainConstructorFailsClosed(t *testing.T) {
	// WithStrictPolicies on the non-error NewEvaluator cannot return an error,
	// but it must still fail closed (deny) and log the failure at Warn.
	logger := &mockLogger{}
	e := NewEvaluator([]*Policy{badRegexDenyPolicy()}, WithStrictPolicies(), WithLogger(logger))

	decision, err := e.Evaluate(context.Background(), &EvaluationRequest{
		TenantID:  "tenant1",
		InputText: "anything",
	})
	if err != nil {
		t.Fatalf("unexpected eval error: %v", err)
	}
	if decision.Action != ActionDeny {
		t.Errorf("expected strict fail-closed deny, got %v", decision.Action)
	}
	if len(logger.getWarnings()) == 0 {
		t.Error("expected a Warn log describing the strict validation failure")
	}
}

func TestStrictMode_ValidPolicyEvaluatesNormally(t *testing.T) {
	// A valid policy under strict mode must behave exactly like normal mode.
	policy := newTestPolicy("good", "tenant1", 100, PolicyStatus_POLICY_STATUS_ACTIVE, []*PolicyRule{
		newTestRule("r1", true, []*PolicyCondition{
			regexCondition("(?i)drop table"),
		}, []*PolicyAction{
			newDenyAction(false, "blocked"),
		}),
	})

	e, err := NewEvaluatorStrict([]*Policy{policy})
	if err != nil {
		t.Fatalf("expected valid policy to pass strict validation, got %v", err)
	}

	// Matches -> deny.
	deny, _ := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1", InputText: "please DROP TABLE users"})
	if deny.Action != ActionDeny {
		t.Errorf("expected deny on match, got %v", deny.Action)
	}
	// No match -> allow.
	allow, _ := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1", InputText: "harmless"})
	if allow.Action != ActionAllow {
		t.Errorf("expected allow on no match, got %v", allow.Action)
	}
}

func TestStrictMode_RecoversAfterValidUpdate(t *testing.T) {
	// Start invalid (armed fail-closed), then load a valid set: the evaluator
	// must clear the fail-closed state and resume normal evaluation.
	e, err := NewEvaluatorStrict([]*Policy{badRegexDenyPolicy()})
	if err == nil {
		t.Fatal("expected initial strict validation to fail")
	}

	denied, _ := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1"})
	if denied.Action != ActionDeny {
		t.Fatalf("expected fail-closed deny before recovery, got %v", denied.Action)
	}

	good := newTestPolicy("good", "tenant1", 100, PolicyStatus_POLICY_STATUS_ACTIVE, []*PolicyRule{
		newTestRule("r1", true, nil, []*PolicyAction{newWarnAction(false, "warn")}),
	})
	if err := e.UpdatePoliciesStrict([]*Policy{good}); err != nil {
		t.Fatalf("expected valid update to succeed, got %v", err)
	}

	got, _ := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1"})
	if got.Action != ActionWarn {
		t.Errorf("expected normal evaluation (warn) after recovery, got %v", got.Action)
	}
}

func TestStrictMode_UpdatePoliciesStrictArmsFailClosed(t *testing.T) {
	// A valid evaluator that is later fed an invalid set via UpdatePoliciesStrict
	// must arm the fail-closed state.
	e, err := NewEvaluatorStrict(nil)
	if err != nil {
		t.Fatalf("nil policy set should be valid, got %v", err)
	}

	if err := e.UpdatePoliciesStrict([]*Policy{badRegexDenyPolicy()}); err == nil {
		t.Fatal("expected UpdatePoliciesStrict to report the invalid set")
	}

	decision, _ := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1"})
	if decision.Action != ActionDeny {
		t.Errorf("expected fail-closed deny after invalid strict update, got %v", decision.Action)
	}
}

// --- (c) default (non-strict) mode still evaluates AND logs a Warn ---

func TestDefaultMode_SkipsFaultingRuleAllowsAndWarns(t *testing.T) {
	logger := &mockLogger{}
	// Default constructor: NO WithStrictPolicies.
	e := NewEvaluator([]*Policy{badRegexDenyPolicy()}, WithLogger(logger))

	decision, err := e.Evaluate(context.Background(), &EvaluationRequest{
		TenantID:  "tenant1",
		InputText: "anything",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Backward compatibility: default mode still returns nil error and ALLOWS
	// (the faulting deny rule is skipped, not enforced).
	if decision.Action != ActionAllow {
		t.Errorf("expected default mode to allow (skip the faulting rule), got %v", decision.Action)
	}

	// But the skip is now VISIBLE at Warn (it used to be a silent Debug).
	warnings := logger.getWarnings()
	if len(warnings) == 0 {
		t.Fatal("expected a Warn log for the skipped faulting rule, got none")
	}
	var found bool
	for _, w := range warnings {
		if contains(w, "rule evaluation error") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a 'rule evaluation error' Warn, got %v", warnings)
	}

	// The faulting-rule skip must NOT also be logged at Debug anymore (the level
	// was deliberately raised so default loggers see it).
	for _, d := range logger.getDebugs() {
		if contains(d, "rule evaluation error") {
			t.Errorf("rule evaluation error should be logged at Warn, not Debug; got Debug %q", d)
		}
	}
}

func TestDefaultMode_NotArmedFailClosed(t *testing.T) {
	// Without WithStrictPolicies, an invalid policy must NOT arm fail-closed:
	// every request is evaluated (and here allowed) rather than blanket-denied.
	e := NewEvaluator([]*Policy{badRegexDenyPolicy()})

	decision, err := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1", InputText: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Action != ActionAllow {
		t.Errorf("default mode must not fail closed; expected allow, got %v", decision.Action)
	}
}

// --- ValidateAllPolicies surfaces problems without strict mode ---

func TestValidateAllPolicies_DetectsBadRegexInDefaultMode(t *testing.T) {
	// An embedder can opt into up-front detection without strict mode by calling
	// ValidateAllPolicies after construction.
	e := NewEvaluator([]*Policy{badRegexDenyPolicy()})

	if err := e.ValidateAllPolicies(); err == nil {
		t.Fatal("expected ValidateAllPolicies to detect the bad regex")
	}

	// It must not change behavior: evaluation still allows (no fail-closed armed).
	decision, _ := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1", InputText: "x"})
	if decision.Action != ActionAllow {
		t.Errorf("ValidateAllPolicies must not arm fail-closed in default mode, got %v", decision.Action)
	}
}

func TestValidateAllPolicies_PassesForValidSet(t *testing.T) {
	policy := newTestPolicy("ok", "tenant1", 100, PolicyStatus_POLICY_STATUS_ACTIVE, []*PolicyRule{
		newTestRule("r1", true, []*PolicyCondition{regexCondition("^ok$")}, []*PolicyAction{newAllowAction(false)}),
	})
	e := NewEvaluator([]*Policy{policy})
	if err := e.ValidateAllPolicies(); err != nil {
		t.Errorf("expected valid set to pass ValidateAllPolicies, got %v", err)
	}
}
