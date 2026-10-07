package codeexec

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestNewSandbox_RejectsLocalTypeInProduction(t *testing.T) {
	_, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeLocal,
		Production: true,
	})
	if !errors.Is(err, ErrLocalSandboxNonProduction) {
		t.Fatalf("expected ErrLocalSandboxNonProduction, got %v", err)
	}
}

func TestNewSandbox_AllowsLocalTypeOutsideProduction(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{Type: SandboxTypeLocal})
	if err != nil {
		t.Fatalf("NewSandbox local: %v", err)
	}
	b := s.Boundary()
	if b.SecureSandbox {
		t.Fatal("local sandbox boundary must not claim a secure sandbox")
	}
	if b.Kind != "local_process" {
		t.Fatalf("boundary kind = %q, want local_process", b.Kind)
	}
}

func TestNewSandbox_RejectsUnknownType(t *testing.T) {
	_, err := NewSandbox(SandboxConfig{Type: SandboxType("wasm")})
	if err == nil || !strings.Contains(err.Error(), "unknown sandbox type") {
		t.Fatalf("expected unknown sandbox type error, got %v", err)
	}
}

func TestNewSandbox_ContainerRequiresImage(t *testing.T) {
	_, err := NewSandbox(SandboxConfig{Type: SandboxTypeContainer, Production: true})
	if !errors.Is(err, ErrSandboxImageRequired) {
		t.Fatalf("expected ErrSandboxImageRequired, got %v", err)
	}
}

func TestContainerSandbox_BoundaryMetadata(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}
	b := s.Boundary()
	if b.Kind != "container" {
		t.Fatalf("boundary kind = %q, want container", b.Kind)
	}
	if !b.SecureSandbox {
		t.Fatal("container boundary must report a named, tested sandbox boundary")
	}
	if b.Name == "" || b.Description == "" {
		t.Fatalf("container boundary must be named and described, got %+v", b)
	}
}

func TestContainerSandbox_MissingRuntimeFailsClosedIndeterminate(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
		Runtime:    "fulcrum-definitely-missing-runtime",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}

	resp, err := s.Execute(context.Background(), &governance.GovernanceRequest{
		RequestID: "req-runtime-missing",
		Language:  "python",
		Code:      "print('never runs')",
		TenantID:  "tenant-1",
		AgentID:   "agent-1",
		Transport: governance.TransportCodeExec,
	})
	if err != nil {
		t.Fatalf("Execute returned error instead of fail-closed deny envelope: %v", err)
	}
	assertIndeterminateDeny(t, resp, "sandbox_runtime_unavailable")
	if resp.Metadata["x-fulcrum-request-id"] != "req-runtime-missing" {
		t.Fatalf("request id missing from deny envelope: %+v", resp.Metadata)
	}
	if resp.Metadata["x-fulcrum-tenant-id"] != "tenant-1" {
		t.Fatalf("tenant id missing from deny envelope: %+v", resp.Metadata)
	}
	if resp.Metadata["x-fulcrum-agent-id"] != "agent-1" {
		t.Fatalf("agent id missing from deny envelope: %+v", resp.Metadata)
	}
	if resp.Metadata["x-fulcrum-transport"] != string(governance.TransportCodeExec) {
		t.Fatalf("transport missing from deny envelope: %+v", resp.Metadata)
	}
}

func TestContainerSandbox_NilRequestFailsClosed(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
		Runtime:    "fulcrum-definitely-missing-runtime",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}
	resp, err := s.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute returned error instead of fail-closed deny envelope: %v", err)
	}
	assertIndeterminateDeny(t, resp, "sandbox_config")
}

func TestContainerSandbox_CallerCancelFailsClosedNotTimeout(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
		// "echo" resolves via LookPath on every host; the canceled context
		// means the run CLI never executes, so the binary itself is
		// irrelevant — the point under test is the cancel classification.
		Runtime: "echo",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := s.Execute(ctx, &governance.GovernanceRequest{
		RequestID: "req-cancel",
		Language:  "python",
		Code:      "print('never runs')",
	})
	if err != nil {
		t.Fatalf("Execute returned error instead of fail-closed deny envelope: %v", err)
	}
	assertIndeterminateDeny(t, resp, "sandbox_canceled")
	if resp.Metadata["timeout"] == "true" {
		t.Fatalf("caller cancel must not be reported as a timeout: %+v", resp.Metadata)
	}
}

