# policyeval

Portable policy evaluation engine with no infrastructure dependencies (single `yaml.v3` import for optional YAML schema). Designed to be embedded in MCP proxies, SDKs, and any Go service that needs to govern agent tool calls before execution. Shipped as part of Fulcrum Boundary.

## Overview

The `policyeval` package provides a thread-safe policy evaluator that operates entirely in-memory with **no infrastructure dependencies** — no database, Redis, or NATS required (the only third-party import is `gopkg.in/yaml.v3`, used solely to parse the optional YAML policy schema). This enables consistent policy evaluation behavior across all deployment contexts:

- **MCP Proxy**: Intercepts JSON-RPC tool calls
- **SDK Instrumentation**: Auto-governs framework executions
- **Fulcrum Server**: Central policy evaluation

## Key Types

### Evaluator

The main policy evaluation engine:

```go
type Evaluator struct {
    // Thread-safe, sorted by priority (highest first)
}

// Create with options
evaluator := policyeval.NewEvaluator(policies,
    policyeval.WithMaxEvaluationTime(10*time.Millisecond),
    policyeval.WithLogger(logger),
    policyeval.WithStopOnDeny(true),
)

// Evaluate policies against a request
decision, err := evaluator.Evaluate(ctx, &policyeval.EvaluationRequest{
    TenantID:   "tenant-123",
    UserID:     "user-456",
    ToolNames:  []string{"file_read"},
    InputText:  "Read /etc/passwd",
})
```

### Decision

The result of policy evaluation:

```go
type Decision struct {
    Action               ActionType          // allow, deny, escalate, warn, require_approval
    MatchedPolicy        *policyeval.Policy    // Policy that produced this decision
    MatchedRules         []*RuleMatch        // Rules that matched
    Actions              []*policyeval.PolicyAction
    Reason               string              // Human-readable explanation
    EvaluationDurationMs int64
    EscalationReason     string              // For ActionEscalate
}
```

### ActionType

Possible evaluation outcomes:

| Action | Description |
|--------|-------------|
| `ActionAllow` | Permit the action to proceed |
| `ActionDeny` | Block the action |
| `ActionEscalate` | Requires phone-home check (e.g., Semantic Judge) |
| `ActionWarn` | Allow but log a warning |
| `ActionRequireApproval` | Requires human approval before proceeding |

### EvaluationRequest

Context for policy evaluation:

```go
type EvaluationRequest struct {
    TenantID    string
    UserID      string
    UserRoles   []string
    WorkflowID  string
    EnvelopeID  string
    Phase       policyeval.ExecutionPhase  // PRE, MID, POST
    ModelID     string
    ToolNames   []string
    InputText   string
    OutputText  string
    Attributes  map[string]string  // Custom key-value pairs
}
```

## Condition Types

The evaluator supports multiple condition types:

| Type | Description |
|------|-------------|
| `FIELD_MATCH` | Exact field comparison (equals, not equals) |
| `REGEX` | Regular expression matching (cached) |
| `RANGE` | Numeric comparisons (>, <, >=, <=) |
| `IN_LIST` | Check if value is in a list |
| `CONTAINS` | String contains check |
| `STARTS_WITH` | String prefix check |
| `ENDS_WITH` | String suffix check |
| `STATISTICAL_SPIKE` | Z-score based anomaly detection |
| `EXTERNAL_CALL` | HTTP webhook (disabled by default) |
| `SEMANTIC` | Requires server escalation for LLM evaluation |
| `LOGICAL` | AND/OR/NOT combinations |

## Usage Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

