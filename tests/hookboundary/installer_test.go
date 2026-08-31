package hookboundary_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installerPath is the shipped installer under test.
func installerPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("install-claude-code.sh is a POSIX sh entrypoint")
	}
	path := filepath.Join(repoRoot, "scripts", "install-claude-code.sh")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("installer is missing: %v", err)
	}
	return path
}

// systemPathWithoutBoundary is a PATH built only from directories POSIX
// utilities (dirname, mkdir, cp, rm, date, uname, ...) live in, deliberately
// excluding any directory a real `boundary` might be installed to (a brew
// prefix, ~/.local/bin, a Go bin dir). Tests that need "boundary is not
// installed" use it so that condition is real without also hiding the
// ordinary system utilities the installer depends on.
func systemPathWithoutBoundary() string {
	return "/usr/bin:/bin"
}

// baseEnv is the minimal environment a plugin-drop or uninstall test needs:
// an isolated HOME (this is the "fake HOME" the installer's --plugin-drop and
// --uninstall targets are derived from) and a PATH that resolves ordinary
// utilities but not `boundary` itself.
func baseEnv(home string) []string {
	return []string{
		"HOME=" + home,
		"PATH=" + systemPathWithoutBoundary(),
	}
}

// runInstaller executes the installer with an explicit environment and no
// stdin, exactly as a person running it from a shell would.
func runInstaller(t *testing.T, env []string, args ...string) (int, string, string) {
	t.Helper()
	cmdArgs := append([]string{installerPath(t)}, args...)
	cmd := exec.Command("sh", cmdArgs...)
	// The installer resolves its own location from $0, not $PWD, so running
	// it from an unrelated directory is deliberate: it proves --plugin-drop
	// does not depend on the caller's working directory.
	cmd.Dir = t.TempDir()
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run installer: %v\nstderr=%s", err, stderr.String())
		}
		code = exitErr.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

// nonTimestampLines drops a receipt's installed_at= line before comparison,
// since that line legitimately differs between two runs of the same install;
// everything else in the receipt should not.
func nonTimestampLines(receipt string) string {
	var kept []string
	for _, line := range strings.Split(receipt, "\n") {
		if strings.HasPrefix(line, "installed_at=") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// TestInstallerSyntaxIsPOSIX is the `sh -n` gate: the installer must parse
// under a plain POSIX shell, not just under the developer's login shell.
func TestInstallerSyntaxIsPOSIX(t *testing.T) {
	output, err := exec.Command("sh", "-n", installerPath(t)).CombinedOutput()
	if err != nil {
		t.Fatalf("sh -n failed: %v\n%s", err, string(output))
	}
	if len(bytes.TrimSpace(output)) != 0 {
		t.Fatalf("sh -n produced output: %s", string(output))
	}
}

// TestInstallerHelp covers --help and its -h alias. Neither should touch the
// network or the filesystem beyond reading the script itself.
func TestInstallerHelp(t *testing.T) {
	code, stdout, stderr := runInstaller(t, baseEnv(t.TempDir()), "--help")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr)
	}
	for _, want := range []string{"Usage:", "--plugin-drop", "--uninstall", "--help", "No sudo"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help output missing %q:\n%s", want, stdout)
		}
	}

	code2, stdout2, stderr2 := runInstaller(t, baseEnv(t.TempDir()), "-h")
	if code2 != 0 {
		t.Fatalf("-h: exit = %d, want 0\nstderr=%s", code2, stderr2)
	}
	if !strings.Contains(stdout2, "Usage:") {
		t.Fatalf("-h did not behave like --help:\n%s", stdout2)
	}
}

// TestInstallerUninstallWithoutInstallRefuses is the headline safety
// behavior of --uninstall: with no receipt on disk it must refuse loudly
// rather than silently doing nothing or guessing what to remove.
func TestInstallerUninstallWithoutInstallRefuses(t *testing.T) {
	code, stdout, stderr := runInstaller(t, baseEnv(t.TempDir()), "--uninstall")
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero\nstdout=%s", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "nothing to uninstall") {
		t.Fatalf("stderr does not explain the refusal: %s", stderr)
	}
}

// TestInstallerPluginDropIntoFakeHome drives --plugin-drop end to end against
// an isolated HOME and checks the structure it leaves behind: the plugin
// content copied into ~/.claude/skills/boundary, the closing instruction, and
// a receipt that names every path it created.
func TestInstallerPluginDropIntoFakeHome(t *testing.T) {
	home := t.TempDir()
	code, stdout, stderr := runInstaller(t, baseEnv(home), "--plugin-drop")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}

	pluginDir := filepath.Join(home, ".claude", "skills", "boundary")
	mustExist(t, filepath.Join(pluginDir, ".claude-plugin", "plugin.json"))
	mustExist(t, filepath.Join(pluginDir, "hooks", "hooks.json"))
	mustExist(t, filepath.Join(pluginDir, "integrations", "claude-code", "pretooluse-boundary.sh"))
	mustExist(t, filepath.Join(pluginDir, "integrations", "claude-code", "sessionend-boundary.sh"))

	// skills/ is an optional component (the installer copies it only if the
	// checkout has it), so assert the copy conditionally on the source
	// existing rather than assuming it always will.
	if _, err := os.Stat(filepath.Join(repoRoot, "skills")); err == nil {
		mustExist(t, filepath.Join(pluginDir, "skills"))
	}

	if !strings.Contains(stdout, "Restart Claude Code, then run /boundary:drill.") {
		t.Fatalf("plugin-drop did not print the documented closing instruction:\n%s", stdout)
	}

	receiptPath := filepath.Join(home, ".local", "state", "boundary", "plugin-drop.receipt")
	receipt := mustReadFile(t, receiptPath)
	for _, want := range []string{
		"schema_version=1",
		"target_dir=" + pluginDir,
		"path=" + filepath.Join(pluginDir, ".claude-plugin"),
		"path=" + filepath.Join(pluginDir, "hooks"),
		"path=" + filepath.Join(pluginDir, "integrations", "claude-code"),
	} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("receipt missing %q:\nreceipt=%s", want, receipt)
		}
	}
}

