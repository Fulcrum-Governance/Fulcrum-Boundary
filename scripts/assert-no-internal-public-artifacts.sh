#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

paths=()
for path in README.md docs docs-site .github actions mkdocs.yml; do
  if [[ -e "$path" ]]; then
    paths+=("$path")
  fi
done

if [[ "${#paths[@]}" -eq 0 ]]; then
  exit 0
fi

grep_args=(
  -RInE
  --exclude-dir=.git
  --exclude-dir=.boundary
  --exclude-dir=site
  --exclude='*.gif'
  --exclude='*.mp4'
  --exclude='*.png'
  --exclude='*.jpg'
  --exclude='*.jpeg'
  --exclude='*.webp'
  --exclude='*.ico'
  --exclude='*.pdf'
)

fail=0

check_pattern() {
  local label="$1"
  local pattern="$2"
  local tmp
  tmp="$(mktemp)"

  if grep "${grep_args[@]}" -i -- "$pattern" "${paths[@]}" >"$tmp" 2>/dev/null; then
    printf 'FAIL: %s found in public surfaces\n' "$label" >&2
    cat "$tmp" >&2
    fail=1
  fi

  rm -f "$tmp"
}

check_pattern "internal planning/session artifact" '(/Users/td|CODEX_SESSION_LOG|docs/superpowers|\.claude/sprint|create_goal|update_goal|get_goal|goal usage|::git-|YC_DEMO_NARRATIVE|Y Combinator|YC version|Next_Spec|Codex execution)'
check_pattern "capture instructions substituting for demo assets" '(terminal capture|terminal screenshot|terminal gif|screenshot script|first screenshot|first moving visual|record the first|asciinema|demo\.tape|record-demo|QuickTime|ScreenFlow)'

if [[ "$fail" -ne 0 ]]; then
  cat >&2 <<'EOF'

Public surfaces must not contain internal planning/session artifacts or capture
instructions that substitute for committed product-facing demo assets. Move
private planning into ignored sprint folders and point public docs at finished
assets or product docs.
EOF
  exit 1
fi
