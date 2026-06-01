# Fulcrum Boundary Lexicon

These terms are canonical for Boundary docs and release surfaces. Use them when
they match repo evidence. Do not use them to upgrade a claim beyond
[`docs/CLAIMS_LEDGER.md`](./CLAIMS_LEDGER.md).

## Action Boundary

Definition: The enforced decision point between agent intent and privileged
execution.

Allowed use: "Boundary is the action boundary for routed agent tools."

Forbidden use: "Boundary is an AI governance dashboard."

Example: "Before a tool call reaches GitHub, the action boundary returns a
verdict."

## Agent Intent

Definition: The agent's requested objective before it becomes a concrete tool
operation.

Allowed use: "Boundary sits between agent intent and privileged tools."

Forbidden use: "Agent intent is always safe if the prompt sounds benign."

Example: "The agent intent was to update a README, but the proposed action was a
private-repo write."

## Proposed Action

Definition: The concrete operation a tool or adapter is about to execute.

Allowed use: "Boundary evaluates the proposed action before execution."

Forbidden use: "Boundary judges vague user intent without a tool action."

Example: "`create_pull_request` against a private repo is the proposed action."

## Privileged Tool

Definition: A tool that can touch a real system, private data, external sink, or
durable state.

Allowed use: "GitHub, shell, database, filesystem, and messaging tools are
privileged tools when they can read or mutate real resources."

Forbidden use: "Every helper function is a privileged tool."

Example: "A GitHub MCP server with repo write permission is a privileged tool."

## Governed Route

Definition: A deployment path where the proposed action must pass through
Boundary before reaching the privileged tool.

Allowed use: "The MCP Safety Gateway is a governed route when the agent cannot
reach Postgres directly."

Forbidden use: "A direct tool call is governed because Boundary is installed
somewhere nearby."

Example: "The governed route sends `tools/call` through Boundary before the
upstream MCP server receives it."

## Bypass Path

Definition: Any route that reaches the privileged tool without passing through
Boundary.

Allowed use: "Direct shell access is a bypass path for the CLI adapter unless
the wrapper is the sole command path."

Forbidden use: "Bypass paths do not matter for preview adapters."

Example: "A Postgres connection string exposed to the agent is a bypass path
around the gateway."

## Verdict

Definition: The action returned by Boundary after evaluating a proposed action.
Valid verdicts are `allow`, `deny`, `warn`, `escalate`, and
`require_approval`.

Allowed use: "The verdict was `deny` because the matched rule blocked a
destructive action."

Forbidden use: "Verdict means the action has already executed."

Example: "The GitHub write received a `deny` verdict before the API call."

## Decision Record

Definition: A structured record of a governed verdict and its context.

Allowed use: "Every governed verdict produces a structured decision record."

Forbidden use: "Every decision record is receipt-grade by default."

Example: "The decision record includes the action, reason, matched rule, request
ID, adapter, tenant, and agent context when available."

## Receipt-Grade Record

Definition: A decision record with request, policy bundle, and decision hashes
that can be verified against the recorded inputs.

Allowed use: "Boundary produces receipt-grade decision records with request,
policy bundle, and decision hashes."

Forbidden use: "Receipt-grade means signed by default."

Example: "`boundary verify-record` checks whether the record still matches its
request and policy hashes."

## Source

Definition: The origin of data entering agent context or a tool request.

Allowed use: "A public GitHub issue body from a non-collaborator is an
untrusted source."

Forbidden use: "All GitHub content is one source class."

Example: "The source was `public_issue_body`."

## Sink

Definition: The destination where data or action effects can leave the agent
context or mutate state.

Allowed use: "A private-repo write is a sink for tainted GitHub context."

Forbidden use: "Only network egress can be a sink."

Example: "Slack messages, PR comments, and file pushes are all sinks."

## Mutation

Definition: A proposed action that changes durable state or externally visible
content.

Allowed use: "Creating a branch, updating a file, and merging a PR are
mutations."

Forbidden use: "A read-only issue fetch is a mutation."

Example: "Boundary denied the private-repo mutation after tainted context
entered the session."

