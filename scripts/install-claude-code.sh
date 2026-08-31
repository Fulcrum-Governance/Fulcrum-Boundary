#!/bin/sh
# install-claude-code.sh — install the `boundary` binary and/or wire the
# Fulcrum Boundary Claude Code plugin, with a byte-for-byte reversible receipt.
#
# POSIX sh, no sudo. Three modes, picked by how you invoke it:
#
#   (default)      Install the `boundary` binary onto PATH: try
#                  `brew install fulcrum-governance/tap/boundary`, and if
#                  Homebrew is missing or the tap install fails, fall back to
#                  downloading the latest GitHub release for your OS/arch,
#                  verified against the release's SHA256SUMS, into
#                  ~/.local/bin.
#   --plugin-drop  Copy this checkout's plugin content (the plugin manifest,
#                  hooks, skills, and the Claude Code integration scripts)
#                  into ~/.claude/skills/boundary/, so Claude Code loads it
#                  next session with zero marketplace steps. Must be run from
#                  a checkout of the repo (it copies from beside this script);
#                  it does not touch the network.
#   --uninstall    Reverse every install this script recorded, byte for byte,
#                  by reading the receipt(s) it wrote. Refuses — rather than
#                  guessing — when no receipt is found.
#
# Both install modes are idempotent: running the same mode again replaces its
# own prior receipt and its own prior files rather than duplicating anything.
# Every action is printed as it happens; nothing here is silent.
#
# This script does not touch Claude Code's settings.json and does not itself
# add the `boundary` binary to any policy. It gets the binary onto PATH and/or
# gets the plugin (manifest + hooks) into Claude Code's plugin search path.
# Boundary governs only the tool calls the resulting hook is wired to
# intercept — see docs/integrations/CLAUDE_CODE_HOOK.md and LIMITATIONS.md.
#
# Environment:
#   BOUNDARY_INSTALL_NO_NETWORK   When set (any non-empty value), the default
#                                 (binary) mode never calls brew or curl; it
#                                 only prints what it would have done. This
#                                 exists so tests and CI dry runs never touch
#                                 the network — it is not meant for normal use.
#   BOUNDARY_INSTALL_BIN_DIR      Override the binary install directory
#                                 (default: $HOME/.local/bin).
#   BOUNDARY_PLUGIN_DROP_DIR      Override the plugin-drop target directory
#                                 (default: $HOME/.claude/skills/boundary).
#   BOUNDARY_INSTALL_STATE_DIR    Override where install receipts are written
#                                 (default: $HOME/.local/state/boundary).

set -u

PROG_NAME=${0##*/}
REPO_SLUG="fulcrum-governance/fulcrum-boundary"
GITHUB_REPO_URL="https://github.com/$REPO_SLUG"
BREW_TAP_FORMULA="fulcrum-governance/tap/boundary"
INSTALLER_RAW_URL="https://raw.githubusercontent.com/$REPO_SLUG/main/scripts/install-claude-code.sh"

INSTALL_BIN_DIR="${BOUNDARY_INSTALL_BIN_DIR:-$HOME/.local/bin}"
PLUGIN_DROP_DIR="${BOUNDARY_PLUGIN_DROP_DIR:-$HOME/.claude/skills/boundary}"
STATE_DIR="${BOUNDARY_INSTALL_STATE_DIR:-$HOME/.local/state/boundary}"
PLUGIN_DROP_RECEIPT="$STATE_DIR/plugin-drop.receipt"
BINARY_RECEIPT="$STATE_DIR/binary.receipt"

say() { printf '%s\n' "$*"; }
err() { printf 'error: %s\n' "$*" >&2; }

print_help() {
	cat <<EOF
Usage: $PROG_NAME [--plugin-drop | --uninstall] [--help]

Install the boundary binary and/or wire the Claude Code plugin, with a
byte-for-byte reversible install receipt. No sudo is used or required.

Modes (pick one; default if none given):
  (default)       Install the 'boundary' binary: try
                    brew install $BREW_TAP_FORMULA
                  and fall back to downloading the latest GitHub release for
                  your OS/arch, verified against SHA256SUMS, into
                  $INSTALL_BIN_DIR.
  --plugin-drop   Copy this checkout's plugin content (manifest, hooks,
                  skills, and the Claude Code integration scripts) into
                    $PLUGIN_DROP_DIR
                  so it auto-loads next Claude Code session with zero
                  marketplace steps. Must be run from a checkout of
                  $GITHUB_REPO_URL (it copies from beside this script);
                  touches no network.
  --uninstall     Reverse every install recorded under
                    $STATE_DIR
                  byte for byte. Refuses if nothing is recorded, rather than
                  guessing what to remove.
  --help, -h      Show this help and exit.

Every action is printed as it happens. Both install modes are idempotent:
running the same mode twice replaces its own prior receipt and files rather
than duplicating anything.

Environment:
  BOUNDARY_INSTALL_NO_NETWORK   When set, the default (binary) mode never
                                calls brew or curl; it only prints what it
                                would have done. For tests and dry runs.
  BOUNDARY_INSTALL_BIN_DIR      Override the binary install directory
                                (default: \$HOME/.local/bin).
  BOUNDARY_PLUGIN_DROP_DIR      Override the plugin-drop target directory
                                (default: \$HOME/.claude/skills/boundary).
  BOUNDARY_INSTALL_STATE_DIR    Override where install receipts are written
                                (default: \$HOME/.local/state/boundary).
EOF
}

