# Secure MCP Server Template

Use this template when adding a new Secure MCP profile. The goal is to make each
profile reviewable before implementation and claim-safe after implementation.

Do not remove sections by saying they are "obvious." If a section does not
apply, write `not_applicable` and explain why.

## Profile Header

```markdown
# Secure <System> MCP Profile

Status: planned | preview | production
Owner: <team or maintainer>
Upstream system: <system name>
Upstream protocol: MCP | API | hybrid
Auth model: <operator-owned credential model>
Boundary command surface: boundary secure <system> ...
Readiness target: planned | preview | production
Claims ledger entries: <claim ids or none yet>
Bypass model: <doc path>
```

## Required Sections

Each profile doc must include these sections in this order:

1. Purpose
2. Status And Claim Boundary
3. Threat Model
4. Supported Tool Set
5. Tool Taxonomy
6. Descriptor Locking
7. Tenant And Resource Scope
8. Taint Model
9. Policy Projection
10. Denial Shape
11. Decision Records
12. Bypass Model
13. Configuration
14. Tests And Evidence
15. Production Gate

## Status And Claim Boundary

Use this exact pattern:

```markdown
This profile is <status>. It governs only calls routed through Boundary.
Direct calls to <upstream> are bypass paths unless the deployment topology
removes them. This profile does not claim production status until the evidence
listed in the production gate exists.
```

Allowed examples:

```text
Secure GitHub MCP is preview. Fixture tests show Boundary denies configured
write-after-taint paths before GitHub is touched.
```

```text
Secure Filesystem MCP is planned. This document defines the contract; it does
not claim runtime coverage.
```

## Tool Declaration Template

Every supported tool needs an entry like this:

```yaml
name: create_or_update_file
upstream_name: create_or_update_file
status: preview
descriptor_hash: sha256:<hex-or-empty-until-locked>
capability_class: W1
source_class: none
sink_class: private_repo_contents
mutation_class: protected_repo_write
tenant_scope:
  fields: [tenant_id, org]
resource_scope:
  fields: [owner, repo, branch, path]
taint:
  reads_untrusted_content: false
  blocked_after_taint: true
policy_projection:
  action: github.create_or_update_file
  resource_type: github.repo_file
  deny_reasons:
    - lethal_trifecta_detected
    - repo_not_allowlisted
    - descriptor_changed
decision_record:
  required_fields:
    - request_id
    - envelope_id
    - tenant_id
    - profile_id
    - tool
    - action
    - capability_class
    - resource_scope
    - verdict
```

## Implementation Checklist

Before opening an implementation PR, confirm:

- The profile declaration exists.
- The supported tool set is explicit.
- Unsupported tools fail closed or return a clear unsupported error.
- Every supported tool has a capability class.
- Source, sink, and mutation classes are defined.
- Tenant and resource scope fields are explicit.
- Taint hooks are documented.
- Descriptor-lock behavior is documented.
- Denied calls cannot reach the upstream system in tests.
- Decision records are emitted for allow and deny verdicts.
- The bypass model names direct-upstream paths.
- Claims updates are limited to behavior with tests.
- README copy, changelog, and claims ledger agree.

## Minimal Command Surface

Profiles should prefer the Boundary CLI namespace:

```text
boundary secure <system> setup
boundary secure <system> serve
boundary secure <system> inspect
boundary secure <system> redteam
```

The command surface may be smaller for planned or preview profiles. Do not add a
new binary unless the Boundary CLI cannot carry the workflow cleanly.

## Configuration Template

```yaml
profile:
  id: secure-<system>
  status: preview

auth:
  model: byo
  credential_ref: ${BOUNDARY_<SYSTEM>_CREDENTIAL}

scope:
  tenant_id: default
  allowlisted_resources: []
  protected_sinks: []

descriptor_lock:
  path: .boundary/secure-<system>.lock.json
  mode: warn

taint:
  enabled: true
  deny_protected_write_after_taint: true

records:
  local_path: .boundary/records
```

Do not put secrets directly in example config.

## Evidence Template

```markdown
## Tests And Evidence

| Evidence | Status | Path |
|---|---|---|
| Fixture denial before upstream call | missing | tests/secure<system>/... |
| Allow path forwards once | missing | tests/secure<system>/... |
| Descriptor change behavior | missing | tests/secure<system>/... |
| Decision record fields | missing | tests/secure<system>/... |
| Bypass proof | missing | docs/deployment/... |
| Live upstream conformance | missing | docs/secure-mcp/... |
```

## Production Gate

A Secure MCP profile can be called production only when all items are true:

1. Supported tools have descriptor hashes or a documented reason descriptor
   locking is not applicable.
2. Parse, identify, evaluate, deny, forward, inspect, metadata, record,
   bypass-proof, and fail-closed lifecycle steps are implemented or formally
   delegated with tests.
3. Denied calls are proven not to reach upstream execution.
4. The bypass model is tested for the deployment posture being claimed.
5. Live upstream conformance exists when the upstream requires credentials,
   service behavior, or protocol compatibility.
6. README, changelog, claims ledger, and readiness matrix all match.

Until then, use preview or planned language.
