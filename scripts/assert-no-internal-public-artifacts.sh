#!/usr/bin/env bash
set -euo pipefail

# Public-surface guard.
#
# Scans ALL tracked text files (git ls-files based) for internal
# planning/session leakage, retired-product framing, capture-instruction
# placeholders, and non-approved contact aliases. Fails CI on any hit so a
# cloner never reads internal scaffolding as repo truth.
#
# Scope: every tracked file EXCEPT binary assets and a small set of files that
# must legitimately reference the forbidden tokens to govern or ignore them
# (this guard script itself; .gitignore, which lists ignore patterns such as
# .claude/sprint/; docs/BOUNDARY_SPEC.md, the master language-control spec whose
# §12 forbidden-language appendix quotes the banned phrases to govern them). The
# campaign label "MCP Safety Gateway" is ALLOWED (C2); only the retired GIL /
# "Governance Interception Layer" framing is forbidden.

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "FAIL: public-surface guard must run inside a Git worktree" >&2
  exit 1
fi

# This guard's own path, relative to ROOT, so it is excluded from its own match
# (its detector strings are a known, intentional false positive).
SELF_REL="scripts/assert-no-internal-public-artifacts.sh"

# Enumerate tracked text files. Exclusions:
#   - binary assets (gif/mp4/png/jpg/jpeg/webp/ico/pdf)
#   - .git / .boundary / site (build + local-state paths; not normally tracked,
#     filtered defensively)
#   - this guard script (self-exclude)
#   - .gitignore (legitimately lists ignore patterns including .claude/sprint/)
files=()
while IFS= read -r file; do
  files+=("$file")
done < <(
  git ls-files -z \
    | tr '\0' '\n' \
    | grep -viE '\.(gif|mp4|png|jpg|jpeg|webp|ico|pdf)$' \
    | grep -vE '^(\.git/|\.boundary/|site/)' \
    | grep -vxF "$SELF_REL" \
    | grep -vxF ".gitignore" \
    | grep -vxF "docs/BOUNDARY_SPEC.md"
)

if [[ "${#files[@]}" -eq 0 ]]; then
  exit 0
fi

fail=0

check_pattern() {
  local label="$1"
  local pattern="$2"
  local tmp
  tmp="$(mktemp)"

  # -I skips binary files; -n shows line numbers; -E extended regex; -i case-insensitive.
  if grep -InE -i -- "$pattern" "${files[@]}" >"$tmp" 2>/dev/null; then
    printf 'FAIL: %s found in tracked files\n' "$label" >&2
    cat "$tmp" >&2
    fail=1
  fi

  rm -f "$tmp"
}

check_pattern_sensitive() {
  # Case-sensitive variant for tokens whose meaning depends on case (e.g. the
  # standalone "GIL" / "YC" acronyms, which must not flag lowercase prose words).
  local label="$1"
  local pattern="$2"
  local tmp
  tmp="$(mktemp)"

  if grep -InE -- "$pattern" "${files[@]}" >"$tmp" 2>/dev/null; then
    printf 'FAIL: %s found in tracked files\n' "$label" >&2
    cat "$tmp" >&2
    fail=1
  fi

  rm -f "$tmp"
}

# 1. Internal planning / session artifacts and local absolute paths.
check_pattern "internal planning/session artifact" \
  '(/Users/|CODEX_SESSION_LOG|docs/superpowers|\.claude/sprint|create_goal|update_goal|get_goal|goal usage|::git-|Next_Spec|Codex execution|superpowers:|\*\*/superpowers/)'

# 2. YC / accelerator / pitch-narrative leakage (acronym case-sensitive; phrases any case).
check_pattern_sensitive "YC acronym / demo narrative" \
  '(\bYC\b|YC_DEMO_NARRATIVE)'
check_pattern "accelerator / pitch narrative" \
  '(Y Combinator|YC version|YC application|Founder Eyes)'

# 3. Retired GIL / governance-kernel product framing (C1).
#    Forbid the standalone GIL acronym and the two known phrasings; do NOT flag
#    the allowed campaign label "MCP Safety Gateway".
check_pattern "retired Governance Interception Layer framing" \
  '(Governance Interception Layer|Governance Interception)'
check_pattern_sensitive "retired standalone GIL product token" \
  '(\bGIL\b)'

# 4. Capture-instruction placeholders substituting for committed demo assets.
check_pattern "capture instructions substituting for demo assets" \
  '(terminal capture|terminal screenshot|terminal gif|screenshot script|first screenshot|first moving visual|record the first|asciinema|demo\.tape|record-demo|QuickTime|ScreenFlow)'

# 5. Non-approved @fulcrumlayer.io aliases. Only agent@fulcrumlayer.io is the
#    approved public contact; any other local-part is forbidden.
non_agent_aliases() {
  local tmp
  tmp="$(mktemp)"
  if grep -InoE -- '[A-Za-z0-9._%+-]+@fulcrumlayer\.io' "${files[@]}" 2>/dev/null \
      | grep -vE ':agent@fulcrumlayer\.io$' >"$tmp"; then
    printf 'FAIL: non-approved @fulcrumlayer.io alias found (only agent@fulcrumlayer.io is approved)\n' >&2
    cat "$tmp" >&2
    fail=1
  fi
  rm -f "$tmp"
}
non_agent_aliases

if [[ "$fail" -ne 0 ]]; then
  cat >&2 <<'EOF'

Tracked files must not contain internal planning/session artifacts, retired
GIL / governance-kernel product framing, capture-instruction placeholders, or
non-approved contact aliases. Move private planning into ignored folders, use
Boundary product language, point public docs at finished assets, and keep the
single approved public contact (agent@fulcrumlayer.io).
EOF
  exit 1
fi
