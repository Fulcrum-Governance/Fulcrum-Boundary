# Secure MCP Contract

Secure MCP is a Boundary pattern for building governed MCP server profiles. A
Secure MCP profile sits between an MCP client and an upstream system, classifies
the proposed tool action, evaluates policy before execution, and emits a
decision record for every governed verdict.

This document is a contract for profile authors and reviewers. It does not mean
every Secure MCP profile exists today, and it does not mark a profile
production-ready. Each profile must carry its own readiness, evidence, tests,
and bypass model.

## Core Rule

Before an MCP tool call touches a real system, Boundary decides whether that
action is allowed.

That decision is only meaningful for calls routed through the Secure MCP
profile. Direct calls to the upstream MCP server or upstream API are bypass
paths unless deployment topology removes or blocks them.

## Participants

| Participant | Responsibility |
|---|---|
| MCP client | Sends JSON-RPC tool calls. |
| Secure MCP profile | Parses the tool call, classifies the action, evaluates policy, and forwards only allowed calls. |
| Boundary pipeline | Produces allow, deny, warn, escalate, or require-approval verdicts. |
| Upstream system | The real API, service, database, repository, or tool server. |
| Operator config | Defines tenant, resource scope, allowlists, policy mode, credentials, and bypass controls. |

## Required Profile Declaration

Every Secure MCP profile must publish a declaration that reviewers can audit.
The declaration can live in code, YAML, or docs, but it must include the same
fields.

```yaml
profile:
  id: secure-github
  status: preview
  upstream: github
  auth_model: byo_github_app
  evidence:
    tests: []
    docs: []
  bypass_model: docs/deployment/secure-github-bypass-proofing.md

descriptor_lock:
  mode: warn
  hash_algorithm: sha256
  canonicalization: canonical-json

tools:
  - name: create_or_update_file
    descriptor_hash: sha256:<hex>
    capability_class: W1
    source_class: none
    sink_class: private_repo_contents
    mutation_class: protected_repo_write
    tenant_scope:
      required: true
      fields: [tenant_id, org]
    resource_scope:
      required: true
      fields: [owner, repo, path, branch]
    taint:
      reads_untrusted_content: false
      blocked_after_taint: true
    policy_projection:
      action: github.create_or_update_file
      resource: github.repo_file
      risk_class: W1
```

## Descriptor Hashes

Secure MCP profiles must treat tool descriptors as part of the security-relevant
surface. A descriptor is the tool name, description, input schema, output shape
where available, and declared upstream target.

Profiles that claim descriptor locking must:

1. Canonicalize descriptor data in a stable format.
2. Hash it with a documented algorithm.
3. Store the hash in a lockfile or profile declaration.
4. Compare the current descriptor hash before serving or forwarding.
5. Apply the configured policy when the descriptor changes.

Allowed descriptor-change policies are:

| Mode | Behavior |
|---|---|
| `warn` | Record the mismatch and continue only when the profile is configured to allow warning mode. |
| `require_approval` | Stop serving or forwarding until an operator accepts the new descriptor. |
| `deny` | Fail closed until the lockfile is updated through an explicit command. |

Descriptor locking detects tool-surface drift. It does not prove the upstream
tool implementation is safe.

## Capability Classification

Every tool must have one capability class. The class is a policy input, not a
claim that the tool is safe.

| Class | Meaning | Example action |
|---|---|---|
| `R0` | Read or observe. May taint the envelope when content is untrusted. | Read an issue, file, row, message, or tool descriptor. |
| `R1` | Low-risk write. Usually reversible or local to a discussion surface. | Add a comment, mark a notification read. |
| `W0` | Medium write. Creates or updates user-visible state. | Create an issue or pull request. |
| `W1` | Protected mutation. Changes code, files, credentials, execution, or private data. | Push files or update a private repository file. |
| `W2` | Critical mutation. Irreversible, admin-grade, deployment-grade, merge-grade, or broad-scope action. | Merge a pull request, create a repository, rotate secrets, deploy. |

Profiles may define narrower local classes, but they must project to the shared
classes above before policy evaluation.

## Source, Sink, And Mutation Classification

Secure MCP profiles must name where information came from and where an allowed
action would send or mutate it.

Source classes should distinguish at least:

- trusted operator input
- allowlisted organization resource
- public resource
- external collaborator content
- generated agent content
- local file or secret source
- prior tool output

Sink classes should distinguish at least:

