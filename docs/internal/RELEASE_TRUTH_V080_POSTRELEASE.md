# Boundary v0.8.0 Post-Release Verification

Date: 2026-06-02

This records the actual post-tag verification for the `v0.8.0` release: the tag
resolves, both the pinned `@v0.8.0` and the `@latest` install paths build and
report `v0.8.0`, and a GitHub Release object exists. It is the evidence
`docs/RELEASE_TRUTH_PUBLIC.md` references in place of the prior "pending" note.

## Tag

- Tag `v0.8.0` (annotated) -> commit `1b7ef8017f9d427f961f0e6a90bb682235d11238`.
- Tagger: Tony Diefenbach.
- Message: "Boundary v0.8.0 — Phase 0A record-UX lane: DecisionRecordV2
  route-context records, boundary explain, boundary replay, and uniform
  record-location output. No new governed action surface."

## Module resolution

`go list -m -json github.com/fulcrum-governance/fulcrum-boundary@latest`:

```
"Path":    "github.com/fulcrum-governance/fulcrum-boundary"
"Version": "v0.8.0"
"Time":    "2026-06-02T00:05:24Z"
```

`@latest` resolves to `v0.8.0`. The local module-cache `Dir` field is omitted.

## Install verification

Pinned tag install:

```
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.8.0
boundary version
# -> Fulcrum Boundary v0.8.0
```

`@latest` install:

```
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest
boundary version
# -> Fulcrum Boundary v0.8.0
```

Both report `Fulcrum Boundary v0.8.0`. On a module-proxy install the `commit:`
and `build_date:` fields read `unknown`; that is expected Go behavior (those
ldflags are stamped only by `make build` in a VCS checkout). The version string
is derived correctly from the module tag.

## GitHub Release

- <https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.8.0>
- Body: `docs/releases/v0.8.0.md`.

## Scope

This verifies the release surface (tag, both install paths, release object). It
does not change what Boundary governs and adds no production claim. The release
ships the Phase 0A record-UX lane already merged to `main`.