// TestInstallerPluginDropRefusesOutsideACheckout proves --plugin-drop finds
// its source from beside the script (via $0), not from the caller's working
// directory: a standalone copy of the installer, with no repo content next to
// it, must fail clearly instead of silently doing nothing or grabbing files
// from somewhere unrelated.
func TestInstallerPluginDropRefusesOutsideACheckout(t *testing.T) {
	body, err := os.ReadFile(installerPath(t))
	if err != nil {
		t.Fatalf("read installer: %v", err)
	}
	isolated := t.TempDir()
	scriptsDir := filepath.Join(isolated, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	standalone := filepath.Join(scriptsDir, "install-claude-code.sh")
	if err := os.WriteFile(standalone, body, 0o755); err != nil {
		t.Fatalf("write standalone copy: %v", err)
	}

	cmd := exec.Command("sh", standalone, "--plugin-drop")
	cmd.Dir = t.TempDir()
	cmd.Env = baseEnv(t.TempDir())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr == nil {
		t.Fatalf("expected a failure outside a checkout, got exit 0\nstdout=%s", stdout.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "checkout") {
		t.Fatalf("stderr does not explain the missing checkout: %s", stderr.String())
	}
}

// TestInstallerPluginDropIsIdempotent re-runs --plugin-drop against the same
// HOME and requires the second run to replace its own receipt and files
// rather than duplicating or merging with the first.
func TestInstallerPluginDropIsIdempotent(t *testing.T) {
	home := t.TempDir()
	env := baseEnv(home)

	if code, stdout, stderr := runInstaller(t, env, "--plugin-drop"); code != 0 {
		t.Fatalf("first run: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	receiptPath := filepath.Join(home, ".local", "state", "boundary", "plugin-drop.receipt")
	first := mustReadFile(t, receiptPath)

	if code, stdout, stderr := runInstaller(t, env, "--plugin-drop"); code != 0 {
		t.Fatalf("second run: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	second := mustReadFile(t, receiptPath)

	if got, want := nonTimestampLines(second), nonTimestampLines(first); got != want {
		t.Fatalf("receipt shape changed across idempotent re-runs:\nfirst=%q\nsecond=%q", want, got)
	}

	pluginDir := filepath.Join(home, ".claude", "skills", "boundary")
	mustExist(t, filepath.Join(pluginDir, ".claude-plugin", "plugin.json"))
	mustExist(t, filepath.Join(pluginDir, "hooks", "hooks.json"))
}

// TestInstallerUninstallReversesPluginDropByteForByte is the round trip:
// --plugin-drop then --uninstall must leave the fake HOME exactly as it was
// (the plugin directory and the receipt both gone), and a second --uninstall
// must refuse exactly like the never-installed case, proving nothing was left
// half-reversed.
func TestInstallerUninstallReversesPluginDropByteForByte(t *testing.T) {
	home := t.TempDir()
	env := baseEnv(home)

	if code, stdout, stderr := runInstaller(t, env, "--plugin-drop"); code != 0 {
		t.Fatalf("plugin-drop: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	pluginDir := filepath.Join(home, ".claude", "skills", "boundary")
	mustExist(t, pluginDir)

	code, stdout, stderr := runInstaller(t, env, "--uninstall")
	if code != 0 {
		t.Fatalf("uninstall: exit = %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Fatalf("plugin-drop directory still present after uninstall (err=%v)", err)
	}
	receiptPath := filepath.Join(home, ".local", "state", "boundary", "plugin-drop.receipt")
	if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("receipt still present after uninstall (err=%v)", err)
	}

	code2, stdout2, stderr2 := runInstaller(t, env, "--uninstall")
	if code2 == 0 {
		t.Fatalf("second uninstall: exit = 0, want non-zero (nothing left to reverse)\nstdout=%s", stdout2)
	}
	if !strings.Contains(strings.ToLower(stderr2), "nothing to uninstall") {
		t.Fatalf("second uninstall stderr does not explain the refusal: %s", stderr2)
	}
}

// writeBinaryReceipt plants a binary install receipt in an isolated HOME,
// alongside the fake installed binary it names, and returns that binary's path.
//
// The direct-release lane is driven end to end by the fake-curl tests below;
// these receipt-shape cases still plant the receipt by hand because they pin
// how uninstall treats bodies the installer (or an older installer) already
// wrote, independent of any install run. terminator lets a caller choose whether the final
// `path=` line ends in a newline: a receipt an older installer wrote did not,
// because its body came from `$(printf ...)` and command substitution strips
// every trailing newline.
func writeBinaryReceipt(t *testing.T, home, method, terminator string) string {
	t.Helper()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", binDir, err)
	}
	installed := filepath.Join(binDir, "boundary")
	if err := os.WriteFile(installed, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	stateDir := filepath.Join(home, ".local", "state", "boundary")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", stateDir, err)
	}
	receipt := "schema_version=1\ninstalled_at=2026-01-01T00:00:00Z\n" +
		"mode=binary\nmethod=" + method + "\npath=" + installed + terminator
	if err := os.WriteFile(filepath.Join(stateDir, "binary.receipt"), []byte(receipt), 0o600); err != nil {
		t.Fatalf("write binary receipt: %v", err)
	}
	return installed
}

// TestInstallerUninstallRemovesTheRecordedBinary is the binary lane's round
// trip, and the regression guard for the failure that made --uninstall lie: a
// receipt whose final `path=` line carried no trailing newline was silently
// dropped by `while read`, so --uninstall removed the receipt, printed
// "Uninstall complete.", and left the binary installed and permanently
// unmanaged. Both shapes must fully reverse.
func TestInstallerUninstallRemovesTheRecordedBinary(t *testing.T) {
	for _, tc := range []struct {
		name       string
		terminator string
	}{
		{"newline terminated", "\n"},
		{"final line unterminated", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			env := baseEnv(home)
			installed := writeBinaryReceipt(t, home, "release-download", tc.terminator)

			code, stdout, stderr := runInstaller(t, env, "--uninstall")
			if code != 0 {
				t.Fatalf("uninstall: exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
			}
			if _, err := os.Stat(installed); !os.IsNotExist(err) {
				t.Fatalf("the recorded binary is still installed after uninstall (err=%v)\nstdout=%s", err, stdout)
			}
			if !strings.Contains(stdout, "removed "+installed) {
				t.Fatalf("uninstall did not report removing the binary it recorded:\n%s", stdout)
			}
			receiptPath := filepath.Join(home, ".local", "state", "boundary", "binary.receipt")
			if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
				t.Fatalf("binary receipt still present after uninstall (err=%v)", err)
			}

			code2, stdout2, stderr2 := runInstaller(t, env, "--uninstall")
			if code2 == 0 {
				t.Fatalf("second uninstall: exit = 0, want non-zero (nothing left to reverse)\nstdout=%s", stdout2)
			}
			if !strings.Contains(strings.ToLower(stderr2), "nothing to uninstall") {
				t.Fatalf("second uninstall stderr does not explain the refusal: %s", stderr2)
			}
		})
	}
}

// TestInstallerUninstallLeavesABrewBinaryToBrew pins the Homebrew posture: a
// brew-installed path is a symlink into the keg, and removing it by hand would
// orphan the keg and corrupt brew's link state while claiming byte-for-byte
// reversal. Uninstall must leave the path, say so, point at `brew uninstall`,
// and still retire the receipt.
func TestInstallerUninstallLeavesABrewBinaryToBrew(t *testing.T) {
	home := t.TempDir()
	env := baseEnv(home)
	installed := writeBinaryReceipt(t, home, "brew", "\n")

	code, stdout, stderr := runInstaller(t, env, "--uninstall")
	if code != 0 {
		t.Fatalf("uninstall: exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("brew-managed binary was removed by hand (err=%v)\nstdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, "brew uninstall") {
		t.Fatalf("uninstall did not point the operator at brew uninstall:\n%s", stdout)
	}
	receiptPath := filepath.Join(home, ".local", "state", "boundary", "binary.receipt")
	if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("binary receipt still present after uninstall (err=%v)", err)
	}
}

// TestInstallerBinaryReceiptBodyEndsWithANewline pins the root cause rather
// than only the symptom: the installer must not build a receipt body with
// command substitution, which strips the trailing newline off the final
// `path=` line. Asserting on the source keeps the two binary-install branches
// honest without needing brew or a release download to run.
func TestInstallerBinaryReceiptBodyEndsWithANewline(t *testing.T) {
	body := mustReadFile(t, installerPath(t))
	if strings.Contains(body, `write_receipt "$BINARY_RECEIPT" "$(printf`) {
		t.Fatal("the binary receipt body is built with command substitution, which strips its trailing newline")
	}
	if !strings.Contains(body, "read_receipt_line()") {
		t.Fatal("the uninstall receipt reader no longer handles a final unterminated line")
	}
	if strings.Contains(body, `while IFS='=' read -r key value; do`) {
		t.Fatal("an uninstall loop still reads receipt lines without the final-unterminated-line guard")
	}
}

// TestInstallerDefaultModeSkipsNetworkWhenGuarded pins the test-safety
// contract: with BOUNDARY_INSTALL_NO_NETWORK set, the default (binary
// install) mode must never reach brew or curl, and must not fabricate a
// receipt for an install that did not happen.
func TestInstallerDefaultModeSkipsNetworkWhenGuarded(t *testing.T) {
	home := t.TempDir()
	env := append(baseEnv(home), "BOUNDARY_INSTALL_NO_NETWORK=1")

	code, stdout, stderr := runInstaller(t, env)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "BOUNDARY_INSTALL_NO_NETWORK") {
		t.Fatalf("did not report that the network install was skipped:\n%s", stdout)
	}

	receiptPath := filepath.Join(home, ".local", "state", "boundary", "binary.receipt")
	if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("a binary receipt was written despite BOUNDARY_INSTALL_NO_NETWORK (err=%v)", err)
	}
}

// TestInstallerRejectsConflictingModes covers passing two mode flags at once:
// the installer should refuse rather than silently picking one.
func TestInstallerRejectsConflictingModes(t *testing.T) {
	code, stdout, stderr := runInstaller(t, baseEnv(t.TempDir()), "--plugin-drop", "--uninstall")
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for conflicting modes\nstdout=%s", stdout)
	}
	if !strings.Contains(stderr, "cannot combine") {
		t.Fatalf("stderr does not explain the conflict: %s", stderr)
	}
}

// TestInstallerRejectsUnknownArgument covers a typo'd flag: it should print
// usage and fail rather than falling through to the default mode.
func TestInstallerRejectsUnknownArgument(t *testing.T) {
	code, stdout, stderr := runInstaller(t, baseEnv(t.TempDir()), "--bogus")
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for an unknown argument\nstdout=%s", stdout)
	}
	if !strings.Contains(stderr, "unknown argument") {
		t.Fatalf("stderr does not explain the rejection: %s", stderr)
	}
}

// writeFakeBoundaryOnPath plants an executable named `boundary` in its own
// directory and returns an env whose PATH resolves it first. hookLane decides
// whether the fake supports `boundary hook --help` (a current build, exit 0)
// or predates the hook lane (a too-old build, exit 1 on the hook verb) — the
// same distinction the plugin wrapper's probe draws.
func writeFakeBoundaryOnPath(t *testing.T, home string, hookLane bool) []string {
	t.Helper()
	binDir := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", binDir, err)
	}
	body := "#!/bin/sh\nexit 0\n"
	if !hookLane {
		body = "#!/bin/sh\nif [ \"$1\" = \"hook\" ]; then exit 1; fi\nexit 0\n"
	}
	if err := os.WriteFile(filepath.Join(binDir, "boundary"), []byte(body), 0o755); err != nil {
		t.Fatalf("write fake boundary: %v", err)
	}
	return []string{
		"HOME=" + home,
		"PATH=" + binDir + ":" + systemPathWithoutBoundary(),
		// Belt over suspenders: the too-old refusal must fire before any
		// network path is reached, and if it ever does not, the guard keeps
		// the test off brew and curl instead of hitting the real network.
		"BOUNDARY_INSTALL_NO_NETWORK=1",
	}
}

// TestInstallerBinaryModeRefusesATooOldBoundaryOnPath is G3-A blocker 6: an
// existing pre-hook boundary must never produce "nothing to install" and then
// a too-old plugin failure. The installer must version-check what PATH
// resolves and stop with the exact supported upgrade guidance.
func TestInstallerBinaryModeRefusesATooOldBoundaryOnPath(t *testing.T) {
	home := t.TempDir()
	env := writeFakeBoundaryOnPath(t, home, false)

	code, stdout, stderr := runInstaller(t, env)
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for a too-old boundary on PATH\nstdout=%s", stdout)
	}
	if strings.Contains(stdout, "nothing to install") {
		t.Fatalf("installer still reports nothing to install over an incompatible binary:\n%s", stdout)
	}
	if !strings.Contains(stderr, "hook pretooluse") {
		t.Fatalf("stderr does not name the missing hook lane: %s", stderr)
	}
	// The stub does not self-report as Fulcrum Boundary, so the guidance must
	// stay product-neutral — never inferring ownership from the install path —
	// and offer BOUNDARY_BIN rather than guessing at brew.
	if !strings.Contains(stderr, "different product") || !strings.Contains(stderr, "BOUNDARY_BIN") {
		t.Fatalf("stderr does not carry the product-neutral resolution guidance: %s", stderr)
	}
}