func TestContainerRunArgs_NeverPullsAndWritesCidfile(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}
	cs, ok := s.(*containerSandbox)
	if !ok {
		t.Fatalf("expected *containerSandbox, got %T", s)
	}
	args := cs.containerRunArgs("fulcrum-codeexec-test", "/tmp/fulcrum-codeexec-test.cid", []string{"python3", "-c", "print(1)"})
	if got := flagValue(args, "--pull"); got != "never" {
		t.Fatalf("--pull = %q, want never (execution must not pull images)", got)
	}
	if got := flagValue(args, "--cidfile"); got != "/tmp/fulcrum-codeexec-test.cid" {
		t.Fatalf("--cidfile = %q, want the per-execution cidfile path", got)
	}
}

// flagValue returns the value following the first occurrence of flag.
func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// hasAnyFlag reports whether args contains any of the given exact flags.
func hasAnyFlag(args []string, flags ...string) bool {
	for _, a := range args {
		for _, f := range flags {
			if a == f {
				return true
			}
		}
	}
	return false
}

func indexOf(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}

// --entrypoint must pin argv[0] to the interpreter: with an image-defined
// ENTRYPOINT, argv appended after the image name is passed to the image's
// entrypoint instead of the interpreter. The arg tail must be exactly
// --entrypoint <interp> <image> <interp args...>.
func TestContainerRunArgs_PinsEntrypointToInterpreter(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}
	cs := s.(*containerSandbox)
	args := cs.containerRunArgs("fulcrum-codeexec-test", "/tmp/fulcrum-codeexec-test.cid",
		[]string{"python3", "-c", "print(1)"})
	if got := flagValue(args, "--entrypoint"); got != "python3" {
		t.Fatalf("--entrypoint = %q, want python3 (the interpreter must own argv[0])", got)
	}
	tail := args[len(args)-5:]
	want := []string{"--entrypoint", "python3", "python:3.11-slim", "-c", "print(1)"}
	for i := range want {
		if tail[i] != want[i] {
			t.Fatalf("arg tail = %v, want %v", tail, want)
		}
	}
	if indexOf(args, "--entrypoint") > indexOf(args, "python:3.11-slim") {
		t.Fatalf("--entrypoint must precede the image reference: %v", args)
	}
}

// The input mount must be a --mount type=bind spec. The adapter must never
// emit -v, --volume, or --volumes-from.
func TestContainerRunArgs_InputMountIsValidatedBindOnly(t *testing.T) {
	hostDir := t.TempDir()
	s, err := NewSandbox(SandboxConfig{
		Type:         SandboxTypeContainer,
		Production:   true,
		Image:        "python:3.11-slim",
		HostInputDir: hostDir,
		MountDestDir: "/input",
	})
	if err != nil {
		t.Fatalf("NewSandbox container with valid mount: %v", err)
	}
	cs := s.(*containerSandbox)
	args := cs.containerRunArgs("fulcrum-codeexec-test", "/tmp/fulcrum-codeexec-test.cid",
		[]string{"python3", "-c", "print(1)"})
	want := "type=bind,src=" + hostDir + ",dst=/input,readonly"
	if got := flagValue(args, "--mount"); got != want {
		t.Fatalf("--mount = %q, want %q", got, want)
	}
	if hasAnyFlag(args, "-v", "--volume", "--volumes-from") {
		t.Fatalf("args must not contain -v/--volume/--volumes-from: %v", args)
	}
}