# resolve_repo_root prints the checkout root this script ships in, derived
# from \$0 rather than \$PWD, so --plugin-drop works regardless of the
# caller's working directory. Prints nothing and returns non-zero if \$0
# cannot be resolved to a directory.
resolve_repo_root() {
	script_path=$0
	case "$script_path" in
	/*) : ;;
	*) script_path="$PWD/$script_path" ;;
	esac
	script_dir=$(CDPATH= cd -- "$(dirname -- "$script_path")" 2>/dev/null && pwd) || return 1
	CDPATH= cd -- "$script_dir/.." 2>/dev/null && pwd
}

# hook_lane_ok probes whether the boundary at $1 carries the `hook` lane this
# plugin's wrapper execs into — the same side-effect-free probe
# integrations/claude-code/pretooluse-boundary.sh runs before every decision.
# It decides nothing, writes no record, and never reads stdin.
hook_lane_ok() {
	"$1" hook --help >/dev/null 2>&1
}

# receipt_records_path reports whether the binary receipt this script wrote
# names $1 as its installed path — i.e. whether the binary is one this
# script's --uninstall would actually manage. A binary this script never
# recorded must not be pointed at --uninstall, which would refuse (no
# receipt) or reverse something else entirely.
receipt_records_path() {
	[ -f "$BINARY_RECEIPT" ] || return 1
	while read_receipt_line; do
		if [ "$key" = "path" ] && [ "$value" = "$1" ]; then
			return 0
		fi
	done <"$BINARY_RECEIPT"
	return 1
}

# self_invocation prints a command line that re-runs this installer with the
# arguments given, in a form the CURRENT invocation proves is executable:
# when $0 names a readable file, that file; otherwise this is a piped
# `curl ... | sh` run — $0 is the shell's own name, not a path anyone can
# re-execute — so the documented re-pipe is printed, with the arguments after
# `sh -s --`.
self_invocation() {
	if [ -f "$0" ]; then
		if [ $# -gt 0 ]; then
			printf "sh '%s' %s" "$0" "$*"
		else
			printf "sh '%s'" "$0"
		fi
		return 0
	fi
	if [ $# -gt 0 ]; then
		printf 'curl -fsSL %s | sh -s -- %s' "$INSTALLER_RAW_URL" "$*"
	else
		printf 'curl -fsSL %s | sh' "$INSTALLER_RAW_URL"
	fi
}

# print_upgrade_command prints supported resolution guidance for the
# incompatible binary at $1, through the printer named by $2 (say or err).
#
# Ownership is never inferred from the install path alone: a `boundary` under
# a Homebrew prefix may be an older Fulcrum build or an entirely different
# product that shares the name. The binary's own `version` self-report decides
# which guidance applies, and Homebrew is named only when the binary
# self-reports as Fulcrum Boundary, sits under brew's prefix, AND `brew list`
# itself reports a boundary install (formula or cask).
print_upgrade_command() {
	old_path=$1
	printer=$2
	if "$old_path" version 2>/dev/null | grep -q '^Fulcrum Boundary'; then
		brew_prefix=""
		if command -v brew >/dev/null 2>&1; then
			brew_prefix=$(brew --prefix 2>/dev/null || true)
		fi
		if [ -n "$brew_prefix" ]; then
			case "$old_path" in
			"$brew_prefix"/*)
				if brew list --versions boundary >/dev/null 2>&1; then
					"$printer" "it self-reports as Fulcrum Boundary and Homebrew lists a 'boundary' formula;"
					"$printer" "upgrade it with exactly: brew upgrade boundary"
					return 0
				fi
				if brew list --cask --versions boundary >/dev/null 2>&1; then
					"$printer" "it self-reports as Fulcrum Boundary and Homebrew lists a 'boundary' cask;"
					"$printer" "upgrade it with exactly: brew upgrade --cask boundary"
					return 0
				fi
				;;
			esac
		fi
		"$printer" "it self-reports as Fulcrum Boundary; upgrade it by removing that binary and re-running:"
		"$printer" "  $(self_invocation)"
		"$printer" "(or set BOUNDARY_BIN to the absolute path of a current binary; the hook wrapper reads it)"
		return 0
	fi
	"$printer" "that binary may be an older Fulcrum Boundary build or a different product that shares the name."
	"$printer" "If it is not Fulcrum Boundary, leave it in place and set BOUNDARY_BIN to a current Fulcrum"
	"$printer" "binary (install one with: $(self_invocation)); if it is, upgrade it via whatever installed it."
}

# write_receipt atomically (best-effort) writes $2 as the body of a receipt at
# $1, preceded by a schema line and a timestamp. Receipts are plain
# key=value lines, one per line, deliberately not JSON: this script is the
# only reader, `--uninstall` parses it with a plain `while read`, and keeping
# it dependency-free (no jq) matches the rest of this integration.
#
# The body is written VERBATIM, so a caller must end it with a newline. Build
# receipt bodies by plain concatenation, never with `$(printf ...)`: command
# substitution strips every trailing newline, which leaves the receipt's last
# line unterminated. `--uninstall` reads those lines defensively (see
# read_receipt_line) so an already-written receipt of that shape still reverses,
# but the file it writes should not need the defense in the first place.
write_receipt() {
	receipt_dir=$(dirname "$1")
	mkdir -p "$receipt_dir" || return 1
	{
		printf 'schema_version=1\n'
		printf 'installed_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
		printf '%s' "$2"
	} >"$1"
}

# ---------------------------------------------------------------------------
# --plugin-drop
# ---------------------------------------------------------------------------

# PLUGIN_DROP_COMPONENTS lists what gets copied, one "repo-relative-path:required"
# entry per line. "skills" is marked optional because it may not exist in
# every checkout yet; the rest are required because a plugin-drop without them
# is not a working plugin.
PLUGIN_DROP_COMPONENTS='.claude-plugin:yes
hooks:yes
integrations/claude-code:yes
skills:no'

do_plugin_drop() {
	repo_root=$(resolve_repo_root)
	if [ -z "$repo_root" ] || [ ! -f "$repo_root/.claude-plugin/plugin.json" ]; then
		err "could not find a Fulcrum Boundary checkout beside this script (looked under: ${repo_root:-<unresolved>})"
		err "run --plugin-drop from a checkout of $GITHUB_REPO_URL, e.g.:"
		err "  git clone $GITHUB_REPO_URL && cd fulcrum-boundary && ./scripts/install-claude-code.sh --plugin-drop"
		return 1
	fi

	# Compatibility preflight, BEFORE any file or receipt is written, against
	# the binary the shipped wrapper will actually execute:
	# pretooluse-boundary.sh runs ${BOUNDARY_BIN:-boundary}, so that exact
	# resolution — an explicit BOUNDARY_BIN first, else `boundary` on PATH —
	# is what gets validated here, never an unrelated PATH binary the wrapper
	# would ignore. An incompatible or unresolvable effective binary stops the
	# drop instead of poisoning it. Only the no-override, no-binary case
	# proceeds: the wrapper then asks visibly, with install instructions,
	# until a binary appears — documented, deliberate behavior.
	validated_path=""
	validated_via=""
	if [ -n "${BOUNDARY_BIN:-}" ]; then
		if ! command -v "$BOUNDARY_BIN" >/dev/null 2>&1; then
			err "BOUNDARY_BIN is set to '$BOUNDARY_BIN', but that is not an executable command,"
			err "and the shipped hook executes \${BOUNDARY_BIN:-boundary} — it would fail against"
			err "this override rather than fall back to PATH. Nothing was installed and no"
			err "receipt was written. Fix or unset BOUNDARY_BIN, then re-run:"
			err "  $(self_invocation --plugin-drop)"
			return 1
		fi
		validated_path=$(command -v "$BOUNDARY_BIN")
		validated_via="BOUNDARY_BIN"
	elif command -v boundary >/dev/null 2>&1; then
		validated_path=$(command -v boundary)
		validated_via="PATH"
	fi
	if [ -n "$validated_path" ] && ! hook_lane_ok "$validated_path"; then
		err "the hook's effective binary ($validated_via: $validated_path) does not support"
		err "'boundary hook pretooluse', which this plugin's hook requires. Nothing was"
		err "installed and no receipt was written."
		print_upgrade_command "$validated_path" err
		err "Then re-run: $(self_invocation --plugin-drop)"
		return 1
	fi

	say "Plugin drop: installing the boundary plugin into $PLUGIN_DROP_DIR"
	say "  (source checkout: $repo_root)"
	if ! mkdir -p "$PLUGIN_DROP_DIR"; then
		err "could not create $PLUGIN_DROP_DIR"
		return 1
	fi

	receipt_body="target_dir=$PLUGIN_DROP_DIR
"
	all_ok=1
	while IFS=':' read -r src_rel required; do
		[ -z "$src_rel" ] && continue
		src="$repo_root/$src_rel"
		dest="$PLUGIN_DROP_DIR/$src_rel"
		if [ ! -e "$src" ]; then
			if [ "$required" = "yes" ]; then
				err "required plugin component missing from checkout: $src"
				all_ok=0
				break
			fi
			say "  (skip) $src_rel — not present in this checkout yet"
			continue
		fi
		# Idempotent: clear any prior copy at this destination before copying
		# fresh, so a re-run reflects the current source exactly rather than
		# merging old and new content.
		if [ -e "$dest" ] || [ -L "$dest" ]; then
			rm -rf "$dest"
		fi
		dest_parent=$(dirname "$dest")
		if ! mkdir -p "$dest_parent"; then
			err "could not create $dest_parent"
			all_ok=0
			break
		fi
		if ! cp -R "$src" "$dest"; then
			err "could not copy $src -> $dest"
			all_ok=0
			break
		fi
		say "  copied $src_rel -> $dest"
		receipt_body="${receipt_body}path=$dest
"
	done <<COMPONENTS
$PLUGIN_DROP_COMPONENTS
COMPONENTS

	if [ "$all_ok" -ne 1 ]; then
		return 1
	fi

	if ! write_receipt "$PLUGIN_DROP_RECEIPT" "$receipt_body"; then
		err "could not write receipt to $PLUGIN_DROP_RECEIPT"
		return 1
	fi
	say "  wrote receipt: $PLUGIN_DROP_RECEIPT"

	if [ -n "$validated_path" ]; then
		# The preflight above already proved this exact binary carries the
		# hook lane; name it, so the user knows what will decide.
		say ""
		if [ "$validated_via" = "BOUNDARY_BIN" ]; then
			say "The hook's effective binary is BOUNDARY_BIN ($validated_path); validated: it supports the hook."
			say "Note: BOUNDARY_BIN must be present in the environment that LAUNCHES Claude Code —"
			say "a value exported only in this shell does not reach the hook, which would then"
			say "fall back to 'boundary' on PATH."
		else
			say "The hook's effective binary is boundary on PATH ($validated_path); validated: it supports the hook."
		fi
	else
		say ""
		say "Note: no 'boundary' binary was found on PATH (and BOUNDARY_BIN is not set). The"
		say "hook installed above asks rather than silently allowing until one is present —"
		say "install it with '$(self_invocation)', or 'brew install $BREW_TAP_FORMULA', or 'make build'."
	fi
	say ""
	say "To reverse this install: $(self_invocation --uninstall)"
	say "Restart Claude Code, then run /boundary:drill."
	return 0
}

# ---------------------------------------------------------------------------
# default: binary install
# ---------------------------------------------------------------------------

# detect_os_arch prints "<goos> <goarch>" for the release-asset naming this
# repo's goreleaser config produces, or returns non-zero on an OS/arch this
# script does not know how to map.
detect_os_arch() {
	os_raw=$(uname -s)
	arch_raw=$(uname -m)
	case "$os_raw" in
	Darwin) goos=darwin ;;
	Linux) goos=linux ;;
	*) return 1 ;;
	esac
	case "$arch_raw" in
	x86_64 | amd64) goarch=amd64 ;;
	arm64 | aarch64) goarch=arm64 ;;
	*) return 1 ;;
	esac
	printf '%s %s\n' "$goos" "$goarch"
}

do_binary_install() {
	if command -v boundary >/dev/null 2>&1; then
		existing_path=$(command -v boundary)
		# "already installed" is only true when the installed binary can do the
		# job the plugin needs. A boundary that predates the `hook` lane would
		# leave the plugin asking on every tool call while this script reports
		# "nothing to install" — the exact poisoned state G3-A blocker 6 names.
		if hook_lane_ok "$existing_path"; then
			say "boundary is already on PATH at $existing_path and supports the Claude Code hook; nothing to install."
			if receipt_records_path "$existing_path"; then
				say "This script's receipt records that install; to reverse it: $(self_invocation --uninstall)"
			else
				say "That binary was not installed or recorded by this script, so '--uninstall' here"
				say "will not manage it; upgrade or remove it via whatever method installed it."
			fi
			return 0
		fi
		err "a 'boundary' on PATH at $existing_path does not support 'boundary hook pretooluse',"
		err "which the Claude Code hook requires; the plugin's wrapper would ask on every tool"
		err "call instead of deciding. Refusing to report it as installed;"
		print_upgrade_command "$existing_path" err
		return 1
	fi

	if [ -n "${BOUNDARY_INSTALL_NO_NETWORK:-}" ]; then
		say "BOUNDARY_INSTALL_NO_NETWORK is set: skipping the brew/curl network install."
		say "In a real run this would try:"
		say "  brew install $BREW_TAP_FORMULA"
		say "and, on failure or a missing brew, download the latest GitHub release for"
		say "this OS/arch and verify it against SHA256SUMS before installing it to"
		say "$INSTALL_BIN_DIR."
		return 0
	fi

	if command -v brew >/dev/null 2>&1; then
		say "Homebrew found. Trying: brew install $BREW_TAP_FORMULA"
		if brew install "$BREW_TAP_FORMULA"; then
			installed_path=$(command -v boundary 2>/dev/null || true)
			if [ -z "$installed_path" ]; then
				err "brew install reported success but 'boundary' is not on PATH; not recording a receipt"
				return 1
			fi
			# Built by concatenation, not `$(printf ...)`: command substitution
			# strips the trailing newline, and an unterminated `path=` line is a
			# line `--uninstall` must not have to guess at. See write_receipt.
			receipt_body="mode=binary
method=brew
path=$installed_path
"
			if ! write_receipt "$BINARY_RECEIPT" "$receipt_body"; then
				err "could not write receipt to $BINARY_RECEIPT"
				return 1
			fi
			say "Installed via Homebrew: $installed_path"
			say "  wrote receipt: $BINARY_RECEIPT"
			say ""
			say "To reverse this install: $(self_invocation --uninstall)"
			say "Next, wire it into Claude Code: $(self_invocation --plugin-drop)"
			say "Then restart Claude Code, then run /boundary:drill."
			return 0
		fi
		say "brew install did not succeed (Homebrew missing the tap, or another brew"
		say "error); falling back to a direct release download."
	else
		say "Homebrew not found; downloading a release binary directly."
	fi

	if ! os_arch=$(detect_os_arch); then
		err "unsupported OS/arch ($(uname -s)/$(uname -m)) for the direct-download path"
		err "download a binary by hand from $GITHUB_REPO_URL/releases and verify it"
		err "against that release's SHA256SUMS"
		return 1
	fi
	goos=${os_arch% *}
	goarch=${os_arch#* }

	if command -v sha256sum >/dev/null 2>&1; then
		sha_tool="sha256sum"
	elif command -v shasum >/dev/null 2>&1; then
		sha_tool="shasum -a 256"
	else
		err "neither sha256sum nor shasum is available; cannot verify a downloaded"
		err "binary, so refusing to install one unverified"
		return 1
	fi
	if ! command -v curl >/dev/null 2>&1; then
		err "curl is not available; cannot download a release"
		return 1
	fi
	if ! command -v tar >/dev/null 2>&1; then
		err "tar is not available; cannot extract a release archive"
		return 1
	fi

	say "Resolving the latest release from $GITHUB_REPO_URL ..."
	latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$GITHUB_REPO_URL/releases/latest")
	if [ -z "$latest_url" ]; then
		err "could not resolve the latest release (network unreachable?)"
		return 1
	fi
	tag=${latest_url##*/}
	if [ -z "$tag" ] || [ "$tag" = "latest" ]; then
		err "could not parse a release tag out of $latest_url"
		return 1
	fi
	version=${tag#v}
	say "Latest release: $tag"

	asset="boundary_${version}_${goos}_${goarch}_static-nocgo.tar.gz"
	base_url="$GITHUB_REPO_URL/releases/download/$tag"

	stage=$(mktemp -d) || {
		err "mktemp -d failed"
		return 1
	}
	trap 'rm -rf "$stage"' EXIT INT TERM

	say "Downloading $asset ..."
	if ! curl -fsSL -o "$stage/$asset" "$base_url/$asset"; then
		err "download failed: $base_url/$asset"
		return 1
	fi
	say "Downloading SHA256SUMS ..."
	if ! curl -fsSL -o "$stage/SHA256SUMS" "$base_url/SHA256SUMS"; then
		err "download failed: $base_url/SHA256SUMS"
		return 1
	fi

	say "Verifying checksum ..."
	# Select by the complete filename field, never a substring: every archive
	# name in the manifest is a prefix of its own `<archive>.spdx.json` SBOM
	# entry, so an unanchored match selects both lines and verification then
	# fails on the never-downloaded SBOM.
	expected_line=$(awk -v name="$asset" '$2 == name' "$stage/SHA256SUMS")
	match_count=$(awk -v name="$asset" '$2 == name { n++ } END { print n + 0 }' "$stage/SHA256SUMS")
	if [ "$match_count" -eq 0 ]; then
		err "no SHA256SUMS entry for $asset; refusing to install an unverifiable binary"
		return 1
	fi
	if [ "$match_count" -ne 1 ]; then
		err "$match_count SHA256SUMS entries for $asset; refusing to install against an ambiguous manifest"
		return 1
	fi
	printf '%s\n' "$expected_line" >"$stage/SHA256SUMS.expected"
	if ! (cd "$stage" && $sha_tool -c SHA256SUMS.expected) >/dev/null 2>&1; then
		err "checksum verification FAILED for $asset; refusing to install"
		return 1
	fi
	say "Checksum OK."

	say "Extracting ..."
	if ! tar -xzf "$stage/$asset" -C "$stage"; then
		err "extraction failed"
		return 1
	fi
	if [ ! -f "$stage/boundary" ]; then
		err "extracted archive did not contain a 'boundary' binary at its root"
		return 1
	fi

	if ! mkdir -p "$INSTALL_BIN_DIR"; then
		err "could not create $INSTALL_BIN_DIR"
		return 1
	fi
	if ! cp "$stage/boundary" "$INSTALL_BIN_DIR/boundary"; then
		err "could not install to $INSTALL_BIN_DIR/boundary"
		return 1
	fi
	chmod 0755 "$INSTALL_BIN_DIR/boundary"

	# Concatenated, not `$(printf ...)` — see write_receipt and the brew branch.
	receipt_body="mode=binary
method=release-download
version=$tag
path=$INSTALL_BIN_DIR/boundary
"
	if ! write_receipt "$BINARY_RECEIPT" "$receipt_body"; then
		err "could not write receipt to $BINARY_RECEIPT"
		return 1
	fi

	say "Installed: $INSTALL_BIN_DIR/boundary ($tag, $goos/$goarch, static-nocgo)"
	say "  wrote receipt: $BINARY_RECEIPT"
	case ":$PATH:" in
	*":$INSTALL_BIN_DIR:"*) : ;;
	*)
		say ""
		say "$INSTALL_BIN_DIR is not on your PATH. Add this to your shell profile:"
		say "  export PATH=\"$INSTALL_BIN_DIR:\$PATH\""
		;;
	esac
	say ""
	say "To reverse this install: $(self_invocation --uninstall)"
	say "Next, wire it into Claude Code: $(self_invocation --plugin-drop)"
	say "Then restart Claude Code, then run /boundary:drill."
	return 0
}

