#!/usr/bin/env bash
set -euo pipefail

ROOT="."
FORMAT="markdown"
SARIF="true"
FAIL_ON_CRITICAL="false"
INCLUDE_DEFAULTS="false"

usage() {
  cat <<'USAGE'
Fulcrum Boundary MCP audit action runner

Usage:
  mcp-audit.sh [flags]

Flags:
  --root <path>                 Repository root to audit (default ".")
  --format <markdown|sarif>     Primary report format (default "markdown")
  --sarif <true|false>          Generate SARIF report (default "true")
  --fail-on-critical <bool>     Exit non-zero when critical paths are found
  --include-defaults <bool>     Include user-level default MCP config paths
  --help                        Show this help
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --root)
      ROOT="${2:?--root requires a value}"
      shift 2
      ;;
    --format)
      FORMAT="${2:?--format requires a value}"
      shift 2
      ;;
    --sarif)
      SARIF="${2:?--sarif requires a value}"
      shift 2
      ;;
    --fail-on-critical)
      FAIL_ON_CRITICAL="${2:?--fail-on-critical requires a value}"
      shift 2
      ;;
    --include-defaults)
      INCLUDE_DEFAULTS="${2:?--include-defaults requires a value}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "mcp-audit: unknown flag $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

lower() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

bool_value() {
  case "$(lower "$1")" in
    true|1|yes|y|on)
      printf 'true'
      ;;
    false|0|no|n|off|"")
      printf 'false'
      ;;
    *)
      echo "mcp-audit: invalid boolean value $1" >&2
      exit 2
      ;;
  esac
}

FORMAT="$(lower "$FORMAT")"
case "$FORMAT" in
  markdown|md)
    FORMAT="markdown"
    ;;
  sarif)
    FORMAT="sarif"
    ;;
  *)
    echo "mcp-audit: unsupported format $FORMAT" >&2
    exit 2
    ;;
esac
SARIF="$(bool_value "$SARIF")"
FAIL_ON_CRITICAL="$(bool_value "$FAIL_ON_CRITICAL")"
INCLUDE_DEFAULTS="$(bool_value "$INCLUDE_DEFAULTS")"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BOUNDARY_REPO="${BOUNDARY_ACTION_REPO:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"
ROOT="$(cd "$ROOT" && pwd)"
REPORT_DIR="${BOUNDARY_MCP_AUDIT_OUT:-${RUNNER_TEMP:-${ROOT}/.boundary-mcp-audit}/boundary-mcp-audit}"
mkdir -p "$REPORT_DIR"

INVENTORY_JSON="${REPORT_DIR}/inventory.json"
INVENTORY_MARKDOWN="${REPORT_DIR}/inventory.md"
INVENTORY_SARIF="${REPORT_DIR}/inventory.sarif.json"
GRAPH_JSON="${REPORT_DIR}/risk-graph.json"
POLICY_DIR="${REPORT_DIR}/starter-policies"
SUMMARY_MARKDOWN="${REPORT_DIR}/summary.md"

run_boundary() {
  if [ -n "${BOUNDARY_BIN:-}" ]; then
    "$BOUNDARY_BIN" "$@"
    return
  fi
  (
    cd "$BOUNDARY_REPO"
    go run ./cmd/boundary "$@"
  )
}

run_boundary inventory \
  --root "$ROOT" \
  --include-defaults="$INCLUDE_DEFAULTS" \
  --format json \
  --out "$INVENTORY_JSON"

run_boundary inventory \
  --root "$ROOT" \
  --include-defaults="$INCLUDE_DEFAULTS" \
  --format markdown \
  --out "$INVENTORY_MARKDOWN"

run_boundary graph \
  --root "$ROOT" \
  --include-defaults="$INCLUDE_DEFAULTS" \
  --format json \
  --out "$GRAPH_JSON"

if [ "$SARIF" = "true" ] || [ "$FORMAT" = "sarif" ]; then
  run_boundary inventory \
    --root "$ROOT" \
    --include-defaults="$INCLUDE_DEFAULTS" \
    --format sarif \
    --out "$INVENTORY_SARIF"
else
  INVENTORY_SARIF=""
fi

rm -rf "$POLICY_DIR"
run_boundary policy generate --out "$POLICY_DIR" --force >/dev/null