// A mount destination that shadows /tmp, a system tree, or the interpreter
// directories can override sandbox controls. Both fields are required
// together, absolute, clean, and free of ':' , ',' and control characters —
// otherwise a --mount/--volume spec splits into extra fields or the source
// silently becomes a named volume.
func TestNewSandbox_MountConfigRejected(t *testing.T) {
	hostDir := t.TempDir()
	tests := []struct {
		name string
		host string
		dest string
	}{
		{"host relative", "data/input", "/input"},
		{"host contains colon", hostDir + ":extra", "/input"},
		{"host contains comma", hostDir + ",extra", "/input"},
		{"host contains control char", hostDir + "\t", "/input"},
		{"host not clean", hostDir + "/sub/..", "/input"},
		{"dest relative", hostDir, "input"},
		{"dest not clean", hostDir, "/data/../input"},
		{"dest is filesystem root", hostDir, "/"},
		{"dest equals protected /etc", hostDir, "/etc"},
		{"dest inside protected /etc", hostDir, "/etc/inputs"},
		{"dest inside protected /tmp", hostDir, "/tmp/inputs"},
		{"dest equals protected /tmp", hostDir, "/tmp"},
		{"dest inside interpreter tree /usr", hostDir, "/usr/local/bin"},
		{"dest equals protected /bin", hostDir, "/bin"},
		{"dest inside protected /proc", hostDir, "/proc/inputs"},
		{"dest inside protected /sys", hostDir, "/sys/inputs"},
		{"dest inside protected /dev", hostDir, "/dev/inputs"},
		{"dest inside protected /opt", hostDir, "/opt/inputs"},
		{"dest contains colon", hostDir, "/input:extra"},
		{"dest contains comma", hostDir, "/input,extra"},
		{"dest contains control char", hostDir, "/input\n"},
		{"dest set without host", "", "/input"},
		{"host set without dest", hostDir, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSandbox(SandboxConfig{
				Type:         SandboxTypeContainer,
				Production:   true,
				Image:        "python:3.11-slim",
				HostInputDir: tt.host,
				MountDestDir: tt.dest,
			})
			if !errors.Is(err, ErrSandboxMountConfig) {
				t.Fatalf("expected ErrSandboxMountConfig, got %v", err)
			}
		})
	}
}

func TestNewSandbox_MountConfigAccepted(t *testing.T) {
	for _, dest := range []string{"/input", "/data", "/data/inputs", "/mnt/inputs"} {
		_, err := NewSandbox(SandboxConfig{
			Type:         SandboxTypeContainer,
			Production:   true,
			Image:        "python:3.11-slim",
			HostInputDir: t.TempDir(),
			MountDestDir: dest,
		})
		if err != nil {
			t.Fatalf("MountDestDir %q rejected: %v", dest, err)
		}
	}
}

// Concurrent executions sharing a RequestID (retries, resumed runs) must get
// distinct container names; an identical name would let one execution's
// cleanup force-remove the other's live container. The validated RequestID
// stays a prefix for audit lookup.
func TestSandboxContainerName_UniquePerExecution(t *testing.T) {
	req := &governance.GovernanceRequest{RequestID: "req-shared"}
	a := sandboxContainerName(req)
	b := sandboxContainerName(req)
	if a == b {
		t.Fatalf("same RequestID produced identical container name %q: concurrent executions would collide", a)
	}
	for _, n := range []string{a, b} {
		if !strings.HasPrefix(n, "fulcrum-codeexec-req-shared-") {
			t.Fatalf("name %q lost the request-id audit prefix", n)
		}
	}
}

// TypeScript has no runtime contract in the sandbox: `node -e` does not
// transpile TS, so the mapping is absent and a TS request fails closed.
func TestInterpreterArgs_TypeScriptFailsClosed(t *testing.T) {
	for _, lang := range []string{"typescript", "TypeScript"} {
		if _, err := interpreterArgs(lang, "const x: number = 1"); err == nil {
			t.Fatalf("interpreterArgs(%q) must fail closed: no TS runtime contract exists", lang)
		}
	}
}

// A TypeScript execution request denies deterministically — the language
// check precedes runtime resolution, so the outcome does not depend on a
// container runtime being present.
func TestContainerSandbox_TypeScriptDeniedDeterministic(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}
	resp, err := s.Execute(context.Background(), &governance.GovernanceRequest{
		RequestID: "req-ts",
		Language:  "typescript",
		Code:      "const x: number = 1",
	})
	if err != nil {
		t.Fatalf("Execute returned error instead of fail-closed deny envelope: %v", err)
	}
	assertIndeterminateDeny(t, resp, "sandbox_config")
}

