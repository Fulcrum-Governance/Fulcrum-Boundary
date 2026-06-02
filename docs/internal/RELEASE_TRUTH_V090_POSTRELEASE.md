# Boundary v0.9.0 Post-Release Verification

Date: 2026-06-02

This records the actual post-tag verification for the `v0.9.0` release: the tag
resolves, both the pinned `@v0.9.0` and the `@latest` install paths build and
report `v0.9.0`, `boundary test` is present in the installed command surface,
and a GitHub Release object exists. It is the evidence
`docs/RELEASE_TRUTH_PUBLIC.md` references in place of the prior "pending" note.

## Tag

- Tag `v0.9.0` (annotated) -> commit
  `5cf0aacb650c128298f7ede0de0e18a72919d346`.
- Tag object:
  `b7ff3afc35a96fa760b09d83f9027eb378adcb79`.
- Tagger: Anthony Joseph Diefenbach.
- Message: "Fulcrum Boundary v0.9.0".

## Module resolution

`go list -m -json github.com/fulcrum-governance/fulcrum-boundary@latest`:

```json
{
  "Path": "github.com/fulcrum-governance/fulcrum-boundary",
  "Version": "v0.9.0",
  "Query": "latest",
  "Time": "2026-06-02T05:34:55Z",
  "GoVersion": "1.25.0"
}
```

`@latest` resolves to `v0.9.0`. Local module-cache path fields are omitted.

`go list -m -json github.com/fulcrum-governance/fulcrum-boundary@v0.9.0`:

```json
{
  "Path": "github.com/fulcrum-governance/fulcrum-boundary",
  "Version": "v0.9.0",
  "Time": "2026-06-02T05:34:55Z",
  "GoVersion": "1.25.0",
  "Origin": {
    "VCS": "git",
    "URL": "https://github.com/fulcrum-governance/fulcrum-boundary",
    "Hash": "5cf0aacb650c128298f7ede0de0e18a72919d346",
    "Ref": "refs/tags/v0.9.0"
  }
}
```

## Install verification

Pinned tag install:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.9.0
boundary version
# -> Fulcrum Boundary v0.9.0
```

`@latest` install:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest
boundary version
# -> Fulcrum Boundary v0.9.0
```

Both report `Fulcrum Boundary v0.9.0`. On a module-proxy install the `commit:`
and `build_date:` fields read `unknown`; that is expected Go behavior (those
ldflags are stamped only by `make build` in a VCS checkout). The version string
is derived correctly from the module tag.

The installed binaries also expose the Phase 1 policy-as-code command:

```bash
boundary test --help
# -> Run local policy-as-code test cases against Boundary policy bundles.
```

## GitHub Release

- <https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.9.0>
- Published at: `2026-06-02T05:36:03Z`.
- Draft: `false`.
- Prerelease: `false`.
- Target commit:
  `5cf0aacb650c128298f7ede0de0e18a72919d346`.
- Body: `docs/releases/v0.9.0.md`.

## Scope

This verifies the release surface (tag, both install paths, release object, and
installed `boundary test` availability). It does not change what Boundary
governs and adds no production claim. The release ships the Phase 0A record-UX
lane from `v0.8.0` plus the Phase 1 local policy-as-code testing lane.