// TestInstallerBinaryModeAcceptsACurrentBoundaryOnPath keeps the healthy early
// exit: a binary that carries the hook lane really is "nothing to install".
func TestInstallerBinaryModeAcceptsACurrentBoundaryOnPath(t *testing.T) {
	home := t.TempDir()
	env := writeFakeBoundaryOnPath(t, home, true)

	code, stdout, stderr := runInstaller(t, env)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "nothing to install") || !strings.Contains(stdout, "supports the Claude Code hook") {
		t.Fatalf("stdout does not report the probed early exit:\n%s", stdout)
	}
}

// TestInstallerPluginDropStopsBeforeWritingOnIncompatibleBoundary is the
// review-tightened contract for an incompatible binary: the compatibility
// preflight runs BEFORE any plugin file or receipt is written, the drop exits
// non-zero, and the fake HOME is left exactly as it was — no plugin
// directory, no receipt, nothing to reverse.
func TestInstallerPluginDropStopsBeforeWritingOnIncompatibleBoundary(t *testing.T) {
	home := t.TempDir()
	env := writeFakeBoundaryOnPath(t, home, false)

	code, stdout, stderr := runInstaller(t, env, "--plugin-drop")
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for an incompatible binary\nstdout=%s", stdout)
	}
	if !strings.Contains(stderr, "hook pretooluse") || !strings.Contains(stderr, "no receipt was written") {
		t.Fatalf("stderr does not explain the stop: %s", stderr)
	}
	pluginDir := filepath.Join(home, ".claude", "skills", "boundary")
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Fatalf("plugin directory was written despite the incompatible binary (err=%v)", err)
	}
	receiptPath := filepath.Join(home, ".local", "state", "boundary", "plugin-drop.receipt")
	if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("a receipt was written despite the incompatible binary (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("~/.claude was created despite the stop (err=%v)", err)
	}
}

