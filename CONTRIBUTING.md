# Contributing to Fulcrum Boundary

Thank you for considering a contribution. Boundary is the action boundary for
routed AI-agent tool paths, and scope discipline matters: changes should make
that boundary clearer, safer, or easier to verify.

Project docs: [Security](./SECURITY.md) | [Code of Conduct](./CODE_OF_CONDUCT.md) | [Changelog](./CHANGELOG.md) | [Citation](./CITATION.cff)

## Maintainer Expectations

Security issues get priority. Response times on other issues may vary. If a
pull request sits idle for more than a few weeks, a polite bump is fine.

## Filing Issues

Use the templates when they fit. They collect the information needed to
diagnose a report.

- Bug reports: Go version, OS, steps to reproduce, expected result, actual
  result, and a minimal reproducer when possible.
- Feature requests: the use case, the proposed behavior, and alternatives
  considered.
- Questions: open an issue and use the `question` label when available.

## Submitting Pull Requests

1. Fork the repository.
2. Create a branch from `main` named for the change.
3. Keep commits focused and scoped to one lane.
4. Run the local checks below before requesting review.
5. Open a pull request against `main`. The pull request template
   (`.github/pull_request_template.md`) lists the verification and claim-safety
   gates to confirm. See [docs/TESTING.md](./docs/TESTING.md) for the test
   architecture and the coverage-attribution note.

For larger changes, including new adapters, new extension points, release
claim changes, or public positioning changes, open an issue first to discuss
the shape before writing code. The bar an adapter must meet to carry the
`production` label — lifecycle steps, bypass-proof delegation, fail-closed
transports, test evidence paths, and the process — is documented in
[`docs/ADAPTER_PRODUCTION_BAR.md`](./docs/ADAPTER_PRODUCTION_BAR.md).

## Local Checks

Run these gates for public-surface or release-truth work:

```bash
make release-check
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
make docs-build
```

Run formatting and static checks for Go changes:

```bash
git ls-files '*.go' | xargs gofmt -l
go vet ./...
```

Expected results:

- `gofmt -l` prints no file paths.
- `go vet ./...` exits cleanly.
- Claim tests and Go tests pass.
- Docs build succeeds in strict mode.
- `make release-check` completes without public-surface or release gate
  failures.

## Code Style

- Use `gofmt` for Go.
- Prefer small, focused functions.
- Keep tests next to the code they test when that matches the existing package
  pattern.
- Public identifiers need doc comments that state the contract callers can rely
  on.
- Preserve routed-only and fixture-only caveats in public language.

## Scope

Boundary work belongs here when it changes:

- Evaluation pipeline stages and their contracts.
- Transport adapter packages and routed preview surfaces.
- Portable policy evaluation primitives.
- Evidence, selftest, release, and local diagnostic utilities.
- Public docs that describe Boundary's released behavior.

The following do not belong in this repository as direct product claims:

- Hosted monitoring or team control-plane promises.
- Complete production policy generation.
- Global control over commands or file writes outside routed Boundary paths.
- Universal protection for tools that bypass Boundary.
- Runtime proof claims beyond the documented proof boundary and empirical
  validation receipts.

## Security Issues

Do not open a public issue for security vulnerabilities. Follow the process in
[SECURITY.md](./SECURITY.md) and email `agent@fulcrumlayer.io`.

## Licensing

By submitting a pull request, you agree that your contribution is licensed
under the Apache License 2.0, the same license as the repository. No CLA is
required.
