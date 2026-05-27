#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

run() {
  printf '+'
  printf ' %q' "$@"
  printf '\n'
  "$@"
}

cd "$ROOT"
run go run ./cmd/boundary demo github-lethal-trifecta