// TestInstallerPluginDropPrintsTheReversalCommand is G3-A blocker 5's exit
// path: a successful local-file plugin drop must print a reversal command
// that names the actual script file — a command the user can paste and run.
func TestInstallerPluginDropPrintsTheReversalCommand(t *testing.T) {
	home := t.TempDir()
	code, stdout, stderr := runInstaller(t, baseEnv(home), "--plugin-drop")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr)
	}
	want := "To reverse this install: sh '" + installerPath(t) + "' --uninstall"
	if !strings.Contains(stdout, want) {
		t.Fatalf("plugin-drop does not print the executable reversal command %q:\n%s", want, stdout)
	}
}

// runInstallerPiped executes the installer the way the public launch path
// does — `curl ... | sh` — by streaming the script body over stdin, so $0 is
// the shell's own name and never a usable script path. No network involved.
func runInstallerPiped(t *testing.T, env []string, args ...string) (int, string, string) {
	t.Helper()
	body, err := os.ReadFile(installerPath(t))
	if err != nil {
		t.Fatalf("read installer: %v", err)
	}
	cmdArgs := append([]string{"-s", "--"}, args...)
	cmd := exec.Command("sh", cmdArgs...)
	cmd.Dir = t.TempDir()
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(body)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if runErr := cmd.Run(); runErr != nil {
		exitErr, ok := runErr.(*exec.ExitError)
		if !ok {
			t.Fatalf("run piped installer: %v\nstderr=%s", runErr, stderr.String())
		}
		code = exitErr.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

// writeFakeBrew plants a fake `brew` in binDir whose `install` subcommand
// drops an executable `boundary` stub into the same directory. It lets the
// binary lane's brew SUCCESS branch run end to end with zero network, which
// is what makes the reversal-line assertions functional rather than
// source-string counting.
func writeFakeBrew(t *testing.T, binDir string) {
	t.Helper()
	brew := "#!/bin/sh\n" +
		"if [ \"$1\" = \"install\" ]; then\n" +
		"  printf '#!/bin/sh\\nexit 0\\n' > " + binDir + "/boundary\n" +
		"  chmod 755 " + binDir + "/boundary\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "brew"), []byte(brew), 0o755); err != nil {
		t.Fatalf("write fake brew: %v", err)
	}
}

// brewEnv is the environment for the fake-brew success runs: isolated HOME, a
// PATH that resolves the fake brew (and later its installed stub) plus the
// system utilities, and NO network guard — the point is to traverse the real
// brew success branch, which the fake keeps entirely local.
func brewEnv(t *testing.T, home string) []string {
	t.Helper()
	binDir := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", binDir, err)
	}
	writeFakeBrew(t, binDir)
	return []string{
		"HOME=" + home,
		"PATH=" + binDir + ":" + systemPathWithoutBoundary(),
	}
}