- private repository
- public repository
- external publication surface
- local filesystem
- database
- messaging system
- deployment target
- secret store

Mutation classes should distinguish at least:

- none
- comment or discussion write
- public issue or pull request creation
- private repository content write
- destructive delete
- merge or release action
- credential or secret mutation
- deployment or runtime mutation

The first Secure GitHub proof path is write-after-taint: untrusted GitHub
content enters the envelope, and a later private-repo mutation is denied before
GitHub is touched.

## Tenant And Resource Scope

Every governed request must carry enough scope for a policy to decide who owns
the action and which resource is at risk.

Minimum fields:

- `tenant_id`
- `profile_id`
- `tool`
- `capability_class`
- `action`
- `resource_type`
- `resource_owner`
- `resource_id`
- `request_id`
- `envelope_id`

Profile-specific resource fields should remain explicit. For GitHub, that means
owner, repo, branch, path, issue number, pull request number, or commit SHA as
appropriate.

## Taint Hooks

Taint is an envelope-level signal that untrusted content entered the agent
context through a governed route. A profile that reads untrusted content must
record a taint source. A profile that writes to a protected sink must check
whether the envelope is tainted before forwarding.

Required taint hooks:

| Hook | Required behavior |
|---|---|
| `on_read` | Mark the envelope when content comes from a non-allowlisted or external source. |
| `on_write` | Deny or require approval when a protected mutation follows taint, according to profile policy. |
| `on_record` | Include taint source, target sink, capability class, and verdict in the decision record. |

For the initial Secure GitHub profile, W1 and W2 private-repo mutations after
taint must deny in fixture proof mode.

## Policy Projection

Profiles must project protocol-specific tool calls into Boundary policy inputs.

At minimum:

```text
MCP tool call
  -> profile parser
  -> GovernanceRequest
  -> Boundary pipeline
  -> verdict
  -> forward only if allowed
  -> decision record
```

Projection must preserve:

- original MCP method and tool name
- normalized action
- tenant and actor identity when available
- resource scope
- capability class
- source, sink, and mutation classes
- taint state and taint source references
- descriptor hash or descriptor lock status when enabled

Unknown mandatory fields must fail closed. Unknown optional fields may pass
through only if they are recorded and do not affect the policy decision.

## Deny Before Execution

A Secure MCP profile must never call the upstream system for a denied request.
This is the central contract.

Required behavior:

1. Parse and classify the proposed action.
2. Evaluate policy before upstream execution.
3. Return an MCP-shaped denial when the verdict is deny.
4. Do not call the upstream API or upstream MCP server for denied requests.
5. Record the denial with request, envelope, profile, tool, capability, and
   reason.

Pipeline errors fail closed. Parse errors return a protocol-shaped error and do
not reach upstream execution.

## Decision Records

Each governed Secure MCP interaction must produce a structured decision record.
The record should include:

- `request_id`
- `envelope_id`
- `tenant_id`
- `profile_id`
- `tool`
- `action`
- `capability_class`
- `source_class`
- `sink_class`
- `mutation_class`
- `resource_scope`
- `taint_state`
- `taint_sources`
- `descriptor_hash`
- `descriptor_lock_status`
- `verdict`
- `rule`
- `reason`
- `timestamp`

Receipt-grade records are a separate capability and must follow
[`docs/RECEIPTS.md`](./RECEIPTS.md).

## Bypass Model

Secure MCP protects only governed routes. Each profile must document the bypass
paths that remain and the deployment controls that close them.

Required bypass questions:

1. Can the agent call the upstream MCP server directly?
2. Can the agent call the upstream API directly with the same credential?
3. Can another MCP server perform the same mutation?
4. Can the operator config be changed by the agent?
5. Can tool descriptors change after policy is written?
6. Can a session mix untrusted reads from one resource with writes to another?

Production language is not allowed until bypass evidence exists for the
deployment topology being described.

## Maturity

Secure MCP is a pattern. Individual profiles own their maturity.

| Maturity | Meaning |
|---|---|
| `planned` | Contract or roadmap only. |
| `preview` | Core fixture or local lifecycle works, but live conformance or bypass evidence is incomplete. |
| `production` | Full lifecycle, live conformance where needed, descriptor behavior, bypass proof, fail-mode tests, and claims ledger evidence exist. |

Secure GitHub MCP remains preview until live GitHub App conformance evidence and
deployment bypass proof exist.