## Tainted Context

Definition: Agent context that contains untrusted content capable of influencing
later tool actions.

Allowed use: "Public issue content from a non-collaborator taints the session."

Forbidden use: "Tainted context proves the content is malicious."

Example: "The session had tainted context from a public issue body."

## Risk Path

Definition: A source-to-sink or source-to-mutation path that could cause
exfiltration, destructive change, or unauthorized publication.

Allowed use: "Boundary detected a risk path from public issue body to private
repo write."

Forbidden use: "A risk path is always an exploit."

Example: "`github.issue_body -> private_repo.write` is a risk path."

## Capability

Definition: A class of action a tool can perform, such as repo read, repo write,
shell execution, database write, or external messaging.

Allowed use: "Inventory classifies MCP tools by capability."

Forbidden use: "Capabilities are only labels for UI display."

Example: "`create_pull_request` has a repo-write capability."

## Tool Descriptor

Definition: The schema and metadata a tool exposes to an agent or MCP client.

Allowed use: "Boundary records descriptor hashes so changed tool behavior can be
detected."

Forbidden use: "A tool descriptor proves the tool is safe."

Example: "A changed `create_pull_request` descriptor triggers lock
verification."

## Descriptor Lock

Definition: A stored hash of approved tool descriptors used to detect tool
changes or rug pulls.

Allowed use: "Descriptor lock detects when a tool's advertised shape changes."

Forbidden use: "Descriptor lock replaces policy evaluation."

Example: "`boundary verify-lock` compares the current descriptor hash to the
approved lockfile."

## Approval Edge

Definition: A policy boundary where the verdict changes from automatic allow or
deny to `require_approval`.

Allowed use: "A W1 write can cross an approval edge when the repo is outside the
allowlist."

Forbidden use: "Approval edge means a human approved the action."

Example: "Paranoid mode places external sinks behind approval edges."

## Escalation

Definition: A verdict or routing outcome that sends the proposed action to a
higher-friction path instead of executing immediately.

Allowed use: "Boundary escalates uncertain high-risk actions instead of allowing
them silently."

Forbidden use: "Escalation is the same as denial."

Example: "A missing descriptor can escalate to review depending on policy."

## Protected Adapter

Definition: An adapter whose deployment route and topology force actions
through Boundary before execution.

Allowed use: "The MCP adapter is protected when upstream tool access is isolated
behind the gateway."

Forbidden use: "An adapter package is protected just because it compiles."

Example: "Protected adapters need bypass proof."

## Preview Adapter

Definition: An adapter with useful lifecycle coverage that has not satisfied all
production readiness gates.

Allowed use: "Managed Agents remains a preview adapter until live upstream
conformance evidence exists."

Forbidden use: "Preview means production-ready with a softer name."

Example: "A2A is preview because the protocol is evolving and live conformance
evidence is not present."

## Production Adapter

Definition: An adapter that satisfies the readiness matrix and has evidence for
the claimed lifecycle in a protected deployment.

Allowed use: "MCP is the production adapter when deployed through the Safety
Gateway topology."

Forbidden use: "All adapters are production."

Example: "Production status requires tests, docs, bypass proof, and fail-closed
behavior."

## Secure MCP Server

Definition: A governed MCP server profile that exposes tool actions through
Boundary's action-boundary model.

Allowed use: "Secure GitHub MCP is a preview Secure MCP profile once the
write-after-taint fixture is implemented."

Forbidden use: "Fulcrum has secure versions of every MCP server."

Example: "A Secure MCP server declares tool risk classes, source and sink
classes, taint rules, write rules, bypass model, and decision-record fields."

## MCP Firewall

Definition: The Boundary feature set for discovering MCP clients, inventorying
tool capabilities, finding dangerous MCP tool paths, generating starter
policies, and routing tools through Boundary.

Allowed use: "MCP Firewall is the install wedge for Boundary."

Forbidden use: "MCP Firewall proves every MCP server is safe."

Example: "`boundary inventory` and `boundary graph` are MCP Firewall commands."
