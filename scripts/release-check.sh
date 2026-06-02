#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

run() {
  printf '+'
  printf ' %q' "$@"
  printf '\n'
  "$@"
}

run_in() {
  local dir="$1"
  shift
  printf '+ (cd %q &&' "$dir"
  printf ' %q' "$@"
  printf ')\n'
  (
    cd "$ROOT/$dir"
    "$@"
  )
}

cd "$ROOT"
tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

run ./scripts/assert-no-public-vendor-refs.sh
run ./scripts/assert-no-internal-public-artifacts.sh
run go vet ./...
run_in adapters/grpc go vet ./...
run go test ./... -count=1 -timeout 5m
run_in adapters/grpc go test ./... -count=1 -timeout 5m
run go test ./tests/... -count=1 -timeout 5m
run go test ./claims/... -count=1 -timeout 5m
run go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies
run go run ./cmd/boundary verify-record --help
run go run ./cmd/boundary test --path tests/fixtures/policy-test/cases
run go run ./cmd/boundary version
run go run ./cmd/boundary selftest
run go run ./cmd/boundary demo github-lethal-trifecta
run go run ./cmd/boundary demo action-boundary
run go run ./cmd/boundary doctor --json
run go run ./cmd/boundary evidence bundle --include-demo --out "$tmp/evidence"
run go run ./cmd/boundary evidence verify "$tmp/evidence"