// TestInstallerLocalBinaryInstallPrintsExecutableReversal drives the brew
// success branch from a local script file: the reversal command must name the
// script file itself.
func TestInstallerLocalBinaryInstallPrintsExecutableReversal(t *testing.T) {
	home := t.TempDir()
	code, stdout, stderr := runInstaller(t, brewEnv(t, home))
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	want := "To reverse this install: sh '" + installerPath(t) + "' --uninstall"
	if !strings.Contains(stdout, want) {
		t.Fatalf("local binary install does not print the executable reversal %q:\n%s", want, stdout)
	}
	mustExist(t, filepath.Join(home, ".local", "state", "boundary", "binary.receipt"))
}

// TestInstallerPipedBinaryInstallPrintsRePipeableReversal is the piped/stdin
// argv0 case the review named: under `curl ... | sh`, $0 is `sh`, so the
// reversal must be the documented re-pipe with `--uninstall` after `sh -s --`
// — and never a command built from $0.
func TestInstallerPipedBinaryInstallPrintsRePipeableReversal(t *testing.T) {
	home := t.TempDir()
	code, stdout, stderr := runInstallerPiped(t, brewEnv(t, home))
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	want := "To reverse this install: curl -fsSL https://raw.githubusercontent.com/fulcrum-governance/fulcrum-boundary/main/scripts/install-claude-code.sh | sh -s -- --uninstall"
	if !strings.Contains(stdout, want) {
		t.Fatalf("piped binary install does not print the re-pipeable reversal %q:\n%s", want, stdout)
	}
	if strings.Contains(stdout, "sh 'sh'") {
		t.Fatalf("piped install still builds a command from $0:\n%s", stdout)
	}
	mustExist(t, filepath.Join(home, ".local", "state", "boundary", "binary.receipt"))
}

