package hookboundary_test

import (
	"bytes"
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
// The binary lane cannot be driven end to end in a test — installing it means
// brew or a release download, and BOUNDARY_INSTALL_NO_NETWORK deliberately
// writes no receipt — so the receipt is written here in exactly the shape the
// installer writes it. terminator lets a caller choose whether the final
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
