package adapter_conformance

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestGRPCConformanceDeclaration(t *testing.T) {
	root := repoRoot(t)
	decl := loadDeclaration(t, filepath.Join(root, "adapters", "grpc"))

	if decl.Status != string(governance.AdapterMaturityPreview) {
		t.Fatalf("gRPC status = %q, want preview", decl.Status)
	}
	want := map[string]string{
		string(governance.AdapterStepParse):       string(governance.AdapterStepImplemented),
		string(governance.AdapterStepIdentify):    string(governance.AdapterStepImplemented),
		string(governance.AdapterStepEvaluate):    string(governance.AdapterStepImplemented),
		string(governance.AdapterStepDeny):        string(governance.AdapterStepImplemented),
		string(governance.AdapterStepForward):     string(governance.AdapterStepDelegated),
		string(governance.AdapterStepInspect):     string(governance.AdapterStepImplemented),
		string(governance.AdapterStepMetadata):    string(governance.AdapterStepImplemented),
		string(governance.AdapterStepRecord):      string(governance.AdapterStepDelegated),
		string(governance.AdapterStepBypassProof): string(governance.AdapterStepDelegated),
		string(governance.AdapterStepFailClosed):  string(governance.AdapterStepImplemented),
	}
	for step, state := range want {
		if got := decl.Lifecycle[step]; got != state {
			t.Fatalf("gRPC lifecycle %s = %q, want %q", step, got, state)
		}
	}
	if len(decl.Gaps) == 0 {
		t.Fatal("gRPC must retain a production gap until bypass and streaming evidence exist")
	}
}

func TestGRPCDocsDeclareStreamingLimitation(t *testing.T) {
	root := repoRoot(t)
	doc := readFile(t, filepath.Join(root, "docs", "adapters", "GRPC.md"))
	for _, phrase := range []string{
		"governance-action",
		"Streaming RPC messages are not individually governed",
		"production readiness for unary RPCs only",
		"Direct access to the underlying gRPC service",
	} {
		if !strings.Contains(doc, phrase) {
			t.Fatalf("gRPC doc missing %q", phrase)
		}
	}
}
