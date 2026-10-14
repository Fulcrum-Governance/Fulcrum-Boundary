package adapter_conformance

import (
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestCodeExecConformanceDeclaration(t *testing.T) {
	root := repoRoot(t)
	decl := loadDeclaration(t, filepath.Join(root, "adapters", "codeexec"))

	if decl.Status != string(governance.AdapterMaturityPreview) {
		t.Fatalf("CodeExec status = %q, want preview", decl.Status)
	}
	if decl.TargetStatus != string(governance.AdapterMaturityPreview) {
		t.Fatalf("CodeExec target_status = %q, want preview until a real sandbox boundary is tested", decl.TargetStatus)
	}
	for _, step := range []governance.AdapterLifecycleStep{
		governance.AdapterStepParse,
		governance.AdapterStepIdentify,
		governance.AdapterStepDeny,
		governance.AdapterStepForward,
		governance.AdapterStepInspect,
		governance.AdapterStepMetadata,
		governance.AdapterStepFailClosed,
	} {
		if decl.Lifecycle[string(step)] != string(governance.AdapterStepImplemented) {
			t.Fatalf("CodeExec lifecycle step %s = %q, want implemented", step, decl.Lifecycle[string(step)])
		}
	}
	for _, step := range []governance.AdapterLifecycleStep{
		governance.AdapterStepEvaluate,
		governance.AdapterStepRecord,
		governance.AdapterStepBypassProof,
	} {
		if decl.Lifecycle[string(step)] != string(governance.AdapterStepDelegated) {
			t.Fatalf("CodeExec lifecycle step %s = %q, want delegated", step, decl.Lifecycle[string(step)])
		}
	}
	if len(decl.Gaps) == 0 {
		t.Fatal("CodeExec preview declaration should retain sandbox and deployment topology gaps")
	}
}

func TestCodeExecDefaultFailClosed(t *testing.T) {
	// Under the inverted enforcement model there is no list to check:
	// TransportCodeExec must enforce required checks by default — an
	// evaluator error blocks code execution with check_indeterminate.
	requireTransportFailsClosedByDefault(t, governance.TransportCodeExec)
}
