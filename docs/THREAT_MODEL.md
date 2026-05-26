# Threat Model

This document describes the trust boundaries and attack surface for the MCP Safety Gateway topology in Fulcrum Boundary v0.2.0.

## Trust Boundary

The boundary is the sole route between the agent and the privileged tool. All governance decisions happen at this boundary before execution.

In the Docker demo, the agent can reach the gateway but cannot reach Postgres directly. That topology turns Boundary into the pre-execution decision point for the demo tool.

## Attack Surface 1: Bypass

**Threat:** The agent attempts to reach the tool directly, bypassing Boundary.

**Demo mitigation:** Docker network topology. The `demo-agent` container is on the frontend network only. Postgres is on the backend network only. The demo explicitly proves that the agent container cannot reach Postgres on the backend network.

**Production mitigation:** Infrastructure controls must enforce the same sole-route constraint. Use firewall rules, service mesh policy, Kubernetes network policies, private networking, or equivalent controls so the governed route is the only route.

## Attack Surface 2: Policy Circumvention

**Threat:** The agent crafts inputs that evade policy rules, such as obfuscated SQL.

**Mitigation:** Security-critical transports fail closed on policy evaluation errors. Unknown or unparseable requests are denied for those transports instead of being allowed through by default.

**Release scope:** The launch policy uses string matching for destructive-action blocking. Semantic SQL analysis and advanced obfuscation resistance are future enhancements.

## Attack Surface 3: Audit Tampering

**Threat:** The agent or a compromised process modifies decision records after Boundary emits them.

**Mitigation:** Production deployments should forward Boundary logs to an external system the agent cannot reach, preferably append-only storage or an equivalent immutable log sink.

**Release scope:** Boundary emits structured decision records to stdout. The Docker demo makes the records inspectable; production log integrity depends on the deployment's log pipeline.

## What The Demo Proves

The MCP Safety Gateway demo proves:

- safe queries pass through Boundary
- destructive demo queries are blocked by policy
- direct bypass fails by network design
- decision records are emitted for every verdict

## What The Demo Does Not Prove

The MCP Safety Gateway demo does not prove:

- resistance to sophisticated SQL obfuscation
- multi-agent coordination governance
- trust-based adaptive termination
- receipt verification

Those capabilities are outside v0.2.0. They remain roadmap items in the adapter ladder described by the launch plan.