func TestContainerSandbox_UnsupportedLanguageFailsClosed(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
		Runtime:    "fulcrum-definitely-missing-runtime",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}
	resp, err := s.Execute(context.Background(), &governance.GovernanceRequest{
		RequestID: "req-lang",
		Language:  "ruby",
		Code:      "puts 1",
	})
	if err != nil {
		t.Fatalf("Execute returned error instead of fail-closed deny envelope: %v", err)
	}
	assertIndeterminateDeny(t, resp, "sandbox_config")
}

func TestForwardGoverned_ExecutorFailClosedEnvelopeStaysDeny(t *testing.T) {
	s, err := NewSandbox(SandboxConfig{
		Type:       SandboxTypeContainer,
		Production: true,
		Image:      "python:3.11-slim",
		Runtime:    "fulcrum-definitely-missing-runtime",
	})
	if err != nil {
		t.Fatalf("NewSandbox container: %v", err)
	}
	a := NewAdapterWithExecutor("tenant-1", s, s.Boundary())
	req, err := a.ParseRequest(context.Background(), &CodeExecInput{
		Code:     "print('never runs')",
		Language: "python",
	})
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	resp, err := a.ForwardGoverned(context.Background(), req, &governance.GovernanceDecision{
		Action:    "allow",
		RequestID: req.RequestID,
	})
	if err != nil {
		t.Fatalf("ForwardGoverned: %v", err)
	}
	if resp.ExitCode != 126 || resp.Metadata["codeexec_denied"] != "true" {
		t.Fatalf("expected fail-closed deny envelope, got %+v", resp)
	}
	if resp.Metadata["x-fulcrum-action"] != "deny" {
		t.Fatalf("fail-closed executor response must not be relabelled with the allow decision, got %q", resp.Metadata["x-fulcrum-action"])
	}
	if resp.Metadata["check_result"] != "CHECK_INDETERMINATE" {
		t.Fatalf("expected CHECK_INDETERMINATE classification, got %+v", resp.Metadata)
	}
	if resp.Metadata["codeexec_boundary_kind"] != "container" {
		t.Fatalf("boundary metadata missing on deny envelope: %+v", resp.Metadata)
	}
}

