# Where Boundary Fits

Your agent is about to touch a real system, and several kinds of tooling
stand near that moment. Scanners decide whether a server was trustworthy to
wire in; gateways route and observe traffic; guardrail libraries classify
content in-process; authorization engines answer policy questions when the
integrator's code asks. Boundary's slot is the **action boundary**: a
per-call `allow / deny / warn / escalate / require-approval` verdict
**before execution** on the routed path, a hash-verifiable decision record
of that verdict, a policy-as-code test lane (`boundary test`), and decision
replay — local, single binary, no control plane. Boundary governs only
routes forced through it; direct access to the same tool is a bypass unless
deployment topology removes that path.

| Shape | The question it answers | Typically not in the shape |
|---|---|---|
| Scanners (static analysis of MCP servers and configs) | "Is this server trustworthy enough to wire in?" | A verdict on the call about to run |
| Gateways / proxies | "Should this traffic pass, and what passed?" | An operator-verifiable per-decision record, a policy test lane, decision replay |
| Guardrail / content libraries | "Does this content look unsafe?" | A deterministic verdict on an action; evidence outside the agent process |
| Authorization engines | "Given these attributes, is this permitted?" | The enforcement itself; per-decision records of agent tool calls |
| Action boundary (Boundary) | "Should this specific call run right now — and can the verdict be checked afterward?" | Anything off the routed path |

Comparisons are framed by category, never by vendor — Boundary's public
surfaces name no third-party tools, and categories describe common shapes,
not any specific tool. MCP is the production route; Command Boundary and
Edit Boundary are delivered previews; the remaining adapters ship as labeled
previews. Every delivered claim links a test that runs in CI — clone the
repo and run `make release-check` yourself.

Canonical comparison page:
[docs/COMPARISON.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/COMPARISON.md)

Related references:

- [docs/CLAIMS_LEDGER.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/CLAIMS_LEDGER.md)
  — every public claim, its status, and its evidence paths.
- [LIMITATIONS.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/LIMITATIONS.md)
  — the routed-only constraint in full.
- [docs/GOVERN_MCP_SERVER.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/GOVERN_MCP_SERVER.md)
  — put Boundary in front of an MCP client and trigger a denial.