func main() {
    // Create policies
    policies := []*policyeval.Policy{
        {
            PolicyId: "block-sensitive-files",
            TenantId: "tenant-123",
            Status:   policyeval.PolicyStatus_POLICY_STATUS_ACTIVE,
            Priority: 100,
            Rules: []*policyeval.PolicyRule{
                {
                    RuleId:  "rule-1",
                    Name:    "Block /etc/passwd access",
                    Enabled: true,
                    Conditions: []*policyeval.PolicyCondition{
                        {
                            ConditionType: policyeval.ConditionType_CONDITION_TYPE_CONTAINS,
                            Field:         "input.text",
                            Value:         &policyeval.PolicyCondition_StringValue{StringValue: "/etc/passwd"},
                        },
                    },
                    Actions: []*policyeval.PolicyAction{
                        {ActionType: policyeval.PolicyActionType_ACTION_TYPE_DENY},
                    },
                },
            },
        },
    }

    // Create evaluator
    evaluator := policyeval.NewEvaluator(policies,
        policyeval.WithMaxEvaluationTime(10*time.Millisecond),
        policyeval.WithStopOnDeny(true),
    )

    // Evaluate
    decision, err := evaluator.Evaluate(context.Background(), &policyeval.EvaluationRequest{
        TenantID:  "tenant-123",
        InputText: "Read the contents of /etc/passwd",
    })
    if err != nil {
        log.Fatalf("Evaluation error: %v", err)
    }

    log.Printf("Decision: %s - %s", decision.Action, decision.Reason)
}
```

## Invalid policies: default skip-with-warn vs. strict fail-closed

A policy is only as good as the conditions it can evaluate. If a rule's condition
cannot be evaluated — an **invalid regex pattern**, an **unknown/typo'd field**,
or an **unsupported type** — the engine has to decide what to do with that rule.

**Default (backward-compatible) behavior — skip with a `Warn` log:** the faulting
rule is skipped and evaluation continues with the remaining rules. The skip is
logged at **`Warn`** (it used to be a silent `Debug`). `Evaluate` returns a `nil`
error, so the overall request proceeds. The consequence to internalize:

> A typo in a *deny* rule (e.g. a bad regex or a misspelled field) means that
> rule **never fires**, and the request it was meant to block can be **allowed by
> default**. This is the single most important failure mode to guard against.

Because the default logger discards everything, **you must pass a real logger
via `WithLogger` to see these warnings.** Wire its `Warn` output to a sink you
actually monitor:

```go
evaluator := policyeval.NewEvaluator(policies, policyeval.WithLogger(myLogger))
```

**Strict mode — validate on load and fail closed:** opt in with
`WithStrictPolicies()` (or use `NewEvaluatorStrict` / `UpdatePoliciesStrict`).
The whole policy set is validated **when it is loaded or replaced**, including
**compiling every regex pattern** and checking every field path. An invalid set
is rejected:

- `NewEvaluatorStrict(policies, opts...) (*Evaluator, error)` and
  `(*Evaluator).UpdatePoliciesStrict(policies) error` **return the validation
  error** to you directly.
- The plain `NewEvaluator` / `UpdatePolicies` keep their no-error signatures for
  backward compatibility. With `WithStrictPolicies()` they cannot return the
  error, so instead they **log it at `Warn` and arm a fail-closed state**:
  `Evaluate` then **denies every request** until a valid policy set is loaded.
  (Loading a valid set via `UpdatePolicies`/`UpdatePoliciesStrict` clears the
  state and resumes normal evaluation.)

```go
// Reject a typo'd policy up front instead of discovering it at request time:
evaluator, err := policyeval.NewEvaluatorStrict(policies, policyeval.WithLogger(myLogger))
if err != nil {
    log.Fatalf("policy set rejected: %v", err) // e.g. invalid regex pattern "[unterminated"
}
```

**Validate without committing to strict mode:** call
`(*Evaluator).ValidateAllPolicies() error` at startup (or after a hot reload) to
detect a bad regex / bad field *before* serving traffic, without changing
evaluation behavior:

```go
evaluator := policyeval.NewEvaluator(policies, policyeval.WithLogger(myLogger))
if err := evaluator.ValidateAllPolicies(); err != nil {
    log.Fatalf("policy validation failed: %v", err)
}
```

Recommended embedder guidance: **prefer strict mode (or at minimum
`ValidateAllPolicies` at startup) in any deployment where a silently-skipped deny
rule is unacceptable**, and always supply a `WithLogger` so skip/validation
warnings are visible. The standalone `ValidatePolicy` / `ValidateRule` /
`ValidateCondition` functions perform the same checks (including regex compile)
on individual values if you want to validate before constructing an evaluator.

## Configuration Options

```go
// Set maximum evaluation time (default: 10ms)
policyeval.WithMaxEvaluationTime(10 * time.Millisecond)

// Set logger for debug/warning messages
policyeval.WithLogger(myLogger)

// Enable external HTTP calls for conditions (default: false)
policyeval.WithExternalCallsEnabled(true)

// Stop evaluating after first deny (default: true)
policyeval.WithStopOnDeny(true)

// Validate policies (incl. regex compile) on load and fail closed on any
// invalid policy instead of silently skipping it (default: off)
policyeval.WithStrictPolicies()
```

## Security Features

- **SSRF Protection**: External calls validate URLs against blocked IP ranges and hostnames
- **Regex Cache**: Compiled regex patterns are cached (max 1000) for performance
- **No Secrets in Memory**: Policies don't contain credentials
- **Fail-Closed for Semantic**: Semantic conditions escalate to server (require LLM)

## Performance

- Design target: <10ms P99 evaluation time (unmeasured — no in-tree
  benchmark; `WithMaxEvaluationTime` enforces a timeout cutoff but does
  not itself benchmark evaluation latency)
- Design goal: <1ms for simple policies
- Regex caching reduces repeated pattern compilation
- Policy sorting by priority enables early termination

## Testing

```bash
# Run tests
go test ./policyeval/... -v

# With coverage
go test ./policyeval/... -cover

# Coverage floor enforced in CI per CONTRIBUTING.md (policyeval >=88%);
# re-run locally to print the current value.
```

## Dependencies

- Policy types are declared in-package (`types.go`) — no protobuf runtime required
- No external infrastructure (Redis, NATS, database)
- One third-party import: `gopkg.in/yaml.v3`, used only by `schema.go` to parse the optional v1 YAML policy document. Constructing policies directly in Go (as the examples above do) exercises no third-party code.

## Related Packages

- [`governance`](../governance/) — the 4-stage pipeline that uses this evaluator in its PolicyEval stage
- [`adapters/mcp`](../adapters/mcp/), [`adapters/cli`](../adapters/cli/), [`adapters/codeexec`](../adapters/codeexec/) — transport adapters that feed the pipeline