# ---------------------------------------------------------------------------
# --uninstall
# ---------------------------------------------------------------------------

# read_receipt_line reads one `key=value` receipt line from stdin into the
# caller's $key and $value, and returns non-zero only at true end of file.
#
# The `|| [ -n "$key" ]` is load-bearing, not defensive noise. POSIX `read`
# returns non-zero when it hits EOF without a terminating newline, so a plain
# `while IFS='=' read -r key value` SILENTLY DROPS a receipt's final line when
# that line is unterminated — and an --uninstall that drops the last `path=`
# line deletes the receipt while leaving the installed file behind, permanently
# unmanaged. Receipts this script writes now end with a newline (see
# write_receipt), so this guard is what keeps a receipt written by an older
# version, or hand-edited, reversing correctly too.
read_receipt_line() {
	IFS='=' read -r key value || [ -n "$key" ]
}

# rmdir_empty_ancestors removes $1's parent directory, then its parent, and so
# on, stopping the moment one is not empty or once it reaches $2 (a directory
# to leave for the caller to remove, or keep, on its own). This exists so
# --uninstall does not leave empty directories behind that a nested
# plugin-drop component (e.g. integrations/claude-code, whose parent
# integrations/ was created only to hold it) created as a side effect.
rmdir_empty_ancestors() {
	dir=$(dirname "$1")
	stop=$2
	while [ "$dir" != "$stop" ] && [ "$dir" != "/" ] && [ "$dir" != "." ]; do
		if rmdir "$dir" 2>/dev/null; then
			say "  removed now-empty $dir"
			dir=$(dirname "$dir")
		else
			break
		fi
	done
}

