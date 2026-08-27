#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

status=0
while IFS= read -r match; do
  ref="${match#*uses:}"
  ref="${ref%%#*}"
  ref="$(printf '%s' "$ref" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
  ref="${ref%\"}"
  ref="${ref#\"}"
  ref="${ref%\'}"
  ref="${ref#\'}"

  [[ "$ref" == ./* || "$ref" == docker://* ]] && continue
  version="${ref##*@}"
  if [[ ! "$version" =~ ^[0-9a-f]{40}$ ]]; then
    printf 'mutable GitHub Action reference: %s\n' "$match" >&2
    status=1
  fi
done < <(grep -RHnE '^[[:space:]]*-?[[:space:]]*uses:' .github/workflows --include='*.yml' --include='*.yaml')

exit "$status"
