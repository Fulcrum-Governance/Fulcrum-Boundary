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
