# Standards and Incident Mapping

The mapping a security engineer needs to evaluate Boundary against the
checklists already in use: OWASP Top 10 for Agentic Applications, the NSA's
MCP security design considerations, the joint Five Eyes agentic-AI guidance,
EU AI Act logging and human-oversight articles, SOC 2 evidence collection,
and the public 2025 incident classes.

Every row maps to a delivered claim in the build-gated claims ledger, and
every row carries the same ground rule: **Boundary governs an action only
when the route is forced through Boundary** — the unrouted path is a bypass
that deployment topology must remove. The mapping is architecture fit, not a
compliance certification or guarantee.

The short version:

| Framework hook | Boundary answer | Maturity |
|---|---|---|
| OWASP ASI01 (Agent Goal Hijack) | Stage 1 trust check + adaptive termination; write-after-taint denial of the consequential routed action | Trust delivered; taint profile preview, fixture-proof |
| OWASP ASI02 (Tool Misuse) | Static policies + interceptors (including the Postgres AST guard) decide routed tool calls before execution | Production on the routed MCP path |
| OWASP ASI05 (Unexpected Code Execution) | Command Boundary decides routed commands before execution | Delivered preview, routed paths only |
| NSA MCP CSI (mediating proxy, invocation logging) | Production MCP JSON-RPC proxy + a structured, hash-verifiable decision record per governed verdict | MCP production; records delivered |
| Five Eyes "accountability opacity" | Hash-verifiable decision records, offline `verify-record`, `explain`, `replay`, evidence bundles | Delivered |
| EU AI Act Art. 12 / Art. 14 | Decision records; warn / escalate / require-approval verdicts | Architecture fit for the 2027 obligations — not compliance |
| SOC 2 evidence requests | `boundary evidence bundle` + `boundary evidence verify` (manifest-hashed, offline-verifiable) | Delivered, local-only |

The remaining seven OWASP ASI codes are listed as out of scope — mapped to
nothing rather than stretched. Incident classes (production-database
deletion, GitHub MCP write-after-taint exfiltration, MCP data exposure) are
mapped to the pipeline stage that denies the routed shape, each with its own
routed-only caveat; the worked example is the fixture-only
`boundary demo github-lethal-trifecta` denial:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

Canonical mapping:
[docs/STANDARDS_MAPPING.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/STANDARDS_MAPPING.md)

Related references:

- [docs/CLAIMS_LEDGER.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/CLAIMS_LEDGER.md)
  — every cited claim ID, its status, and its evidence paths.
- [docs/ADAPTER_READINESS_MATRIX.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/ADAPTER_READINESS_MATRIX.md)
  — which adapter is production and which are previews.
- [LIMITATIONS.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/LIMITATIONS.md)
  — the routed-only constraint in full.
- [docs/DEMO_GITHUB_LETHAL_TRIFECTA.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/DEMO_GITHUB_LETHAL_TRIFECTA.md)
  — the worked incident demo and what it does not prove.