func TestLocalSandbox_ExecutesAndCapturesOutput(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available on this host")
	}
	s, err := NewSandbox(SandboxConfig{Type: SandboxTypeLocal, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("NewSandbox local: %v", err)
	}
	resp, err := s.Execute(context.Background(), &governance.GovernanceRequest{
		RequestID: "req-local",
		Language:  "python",
		Code:      "print('local-sandbox-ok')",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.ExitCode != 0 || !strings.Contains(string(resp.Content), "local-sandbox-ok") {
		t.Fatalf("expected executed output, got %+v", resp)
	}
	if resp.Metadata["timeout"] != "false" {
		t.Fatalf("unexpected timeout flag: %+v", resp.Metadata)
	}
}

func TestLocalSandbox_TimeoutKillsProcess(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available on this host")
	}
	s, err := NewSandbox(SandboxConfig{Type: SandboxTypeLocal, Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewSandbox local: %v", err)
	}
	resp, err := s.Execute(context.Background(), &governance.GovernanceRequest{
		Language: "python",
		Code:     "import time\ntime.sleep(30)",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.Metadata["timeout"] != "true" {
		t.Fatalf("expected timeout=true, got %+v", resp.Metadata)
	}
	if resp.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code on timeout, got %+v", resp)
	}
}

func TestLocalSandbox_CallerCancelFailsClosedNotTimeout(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available on this host")
	}
	s, err := NewSandbox(SandboxConfig{Type: SandboxTypeLocal, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("NewSandbox local: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := s.Execute(ctx, &governance.GovernanceRequest{
		Language: "python",
		Code:     "print('never runs')",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	assertIndeterminateDeny(t, resp, "sandbox_canceled")
	if resp.Metadata["timeout"] == "true" {
		t.Fatalf("caller cancel must not be reported as a timeout: %+v", resp.Metadata)
	}
}

func assertIndeterminateDeny(t *testing.T, resp *governance.ToolResponse, wantCategory string) {
	t.Helper()
	if resp == nil {
		t.Fatal("expected a deny envelope, got nil response")
	}
	if resp.ExitCode != 126 {
		t.Fatalf("exit code = %d, want 126 (execution blocked)", resp.ExitCode)
	}
	if resp.Metadata["x-fulcrum-action"] != "deny" {
		t.Fatalf("x-fulcrum-action = %q, want deny", resp.Metadata["x-fulcrum-action"])
	}
	if resp.Metadata["codeexec_denied"] != "true" {
		t.Fatalf("codeexec_denied marker missing: %+v", resp.Metadata)
	}
	if resp.Metadata["check_result"] != "CHECK_INDETERMINATE" {
		t.Fatalf("check_result = %q, want CHECK_INDETERMINATE", resp.Metadata["check_result"])
	}
	if resp.Metadata["check_class"] != "sandbox_executor" {
		t.Fatalf("check_class = %q, want sandbox_executor", resp.Metadata["check_class"])
	}
	if resp.Metadata["failure_category"] != wantCategory {
		t.Fatalf("failure_category = %q, want %q", resp.Metadata["failure_category"], wantCategory)
	}
	if resp.Metadata["enforcement_stage"] != "execution" {
		t.Fatalf("enforcement_stage = %q, want execution", resp.Metadata["enforcement_stage"])
	}
	if !strings.Contains(string(resp.Content), "code execution blocked") {
		t.Fatalf("deny content missing, got %q", string(resp.Content))
	}
}

func TestDefaultSandboxPolicy(t *testing.T) {
	p := DefaultSandboxPolicy()

	if p.MaxOutputSize != 50*1024 {
		t.Errorf("MaxOutputSize = %d, want %d", p.MaxOutputSize, 50*1024)
	}

	// filesystem_read and env_access should be allowed by default.
	if !p.AllowedCapabilities[CapabilityFilesystemRead] {
		t.Error("expected filesystem_read to be allowed")
	}
	if !p.AllowedCapabilities[CapabilityEnvAccess] {
		t.Error("expected env_access to be allowed")
	}

	// network, filesystem_write, subprocess should be denied.
	if p.AllowedCapabilities[CapabilityNetwork] {
		t.Error("expected network to be denied")
	}
	if p.AllowedCapabilities[CapabilityFilesystemWrite] {
		t.Error("expected filesystem_write to be denied")
	}
	if p.AllowedCapabilities[CapabilitySubprocess] {
		t.Error("expected subprocess to be denied")
	}

	wantLangs := map[string]bool{"python": true, "javascript": true, "typescript": true}
	for _, lang := range p.AllowedLanguages {
		if !wantLangs[lang] {
			t.Errorf("unexpected language %q in default policy", lang)
		}
		delete(wantLangs, lang)
	}
	if len(wantLangs) > 0 {
		t.Errorf("missing languages in default policy: %v", wantLangs)
	}
}

func TestEnforcePolicy(t *testing.T) {
	tests := []struct {
		name        string
		policy      SandboxPolicy
		ops         []Operation
		wantAllowed int
		wantDenied  int
	}{
		{
			name:        "empty ops",
			policy:      DefaultSandboxPolicy(),
			ops:         nil,
			wantAllowed: 0,
			wantDenied:  0,
		},
		{
			name:   "default policy allows file_read and env_access",
			policy: DefaultSandboxPolicy(),
			ops: []Operation{
				{Type: "file_read", Detail: "open()", RiskLevel: "read"},
				{Type: "env_access", Detail: "os.getenv()", RiskLevel: "read"},
			},
			wantAllowed: 2,
			wantDenied:  0,
		},
		{
			name:   "default policy denies network and subprocess",
			policy: DefaultSandboxPolicy(),
			ops: []Operation{
				{Type: "network_call", Detail: "requests.get", RiskLevel: "write"},
				{Type: "subprocess", Detail: "subprocess.run", RiskLevel: "admin"},
			},
			wantAllowed: 0,
			wantDenied:  2,
		},
		{
			name:   "default policy denies file_write and file_delete",
			policy: DefaultSandboxPolicy(),
			ops: []Operation{
				{Type: "file_write", Detail: "shutil.copy", RiskLevel: "write"},
				{Type: "file_delete", Detail: "os.remove", RiskLevel: "destructive"},
			},
			wantAllowed: 0,
			wantDenied:  2,
		},
		{
			name:   "default policy denies system_call and restricted_import",
			policy: DefaultSandboxPolicy(),
			ops: []Operation{
				{Type: "system_call", Detail: "eval()", RiskLevel: "admin"},
				{Type: "restricted_import", Detail: "import ctypes", RiskLevel: "admin"},
			},
			wantAllowed: 0,
			wantDenied:  2,
		},
		{
			name: "permissive policy allows everything",
			policy: SandboxPolicy{
				AllowedCapabilities: map[Capability]bool{
					CapabilityNetwork:         true,
					CapabilityFilesystemRead:  true,
					CapabilityFilesystemWrite: true,
					CapabilitySubprocess:      true,
					CapabilityEnvAccess:       true,
				},
				MaxOutputSize: 100 * 1024,
			},
			ops: []Operation{
				{Type: "network_call", Detail: "fetch", RiskLevel: "write"},
				{Type: "file_read", Detail: "open()", RiskLevel: "read"},
				{Type: "file_delete", Detail: "os.remove", RiskLevel: "destructive"},
				{Type: "subprocess", Detail: "exec()", RiskLevel: "admin"},
				{Type: "env_access", Detail: "process.env", RiskLevel: "read"},
			},
			wantAllowed: 5,
			wantDenied:  0,
		},
		{
			name:   "mixed allow/deny",
			policy: DefaultSandboxPolicy(),
			ops: []Operation{
				{Type: "file_read", Detail: "open()", RiskLevel: "read"},
				{Type: "network_call", Detail: "requests.get", RiskLevel: "write"},
				{Type: "env_access", Detail: "os.environ", RiskLevel: "read"},
				{Type: "subprocess", Detail: "subprocess.run", RiskLevel: "admin"},
			},
			wantAllowed: 2,
			wantDenied:  2,
		},
		{
			name:   "unknown operation type denied by default",
			policy: DefaultSandboxPolicy(),
			ops: []Operation{
				{Type: "quantum_teleport", Detail: "spooky action", RiskLevel: "admin"},
			},
			wantAllowed: 0,
			wantDenied:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, denied := EnforcePolicy(tt.policy, tt.ops)
			if len(allowed) != tt.wantAllowed {
				t.Errorf("allowed count = %d, want %d", len(allowed), tt.wantAllowed)
			}
			if len(denied) != tt.wantDenied {
				t.Errorf("denied count = %d, want %d", len(denied), tt.wantDenied)
			}
		})
	}
}

func TestHighestOperationRisk(t *testing.T) {
	tests := []struct {
		name     string
		ops      []Operation
		wantRisk string
	}{
		{
			name:     "empty ops defaults to read",
			ops:      nil,
			wantRisk: "read",
		},
		{
			name:     "single read",
			ops:      []Operation{{RiskLevel: "read"}},
			wantRisk: "read",
		},
		{
			name:     "single write",
			ops:      []Operation{{RiskLevel: "write"}},
			wantRisk: "write",
		},
		{
			name:     "single admin",
			ops:      []Operation{{RiskLevel: "admin"}},
			wantRisk: "admin",
		},
		{
			name:     "single destructive",
			ops:      []Operation{{RiskLevel: "destructive"}},
			wantRisk: "destructive",
		},
		{
			name: "mixed — destructive wins",
			ops: []Operation{
				{RiskLevel: "read"},
				{RiskLevel: "write"},
				{RiskLevel: "destructive"},
				{RiskLevel: "admin"},
			},
			wantRisk: "destructive",
		},
		{
			name: "mixed — admin wins over write",
			ops: []Operation{
				{RiskLevel: "read"},
				{RiskLevel: "write"},
				{RiskLevel: "admin"},
			},
			wantRisk: "admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HighestOperationRisk(tt.ops)
			if got != tt.wantRisk {
				t.Errorf("HighestOperationRisk = %s, want %s", got, tt.wantRisk)
			}
		})
	}
}