// TestInstallerPipedUpgradeGuidanceUsesThePipeForm covers the remove-and-rerun
// guidance under a piped invocation: an incompatible binary refused during a
// `curl ... | sh` run must be pointed at the re-pipe, never at $0.
func TestInstallerPipedUpgradeGuidanceUsesThePipeForm(t *testing.T) {
	home := t.TempDir()
	code, stdout, stderr := runInstallerPiped(t, writeFakeBoundaryOnPath(t, home, false))
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for an incompatible binary\nstdout=%s", stdout)
	}
	if !strings.Contains(stderr, "curl -fsSL https://raw.githubusercontent.com/fulcrum-governance/fulcrum-boundary/main/scripts/install-claude-code.sh | sh") {
		t.Fatalf("piped refusal guidance does not use the re-pipe form: %s", stderr)
	}
	if strings.Contains(stderr, "sh 'sh'") {
		t.Fatalf("piped refusal still builds a command from $0: %s", stderr)
	}
}

// writeStubAt plants an executable boundary stub at an arbitrary absolute
// path, for BOUNDARY_BIN overrides. hookLane as in writeFakeBoundaryOnPath.
func writeStubAt(t *testing.T, path string, hookLane bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	body := "#!/bin/sh\nexit 0\n"
	if !hookLane {
		body = "#!/bin/sh\nif [ \"$1\" = \"hook\" ]; then exit 1; fi\nexit 0\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", path, err)
	}
}

// assertNothingDropped asserts the plugin-drop wrote neither plugin files nor
// a receipt into the fake HOME.
func assertNothingDropped(t *testing.T, home string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("~/.claude was created despite the stop (err=%v)", err)
	}
	receiptPath := filepath.Join(home, ".local", "state", "boundary", "plugin-drop.receipt")
	if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("a receipt was written despite the stop (err=%v)", err)
	}
}

// TestInstallerPluginDropPrefersACompatibleBoundaryBin is the wrapper-parity
// control: the shipped hook executes ${BOUNDARY_BIN:-boundary}, so a
// compatible absolute BOUNDARY_BIN must be what the preflight validates —
// and the drop must succeed even when an incompatible unrelated binary sits
// on PATH.
func TestInstallerPluginDropPrefersACompatibleBoundaryBin(t *testing.T) {
	home := t.TempDir()
	env := writeFakeBoundaryOnPath(t, home, false) // incompatible on PATH
	override := filepath.Join(home, "override", "boundary")
	writeStubAt(t, override, true)
	env = append(env, "BOUNDARY_BIN="+override)

	code, stdout, stderr := runInstaller(t, env, "--plugin-drop")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	mustExist(t, filepath.Join(home, ".claude", "skills", "boundary", "hooks", "hooks.json"))
	mustExist(t, filepath.Join(home, ".local", "state", "boundary", "plugin-drop.receipt"))
	if !strings.Contains(stdout, "BOUNDARY_BIN ("+override+")") {
		t.Fatalf("stdout does not name the validated effective binary:\n%s", stdout)
	}
	if !strings.Contains(stdout, "LAUNCHES Claude Code") {
		t.Fatalf("stdout does not state where BOUNDARY_BIN must be present:\n%s", stdout)
	}
}

// TestInstallerPluginDropStopsOnIncompatibleBoundaryBinOverride: an explicit
// override is what the wrapper will execute, so a compatible PATH binary must
// not rescue an incompatible BOUNDARY_BIN — stop, write nothing.
func TestInstallerPluginDropStopsOnIncompatibleBoundaryBinOverride(t *testing.T) {
	home := t.TempDir()
	env := writeFakeBoundaryOnPath(t, home, true) // compatible on PATH
	override := filepath.Join(home, "override", "boundary")
	writeStubAt(t, override, false)
	env = append(env, "BOUNDARY_BIN="+override)

	code, stdout, stderr := runInstaller(t, env, "--plugin-drop")
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for an incompatible BOUNDARY_BIN\nstdout=%s", stdout)
	}
	if !strings.Contains(stderr, "BOUNDARY_BIN: "+override) || !strings.Contains(stderr, "no receipt was written") {
		t.Fatalf("stderr does not name the incompatible effective binary: %s", stderr)
	}
	assertNothingDropped(t, home)
}

// TestInstallerPluginDropStopsOnMissingBoundaryBinOverride: an override that
// resolves to nothing would make the wrapper fail against it rather than
// fall back, so the drop stops before writing anything.
func TestInstallerPluginDropStopsOnMissingBoundaryBinOverride(t *testing.T) {
	home := t.TempDir()
	env := writeFakeBoundaryOnPath(t, home, true) // compatible on PATH
	env = append(env, "BOUNDARY_BIN="+filepath.Join(home, "missing", "boundary"))

	code, stdout, stderr := runInstaller(t, env, "--plugin-drop")
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for a missing BOUNDARY_BIN\nstdout=%s", stdout)
	}
	if !strings.Contains(stderr, "not an executable command") || !strings.Contains(stderr, "no") {
		t.Fatalf("stderr does not explain the unresolvable override: %s", stderr)
	}
	assertNothingDropped(t, home)
}

// TestInstallerPluginDropSucceedsWithOnlyBoundaryBin: no PATH binary at all,
// compatible absolute BOUNDARY_BIN — the effective binary exists, so the
// drop proceeds and names it.
func TestInstallerPluginDropSucceedsWithOnlyBoundaryBin(t *testing.T) {
	home := t.TempDir()
	override := filepath.Join(home, "override", "boundary")
	writeStubAt(t, override, true)
	env := append(baseEnv(home), "BOUNDARY_BIN="+override)

	code, stdout, stderr := runInstaller(t, env, "--plugin-drop")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	mustExist(t, filepath.Join(home, ".claude", "skills", "boundary", "hooks", "hooks.json"))
	if !strings.Contains(stdout, "BOUNDARY_BIN ("+override+")") {
		t.Fatalf("stdout does not name the validated effective binary:\n%s", stdout)
	}
}

