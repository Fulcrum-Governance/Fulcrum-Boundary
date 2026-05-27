#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

term_a="bumble"
term_b="bee"
joined_term="${term_a}${term_b}"
spaced_term="${term_a} ${term_b}"

if git grep -n -i "$joined_term"; then
  echo "Forbidden public vendor reference found" >&2
  exit 1
fi

if git grep -n -i "$spaced_term"; then
  echo "Forbidden public vendor reference found" >&2
  exit 1
fi