do_uninstall() {
	found_any=0

	if [ -f "$PLUGIN_DROP_RECEIPT" ]; then
		found_any=1
		say "Reversing plugin-drop install recorded at $PLUGIN_DROP_RECEIPT"
		target_dir=""
		while read_receipt_line; do
			case "$key" in
			target_dir) target_dir=$value ;;
			path)
				if [ -e "$value" ] || [ -L "$value" ]; then
					rm -rf "$value"
					say "  removed $value"
					rmdir_empty_ancestors "$value" "$target_dir"
				else
					say "  (already gone) $value"
				fi
				;;
			esac
		done <"$PLUGIN_DROP_RECEIPT"
		if [ -n "$target_dir" ] && [ -d "$target_dir" ]; then
			if rmdir "$target_dir" 2>/dev/null; then
				say "  removed now-empty $target_dir"
			else
				say "  left $target_dir in place (not empty; something else is using it)"
			fi
		fi
		rm -f "$PLUGIN_DROP_RECEIPT"
		say "  removed receipt $PLUGIN_DROP_RECEIPT"
	fi

	if [ -f "$BINARY_RECEIPT" ]; then
		found_any=1
		say "Reversing binary install recorded at $BINARY_RECEIPT"
		# method= precedes path= in every receipt this script writes, so a single
		# pass sees the method before it has to act on the path. A brew-managed
		# path is a symlink into the Homebrew keg: removing it by hand would
		# orphan the keg and corrupt brew's link state, so brew uninstalls brew.
		receipt_method=""
		while read_receipt_line; do
			case "$key" in
			method)
				receipt_method="$value"
				;;
			path)
				if [ "$receipt_method" = "brew" ]; then
					say "  leaving $value to Homebrew; run: brew uninstall $BREW_TAP_FORMULA"
				elif [ -e "$value" ]; then
					rm -f "$value"
					say "  removed $value"
				else
					say "  (already gone) $value"
				fi
				;;
			esac
		done <"$BINARY_RECEIPT"
		rm -f "$BINARY_RECEIPT"
		say "  removed receipt $BINARY_RECEIPT"
	fi

	if [ "$found_any" -eq 0 ]; then
		err "nothing to uninstall: no install receipt found under $STATE_DIR"
		err "(refusing rather than guessing what to remove; if you installed boundary"
		err "some other way, e.g. 'make build' or a manual brew install, remove it"
		err "yourself)"
		return 1
	fi

	rmdir "$STATE_DIR" 2>/dev/null || true
	say ""
	say "Uninstall complete."
	return 0
}

# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------

main() {
	mode=""
	for arg in "$@"; do
		case "$arg" in
		--help | -h)
			print_help
			exit 0
			;;
		--plugin-drop | --uninstall)
			new_mode=${arg#--}
			if [ -n "$mode" ] && [ "$mode" != "$new_mode" ]; then
				err "cannot combine --$mode with $arg; pick one mode"
				exit 1
			fi
			mode=$new_mode
			;;
		*)
			err "unknown argument: $arg"
			print_help >&2
			exit 1
			;;
		esac
	done
	mode=${mode:-binary}

	case "$mode" in
	binary) do_binary_install ;;
	plugin-drop) do_plugin_drop ;;
	uninstall) do_uninstall ;;
	esac
}

main "$@"