// TestInstallerPipedNothingToInstallHasNoArgvZero is the round-3 piped
// control for the compatible-existing-binary early exit: with no receipt the
// message must say this script does not manage the binary, and no command
// built from $0 may appear.
func TestInstallerPipedNothingToInstallHasNoArgvZero(t *testing.T) {
	home := t.TempDir()
	env := writeFakeBoundaryOnPath(t, home, true)

	code, stdout, stderr := runInstallerPiped(t, env)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "not installed or recorded by this script") {
		t.Fatalf("early exit is not receipt-aware:\n%s", stdout)
	}
	if strings.Contains(stdout, "sh 'sh'") || strings.Contains(stdout, "$0") {
		t.Fatalf("piped early exit still leaks an argv0-built command:\n%s", stdout)
	}
}

// TestInstallerPipedNothingToInstallPrintsRePipeableReversalWhenRecorded: the
// same early exit, but the binary IS the one this script's receipt records —
// so the reversal is printed, in the piped-safe re-pipe form.
func TestInstallerPipedNothingToInstallPrintsRePipeableReversalWhenRecorded(t *testing.T) {
	home := t.TempDir()
	installed := writeBinaryReceipt(t, home, "release-download", "\n")
	env := []string{
		"HOME=" + home,
		"PATH=" + filepath.Dir(installed) + ":" + systemPathWithoutBoundary(),
		"BOUNDARY_INSTALL_NO_NETWORK=1",
	}

	code, stdout, stderr := runInstallerPiped(t, env)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "This script's receipt records that install") {
		t.Fatalf("early exit did not recognize its own receipt:\n%s", stdout)
	}
	want := "curl -fsSL https://raw.githubusercontent.com/fulcrum-governance/fulcrum-boundary/main/scripts/install-claude-code.sh | sh -s -- --uninstall"
	if !strings.Contains(stdout, want) {
		t.Fatalf("recorded-install early exit does not print the re-pipeable reversal:\n%s", stdout)
	}
}

