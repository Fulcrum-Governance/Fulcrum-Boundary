# Release Checklist

Use this checklist before tagging a Boundary release.

## Claims

- Review [`docs/CLAIMS_LEDGER.md`](./CLAIMS_LEDGER.md) and
  [`claims/boundary_claims.yaml`](../claims/boundary_claims.yaml).
- Confirm every release-note claim is marked `delivered`, or is explicitly
  qualified with the gap language from a `partial` claim.
- Run the claims gate:

```bash
env -u GOROOT go test ./claims
```

## Adapter Readiness

- Review [`docs/ADAPTER_READINESS_MATRIX.md`](./ADAPTER_READINESS_MATRIX.md).
- Confirm every adapter under `adapters/` has `readiness.yaml`.
- Confirm `README.md` lists adapters by maturity level.
- Run the adapter conformance gate:

```bash
env -u GOROOT go test ./tests/adapter_conformance
```

## Regression

Run the standard Boundary gates before handoff:

```bash
env -u GOROOT go test ./... -count=1 -timeout 5m
env -u GOROOT go vet ./...
git ls-files '*.go' | xargs gofmt -l
git diff --check
```
