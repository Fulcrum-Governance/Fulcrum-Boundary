package adapter_conformance

import (
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestA2AConformanceDeclaration(t *testing.T) {
	root := repoRoot(t)
	decl := loadDeclaration(t, filepath.Join(root, "adapters", "a2a"))

	if decl.Status != string(governance.AdapterMaturityPreview) {
		t.Fatalf("A2A status = %q, want preview", decl.Status)
	}
	if decl.TargetStatus != string(governance.AdapterMaturityPreview) {
		t.Fatalf("A2A target_status = %q, want preview", decl.TargetStatus)
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
			t.Fatalf("A2A lifecycle step %s = %q, want implemented", step, decl.Lifecycle[string(step)])
		}
	}
	for _, step := range []governance.AdapterLifecycleStep{
		governance.AdapterStepEvaluate,
		governance.AdapterStepRecord,
		governance.AdapterStepBypassProof,
	} {
		if decl.Lifecycle[string(step)] != string(governance.AdapterStepDelegated) {
			t.Fatalf("A2A lifecycle step %s = %q, want delegated", step, decl.Lifecycle[string(step)])
		}
	}
	if len(decl.Gaps) == 0 {
		t.Fatal("A2A preview declaration should retain live conformance/bypass limitation gaps")
	}
}

func TestA2ADefaultFailClosed(t *testing.T) {
	found := false
	for _, tr := range governance.DefaultFailClosedTransports {
		if tr == governance.TransportA2A {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TransportA2A must default to fail-closed for preview lifecycle")
	}
}