// writeFakeCurl plants an executable `curl` that serves the installer's
// direct-release lane entirely from local fixtures: the latest-release
// resolution (`-o /dev/null -w '%{url_effective}'`) prints latestURL, and
// every other `-o <target> <url>` download copies the fixture file named by
// the URL's basename. No network is reachable through it.
func writeFakeCurl(t *testing.T, binDir, fixtureDir, latestURL string) {
	t.Helper()
	script := `#!/bin/sh
out=""
want_effective=0
prev=""
for a in "$@"; do
	case "$prev" in
	-o) out=$a ;;
	-w) want_effective=1 ;;
	esac
	prev=$a
done
url=""
for a in "$@"; do url=$a; done
if [ "$want_effective" -eq 1 ]; then
	printf '%s' "` + latestURL + `"
	exit 0
fi
exec cp "` + fixtureDir + `/$(basename "$url")" "$out"
`
	if err := os.WriteFile(filepath.Join(binDir, "curl"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake curl: %v", err)
	}
}

// stageDirectRelease builds the local fixtures a direct-release install needs:
// a real tar.gz archive whose root contains a fake `boundary` binary, named
// exactly as the installer will request it for this host, plus that archive's
// true SHA-256. The manifest itself is written per test case.
func stageDirectRelease(t *testing.T, fixtureDir, version string) (asset, archiveSum string) {
	t.Helper()
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if (goos != "darwin" && goos != "linux") || (goarch != "amd64" && goarch != "arm64") {
		t.Skipf("direct-release lane supports darwin/linux amd64/arm64; host is %s/%s", goos, goarch)
	}
	content := t.TempDir()
	if err := os.WriteFile(filepath.Join(content, "boundary"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake boundary payload: %v", err)
	}
	asset = "boundary_" + version + "_" + goos + "_" + goarch + "_static-nocgo.tar.gz"
	archivePath := filepath.Join(fixtureDir, asset)
	if out, err := exec.Command("tar", "-czf", archivePath, "-C", content, "boundary").CombinedOutput(); err != nil {
		t.Fatalf("build fixture archive: %v\n%s", err, out)
	}
	body, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read fixture archive: %v", err)
	}
	sum := sha256.Sum256(body)
	return asset, hex.EncodeToString(sum[:])
}

// directReleaseEnv is the hermetic environment for a default-mode run that
// must take the direct-release lane: an isolated HOME, and a PATH whose first
// entry holds only the fake curl — so `brew` cannot resolve, forcing the
// no-Homebrew fallback, while every ordinary utility (sh, uname, tar, awk,
// shasum/sha256sum) still comes from the system directories.
func directReleaseEnv(t *testing.T, home, fakeBinDir string) []string {
	t.Helper()
	return []string{
		"HOME=" + home,
		"PATH=" + fakeBinDir + ":" + systemPathWithoutBoundary(),
	}
}

// TestInstallerDirectReleaseInstallsDespiteSBOMPrefixCollision drives the
// actual download/verify/extract/install/receipt path against the manifest
// shape the published v0.13.0 release actually has — every archive entry
// followed by its `<archive>.spdx.json` entry, whose filename contains the
// archive's as a prefix. The shipped v0.13.0 installer selected both lines
// with an unanchored fixed-string grep and then failed closed on the
// never-downloaded SBOM; this pins the exact-field selection fix, the
// resulting receipt, and the receipt-driven uninstall round trip.
func TestInstallerDirectReleaseInstallsDespiteSBOMPrefixCollision(t *testing.T) {
	home := t.TempDir()
	fixtures := t.TempDir()
	fakeBin := t.TempDir()
	asset, sum := stageDirectRelease(t, fixtures, "9.9.9")
	manifest := sum + "  " + asset + "\n" +
		strings.Repeat("1", 64) + "  " + asset + ".spdx.json\n"
	if err := os.WriteFile(filepath.Join(fixtures, "SHA256SUMS"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}
	writeFakeCurl(t, fakeBin, fixtures, "https://github.com/fulcrum-governance/fulcrum-boundary/releases/tag/v9.9.9")
	env := directReleaseEnv(t, home, fakeBin)

	code, stdout, stderr := runInstaller(t, env)
	if code != 0 {
		t.Fatalf("direct-release install: exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Checksum OK.") {
		t.Fatalf("install did not report checksum success:\n%s", stdout)
	}
	installed := filepath.Join(home, ".local", "bin", "boundary")
	mustExist(t, installed)
	receiptPath := filepath.Join(home, ".local", "state", "boundary", "binary.receipt")
	receipt := mustReadFile(t, receiptPath)
	for _, want := range []string{"mode=binary\n", "method=release-download\n", "version=v9.9.9\n", "path=" + installed + "\n"} {
		if !strings.Contains(receipt, want) {
			t.Fatalf("release-download receipt is missing %q:\n%s", want, receipt)
		}
	}

	code2, stdout2, stderr2 := runInstaller(t, env, "--uninstall")
	if code2 != 0 {
		t.Fatalf("uninstall after direct-release install: exit = %d, want 0\nstdout=%s\nstderr=%s", code2, stdout2, stderr2)
	}
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatalf("installed binary still present after uninstall (err=%v)", err)
	}
	if _, err := os.Stat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("binary receipt still present after uninstall (err=%v)", err)
	}
}

// TestInstallerDirectReleaseFailsClosedWithoutExactManifestEntry pins the
// zero-match side of exact-field selection: a manifest that carries only the
// SBOM entry — a filename the archive's name merely prefixes — must not
// satisfy verification, and nothing may be installed or receipted.
func TestInstallerDirectReleaseFailsClosedWithoutExactManifestEntry(t *testing.T) {
	home := t.TempDir()
	fixtures := t.TempDir()
	fakeBin := t.TempDir()
	asset, _ := stageDirectRelease(t, fixtures, "9.9.9")
	manifest := strings.Repeat("1", 64) + "  " + asset + ".spdx.json\n"
	if err := os.WriteFile(filepath.Join(fixtures, "SHA256SUMS"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}
	writeFakeCurl(t, fakeBin, fixtures, "https://github.com/fulcrum-governance/fulcrum-boundary/releases/tag/v9.9.9")
	env := directReleaseEnv(t, home, fakeBin)

	code, stdout, stderr := runInstaller(t, env)
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for a manifest with no exact entry\nstdout=%s", stdout)
	}
	if !strings.Contains(stderr, "no SHA256SUMS entry for "+asset) {
		t.Fatalf("stderr does not explain the missing exact entry: %s", stderr)
	}
	assertNothingInstalled(t, home)
}

// TestInstallerDirectReleaseFailsClosedOnDuplicateManifestEntries pins the
// many-match side: two exact entries for the same archive are an ambiguous
// manifest, and the installer must refuse rather than pick one, with nothing
// installed or receipted.
func TestInstallerDirectReleaseFailsClosedOnDuplicateManifestEntries(t *testing.T) {
	home := t.TempDir()
	fixtures := t.TempDir()
	fakeBin := t.TempDir()
	asset, sum := stageDirectRelease(t, fixtures, "9.9.9")
	manifest := sum + "  " + asset + "\n" +
		strings.Repeat("2", 64) + "  " + asset + "\n" +
		strings.Repeat("1", 64) + "  " + asset + ".spdx.json\n"
	if err := os.WriteFile(filepath.Join(fixtures, "SHA256SUMS"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest fixture: %v", err)
	}
	writeFakeCurl(t, fakeBin, fixtures, "https://github.com/fulcrum-governance/fulcrum-boundary/releases/tag/v9.9.9")
	env := directReleaseEnv(t, home, fakeBin)

	code, stdout, stderr := runInstaller(t, env)
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero for duplicate exact manifest entries\nstdout=%s", stdout)
	}
	if !strings.Contains(stderr, "SHA256SUMS entries for "+asset) {
		t.Fatalf("stderr does not explain the ambiguous manifest: %s", stderr)
	}
	assertNothingInstalled(t, home)
}

// assertNothingInstalled proves a failed direct-release run wrote neither the
// binary nor a receipt into the isolated HOME.
func assertNothingInstalled(t *testing.T, home string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "boundary")); !os.IsNotExist(err) {
		t.Fatalf("a binary was installed by a failed run (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "state", "boundary", "binary.receipt")); !os.IsNotExist(err) {
		t.Fatalf("a receipt was written by a failed run (err=%v)", err)
	}
}
