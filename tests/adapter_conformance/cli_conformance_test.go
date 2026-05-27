package adapter_conformance

import (
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestCLIConformanceDeclaration(t *testing.T) {
	root := repoRoot(t)
	decl := loadDeclaration(t, filepath.Join(root, "adapters", "cli"))

	if decl.Status != string(governance.AdapterMaturityPreview) {
		t.Fatalf("CLI status = %q, want preview", decl.Status)
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
			t.Fatalf("CLI lifecycle step %s = %q, want implemented", step, decl.Lifecycle[string(step)])
		}
	}
	for _, step := range []governance.AdapterLifecycleStep{
		governance.AdapterStepEvaluate,
		governance.AdapterStepRecord,
		governance.AdapterStepBypassProof,
	} {
		if decl.Lifecycle[string(step)] != string(governance.AdapterStepDelegated) {
			t.Fatalf("CLI lifecycle step %s = %q, want delegated", step, decl.Lifecycle[string(step)])
		}
	}
}

func TestCLIDefaultFailClosed(t *testing.T) {
	found := false
	for _, tr := range governance.DefaultFailClosedTransports {
		if tr == governance.TransportCLI {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TransportCLI must default to fail-closed for wrapper-owned execution")
	}
}
