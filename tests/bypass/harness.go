//go:build bypass

// Package bypass runs deployment-topology bypass-resistance probes.
//
// Each probe executes inside a docker compose topology as a fixed unprivileged
// user and is judged on real outcomes: exit codes, stdout/stderr content, and
// the decision records Boundary writes. A topology supplies a compose file and
// a probe list; adding a new reference topology is a new compose directory plus
// a new test, with no change to this file.
//
// Docker is required. Without Docker the tests skip locally and fail when
// CI=true or REQUIRE_DOCKER=1.
package bypass

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ExecResult records one probe invocation inside the topology: the exact argv
// passed to docker compose exec, the process exit code, and captured output.
// docker exec forwards the in-container exit code, so a non-zero ExitCode is
// the command's own result, not a harness error.
type ExecResult struct {
	Argv     []string `json:"argv"`
	ExitCode int      `json:"exit_code"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
}

// Harness drives one docker compose topology. Dir holds the compose file,
// Service is the compose service probes exec into, and User is the
// unprivileged container user every probe runs as.
type Harness struct {
	Dir     string
	Service string
	User    string
}

// RequireDocker skips the test when Docker is unusable, unless running under
// CI=true or REQUIRE_DOCKER=1, in which case it fails: a skipped bypass suite
// in CI is a missing gate, not a green one.
func RequireDocker(t *testing.T) {
	t.Helper()
	_, err := exec.LookPath("docker")
	if err == nil {
		err = exec.Command("docker", "info").Run()
	}
	if err == nil {
		return
	}
	if os.Getenv("CI") == "true" || os.Getenv("REQUIRE_DOCKER") == "1" {
		t.Fatalf("docker is required for bypass probes but is unavailable: %v", err)
	}
	t.Skipf("docker unavailable (%v); skipping bypass probes (set REQUIRE_DOCKER=1 to fail instead)", err)
}

// Up builds and starts the topology. BOUNDARY_COMMIT is forwarded as a
// compose build argument so the image stamps the boundary binary with the
// commit under test when the caller supplies one.
func (h Harness) Up(t *testing.T, env ...string) {
	t.Helper()
	cmd := h.compose("up", "-d", "--build")
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose up failed: %v\nOutput:\n%s", err, out)
	}
	h.waitReady(t)
}

// Down tears the topology down and removes volumes.
func (h Harness) Down(t *testing.T) {
	t.Helper()
	cmd := h.compose("down", "-v", "--remove-orphans")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("docker compose down failed: %v\nOutput:\n%s", err, out)
	}
}

// Exec runs argv inside the topology as the configured user and captures the
// result. A transport failure (docker itself cannot run) fails the test; an
// in-container non-zero exit is reported in ExecResult.ExitCode.
func (h Harness) Exec(t *testing.T, argv ...string) ExecResult {
	t.Helper()
	args := append([]string{"exec", "-T", "-u", h.User, h.Service}, argv...)
	cmd := h.compose(args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := ExecResult{Argv: argv, Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result
	}
	t.Fatalf("docker compose exec failed to run (argv=%q): %v", strings.Join(argv, " "), err)
	return result
}

// ImageDigests returns the image IDs the topology was built from, so evidence
// binds the probe outcomes to exact image content.
func (h Harness) ImageDigests(t *testing.T) []string {
	t.Helper()
	cmd := h.compose("images", "-q")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("docker compose images failed: %v", err)
	}
	var digests []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			digests = append(digests, line)
		}
	}
	return digests
}

// BoundaryVersion returns the raw `boundary version` output from inside the
// topology, recording which binary the probes ran against.
func (h Harness) BoundaryVersion(t *testing.T) string {
	t.Helper()
	res := h.Exec(t, "boundary", "version")
	if res.ExitCode != 0 {
		t.Fatalf("boundary version inside topology failed: exit=%d stderr=%s", res.ExitCode, res.Stderr)
	}
	return strings.TrimSpace(res.Stdout)
}

func (h Harness) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		cmd := h.compose("exec", "-T", "-u", h.User, h.Service, "true")
		if err := cmd.Run(); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("topology did not become exec-ready within 60s")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (h Harness) compose(args ...string) *exec.Cmd {
	dir, err := filepath.Abs(h.Dir)
	if err != nil {
		dir = h.Dir
	}
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = dir
	return cmd
}
