# Install

Fulcrum Boundary ships the `boundary` CLI from the Go module
`github.com/fulcrum-governance/fulcrum-boundary`. The binary name remains
`boundary`.

## Install Channels

| Channel | Command | SQL classification |
|---|---|---|
| Homebrew (macOS, Linux) | `brew install fulcrum-governance/tap/boundary` | Reduced — static build (see below) |
| Release archives — static | download `*_static-nocgo` + verify `SHA256SUMS` | Reduced — static build |
| Release archives — cgo | download `*_cgo` + verify `SHA256SUMS-cgo` | Full Postgres AST classifier |
| Container image | `docker run --rm ghcr.io/fulcrum-governance/boundary:<tag>` | Reduced — static build |
| Build from source | `go install …/cmd/boundary@<tag>` (Go 1.25+, C toolchain) | Full Postgres AST classifier |

Channel availability: prebuilt binaries, the tap formula, the container image,
and both checksum manifests publish from the tag-gated release workflow
(`.github/workflows/release.yml`) for releases after `v0.9.0`. Releases up to
and including `v0.9.0` shipped source-only; for those, use
[Build From Source](#build-from-source-full-classifier).

## Homebrew

```bash
brew install fulcrum-governance/tap/boundary
boundary selftest
```

The formula installs the static (`CGO_ENABLED=0`) build — see
[Static vs cgo](#static-vs-cgo-builds) for the one capability difference. For
the full SQL classifier, download a `_cgo` release archive or build from
source.

## Release Archives

Each release attaches two archive families plus checksum manifests:

- `boundary_<version>_<os>_<arch>_static-nocgo.tar.gz` (`.zip` on Windows) —
  static builds for darwin, linux, and windows on amd64 and arm64, listed in
  `SHA256SUMS`.
- `boundary_<version>_<os>_<arch>_cgo.tar.gz` — native-cgo builds with the
  full Postgres AST classifier for darwin and linux on amd64 and arm64,
  listed in `SHA256SUMS-cgo`. Windows `_cgo` archives are not produced; use
  the static archive or build from source there.

Download, verify, and install (example: static build on Apple Silicon —
substitute the version, OS, arch, and variant you need):

```bash
VERSION=<version>   # e.g. the latest tag without the leading v
BASE=https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/download
curl -fsSLO "$BASE/v$VERSION/boundary_${VERSION}_darwin_arm64_static-nocgo.tar.gz"
curl -fsSLO "$BASE/v$VERSION/SHA256SUMS"
shasum -a 256 -c SHA256SUMS --ignore-missing
tar -xzf "boundary_${VERSION}_darwin_arm64_static-nocgo.tar.gz"
install -m 0755 boundary ~/.local/bin/boundary   # or any directory on PATH
boundary selftest
```

For a `_cgo` archive, verify against `SHA256SUMS-cgo` instead. On Linux,
`sha256sum -c` replaces `shasum -a 256 -c`.

## Container Image

```bash
docker run --rm ghcr.io/fulcrum-governance/boundary:latest selftest
docker run --rm ghcr.io/fulcrum-governance/boundary:v<version> doctor --json
```

The image carries the static build (reduced SQL classification, below) on a
distroless base; the entrypoint is `boundary`.

## Static Vs Cgo Builds

The Postgres AST guard ([`docs/policies/POSTGRES.md`](policies/POSTGRES.md))
classifies routed SQL with `pg_query_go`, a cgo binding for the PostgreSQL
parser. That produces exactly one capability difference between the two
binary families:

- **Cgo builds** (`_cgo` archives, or source builds with a C toolchain):
  statements classify as `READ` / `WRITE` / `ADMIN` / `DESTRUCTIVE` /
  `UNKNOWN`, and policy acts on those classes.
- **Static builds** (`_static-nocgo` archives, the Homebrew formula, the
  container image, or `CGO_ENABLED=0` source builds): the AST parser is
  unavailable. Every routed SQL statement classifies as `UNKNOWN` with the
  reason `sql ast classification unavailable in this build (CGO disabled)`,
  and the Postgres guard denies it fail-closed.

The reduction is classification, not posture: the static build can deny SQL
the cgo build would allow (including reads), and it never allows SQL the cgo
build would deny. A `//go:build !cgo` test sweep
(`interceptors/sql/ast_classifier_nocgo_test.go`) asserts every case in the
SQL evasion corpus lands in the `UNKNOWN` deny bucket in static builds. If a
deployment depends on SQL class distinctions (for example, allowing `READ`
while escalating `ADMIN`), use a cgo build on that route.

Everything else — the four-stage pipeline, decision records, demos,
`selftest`, `doctor`, `evidence`, policy testing — behaves identically in
both variants. Archive names carry the variant; with a Go toolchain you can
also confirm a binary's variant via `go version -m <path-to-boundary>` and
the `CGO_ENABLED` build setting.

## Build From Source (Full Classifier)

Requires Go 1.25+ and, for the default cgo build, a C toolchain (a C compiler
such as gcc/clang on `PATH`):

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.9.0
boundary selftest
```

`@v0.9.0` is the recommended repeatable install target for the current launch
release. `@latest` resolves to the latest published release after the Go proxy
refreshes.

From a checkout:

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary
go run ./cmd/boundary selftest
```

Without a C toolchain, the static variant builds from the same source:

```bash
CGO_ENABLED=0 go build -o boundary ./cmd/boundary
```

Source checkouts also include Make targets for the no-credential release path:

```bash
make selftest
make demo-github
make release-check
```

## First Useful Commands

Run the local release smoke test:

```bash
boundary selftest
```

Run the fixture-only GitHub lethal-trifecta demo:

```bash
boundary demo github-lethal-trifecta
```

The demo uses fixture data and does not require live GitHub credentials or make
upstream GitHub mutations.

Run the policy-as-code fixture corpus:

```bash
boundary test --path tests/fixtures/policy-test/cases
```

`boundary test` evaluates local policy bundles against routed request fixtures
and exits non-zero on unexpected verdicts. See
[`docs/POLICY_TESTING.md`](POLICY_TESTING.md).

## Uninstall

Remove a Homebrew install:

```bash
brew uninstall fulcrum-governance/tap/boundary
```

Remove a binary installed from a release archive by deleting it from wherever
you placed it (for example `~/.local/bin/boundary`). Remove the binary
installed by Go:

```bash
rm "$(go env GOPATH)/bin/boundary"
```

If you used `boundary install` to rewrite an MCP client config, restore through
the install receipt created at install time:

```bash
boundary uninstall --receipt .boundary/firewall/install-receipts/<receipt>.json
```

Use `--dry-run` first to inspect the planned restore without mutating local MCP
client config files.
