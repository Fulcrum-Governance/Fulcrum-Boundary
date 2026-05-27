package adapter_conformance

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestWebhookConformanceDeclaration(t *testing.T) {
	root := repoRoot(t)
	decl := loadDeclaration(t, filepath.Join(root, "adapters", "webhook"))

	if decl.Status != string(governance.AdapterMaturityPreview) {
		t.Fatalf("webhook status = %q, want preview", decl.Status)
	}
	want := map[string]string{
		string(governance.AdapterStepParse):       string(governance.AdapterStepImplemented),
		string(governance.AdapterStepIdentify):    string(governance.AdapterStepImplemented),
		string(governance.AdapterStepEvaluate):    string(governance.AdapterStepImplemented),
		string(governance.AdapterStepDeny):        string(governance.AdapterStepImplemented),
		string(governance.AdapterStepForward):     string(governance.AdapterStepDelegated),
		string(governance.AdapterStepInspect):     string(governance.AdapterStepImplemented),
		string(governance.AdapterStepMetadata):    string(governance.AdapterStepDelegated),
		string(governance.AdapterStepRecord):      string(governance.AdapterStepDelegated),
		string(governance.AdapterStepBypassProof): string(governance.AdapterStepDelegated),
		string(governance.AdapterStepFailClosed):  string(governance.AdapterStepImplemented),
	}
	for step, state := range want {
		if got := decl.Lifecycle[step]; got != state {
			t.Fatalf("webhook lifecycle %s = %q, want %q", step, got, state)
		}
	}
	if len(decl.Gaps) == 0 {
		t.Fatal("webhook must retain a production gap until deployment bypass evidence exists")
	}
}

func TestWebhookDocsSeparateModes(t *testing.T) {
	root := repoRoot(t)
	doc := readFile(t, filepath.Join(root, "docs", "adapters", "WEBHOOK.md"))
	for _, phrase := range []string{
		"Informational mode is a post-execution audit path",
		"cannot deny before execution",
		"Execution mode is a pre-execution approval path",
		"must not forward denied webhooks",
		"Informational webhooks are inherently bypassable",
	} {
		if !strings.Contains(doc, phrase) {
			t.Fatalf("webhook doc missing %q", phrase)
		}
	}
}