python3 - "$INVENTORY_JSON" "$GRAPH_JSON" "$SUMMARY_MARKDOWN" <<'PY'
import json
import sys
from pathlib import Path

inventory_path = Path(sys.argv[1])
graph_path = Path(sys.argv[2])
summary_path = Path(sys.argv[3])

inventory = json.loads(inventory_path.read_text())
graph = json.loads(graph_path.read_text())

summary = inventory.get("summary") or {}
paths = graph.get("paths") or []
critical_paths = [p for p in paths if p.get("risk_class") == "W2"]
high_tools = []
for server in inventory.get("servers") or []:
    for capability in server.get("capabilities") or []:
        if capability.get("class") == "W1":
            high_tools.append((server.get("name", "unknown"), capability.get("name", "unknown")))

lines = [
    "# Fulcrum Boundary MCP Audit",
    "",
    f"MCP configs found: {summary.get('config_files', 0)}",
    f"Servers found: {summary.get('servers', 0)}",
    f"Critical paths: {len(critical_paths)}",
    f"High-risk tools: {len(high_tools)}",
    "",
]

if critical_paths:
    lines.append("## Critical")
    lines.append("")
    for path in critical_paths:
        source = path.get("source", "unknown_source")
        tool = path.get("tool", "unknown_tool")
        sink = path.get("sink", "unknown_sink")
        reason = path.get("reason", "Boundary classified this MCP path as critical.")
        lines.append(f"- `{source}` -> `{tool}` -> `{sink}`")
        lines.append(f"  - {reason}")
    lines.append("")
    lines.append("## Recommendation")
    lines.append("")
    lines.append("Use the Secure GitHub MCP preview profile or deny write-after-taint policies for governed routes.")
else:
    lines.append("No critical MCP risk paths were detected in repo-local configs.")
    lines.append("")
    lines.append("Generated starter policies are dry-run artifacts and still require operator review.")

summary_path.write_text("\n".join(lines) + "\n")

print(f"critical_count={len(critical_paths)}")
print(f"high_count={len(high_tools)}")
PY

COUNTS="$(python3 - "$INVENTORY_JSON" "$GRAPH_JSON" <<'PY'
import json
import sys

inventory = json.load(open(sys.argv[1]))
graph = json.load(open(sys.argv[2]))
summary = inventory.get("summary") or {}
critical = sum(1 for path in graph.get("paths") or [] if path.get("risk_class") == "W2")
high = sum(
    1
    for server in inventory.get("servers") or []
    for capability in server.get("capabilities") or []
    if capability.get("class") == "W1"
)
print(f"{critical} {high} {summary.get('config_files', 0)} {summary.get('servers', 0)}")
PY
)"
read -r CRITICAL_COUNT HIGH_COUNT CONFIG_COUNT SERVER_COUNT <<< "$COUNTS"

REPORT_PATH="$SUMMARY_MARKDOWN"
if [ "$FORMAT" = "sarif" ] && [ -n "$INVENTORY_SARIF" ]; then
  REPORT_PATH="$INVENTORY_SARIF"
fi

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  cat "$SUMMARY_MARKDOWN" >> "$GITHUB_STEP_SUMMARY"
fi

if [ -n "${GITHUB_OUTPUT:-}" ]; then
  {
    echo "critical-count=${CRITICAL_COUNT}"
    echo "high-count=${HIGH_COUNT}"
    echo "report-path=${REPORT_PATH}"
    echo "sarif-path=${INVENTORY_SARIF}"
  } >> "$GITHUB_OUTPUT"
fi

echo "Fulcrum Boundary MCP Audit"
echo "MCP configs found: ${CONFIG_COUNT}"
echo "Servers found: ${SERVER_COUNT}"
echo "Critical paths: ${CRITICAL_COUNT}"
echo "High-risk tools: ${HIGH_COUNT}"
echo "Report: ${REPORT_PATH}"
if [ -n "$INVENTORY_SARIF" ]; then
  echo "SARIF: ${INVENTORY_SARIF}"
fi

if [ "$FAIL_ON_CRITICAL" = "true" ] && [ "$CRITICAL_COUNT" -gt 0 ]; then
  echo "mcp-audit: critical MCP risk paths found" >&2
  exit 1
fi
