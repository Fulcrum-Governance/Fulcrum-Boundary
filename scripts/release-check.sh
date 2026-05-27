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
run go test ./... -count=1 -timeout 5m
run_in adapters/grpc go test ./... -count=1 -timeout 5m
run go test ./tests/... -count=1 -timeout 5m
run go test ./claims/... -count=1 -timeout 5m
run go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies
run go run ./cmd/boundary verify-record --help
run go run ./cmd/boundary selftest
run go run ./cmd/boundary demo github-lethal-trifecta
