# Boundary Conditions

Fulcrum Boundary protects a tool only when the agent's route to that tool passes through the boundary. The boundary is the decision point; deployment topology decides whether the agent can route around it.

## Protected

Fulcrum Boundary protects the tool when the agent's only route to the tool passes through Boundary.

The MCP Safety Gateway demo enforces this with Docker network isolation:

- `demo-agent` is attached only to the frontend network.
- `gateway` is attached to the frontend and backend networks.
- `postgres` is attached only to the backend network.
- The backend network is internal, so the demo agent cannot reach Postgres directly.

In that topology, the agent must send tool requests through Boundary before Postgres can be touched.

## Not Protected

Fulcrum Boundary does not protect a tool when the agent has a direct network path to that tool.

Examples:

- The agent can connect to Postgres directly.
- The agent can call a tool through a transport Boundary does not intercept.
- The agent invokes a mechanism outside the configured adapter, such as a raw TCP connection instead of an MCP JSON-RPC request.

Adapters change how actions enter Boundary. They do not remove the deployment requirement that privileged tools must be reachable only through the governed route.

## Fail-Closed Behavior

For security-critical transports, policy evaluation errors deny the request. This fail-closed behavior is deliberate for:

- MCP
- CodeExec
- gRPC

Fail-open behavior would defeat the purpose of an action boundary: a transient evaluator error would become an ungoverned tool path.

## Production Deployment

The Docker demo proves the sole-route constraint with Docker network isolation. Production deployments must enforce the same constraint with their own infrastructure controls, such as:

- firewall rules
- service mesh policy
- Kubernetes network policies
- private subnets and explicit egress controls

Boundary makes the verdict. Infrastructure must make the bypass path unavailable.

## Policy Scope

The launch policy is demo-grade destructive-action blocking via string matching. It is not a general SQL firewall.

Unless explicitly tested in this release, the launch policy does not claim coverage for:

- case sensitivity edge cases
- whitespace variations
- SQL comments
- dialect-specific syntax
- semantic SQL analysis
- sophisticated obfuscation

The demo proves the release spine: safe queries pass, destructive demo queries are blocked, bypass fails by network design, and every verdict emits an inspectable decision record.
