# FAQ

Canonical repository reference:
[docs/FAQ.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/FAQ.md)

A skeptic's FAQ for Fulcrum Boundary. Every answer in the canonical version is
bound to a doc in the repository and is written to its limitations, not past
them. Highlights:

- **Routed-only.** Boundary governs an action only when the route is forced
  through it. Direct shell, editor, CI, SSH, or API access to the same tool is a
  bypass unless deployment topology removes that path; closing it is a
  deployment-topology job, confirmed with the route-conformance checklist.
- **Fixture-only demos.** The two proof lanes run against synthetic fixtures with
  no credentials, no network calls, and no live mutation. They demonstrate the
  deny-before-execution decision; they do not prove every deployment route is
  protected.
- **Hash-verifiable = integrity, not authenticity.** Decision-record hashes are
  unkeyed SHA-256 over canonical bytes. Recomputation with `boundary
  verify-record` detects post-emission tampering; it does not make a record
  proved, tamper-proof, immutable, or attested, and `upstream_called=false` /
  `executed=false` are adapter self-reports.
- **Record-scoped RFC 8785 / JCS.** That conformance statement is scoped to the
  decision record's canonical bytes;
  it is not a claim that Boundary as a whole is standards-conformant.
- **MCP is the only production route.** All other adapters and profiles ship as
  labeled previews; production status requires passing the adapter-readiness gate.
- **Static and cgo builds differ only in SQL classification depth.** Cgo builds
  use the full Postgres AST classifier. Static builds (`CGO_ENABLED=0`,
  Homebrew, container images, and `_static-nocgo` archives) classify routed SQL
  as `UNKNOWN` and deny it fail-closed.
- **No model in the local verdict path.** The verdict path is deterministic;
  semantic rules escalate rather than guess, and Boundary does not emit `proved`.
- **Standalone vs kernel.** Standalone is the zero-dependency OSS path; kernel is
  the commercial Fulcrum integration and fails hard on incomplete config.
- **Replay reproduces the decision, not enforcement.** A reproduced `deny` is not
  evidence the action was blocked, and a match does not prove the verdict was
  correct.
- **The claims gate is an exit code.** Clone the repo and run `go test
  ./claims/... -count=1` and `make release-check` to check the docs against the
  tests yourself.

Related references:
[How We Keep Ourselves Honest](reference/how-we-keep-ourselves-honest.md) ·
[Claims](reference/claims.md) ·
[Route Conformance](reference/route-conformance.md) ·
[Troubleshooting](reference/troubleshooting.md)
