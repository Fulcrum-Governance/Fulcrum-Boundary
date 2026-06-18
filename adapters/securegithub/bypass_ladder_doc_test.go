package securegithub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoDoc reads a repo-root-relative doc from the adapters/securegithub package
// directory (two levels down: adapters/securegithub -> repo root).
func repoDoc(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(filepath.Join("..", "..", rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestBypassLadderDocIsLimitationFramed(t *testing.T) {
	doc := strings.ToLower(repoDoc(t, "docs/secure-mcp/GITHUB_BYPASS_LADDER.md"))
	// Must teach the ladder.
	for _, want := range []string{"l0", "l1", "l2", "l3", "production-candidate"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("ladder doc missing %q", want)
		}
	}
	// Must stay honest: deny bypass-resistance and a public production label.
	if !strings.Contains(doc, "does not prove") {
		t.Fatal("ladder doc must state what it does not prove")
	}
	if !strings.Contains(doc, "preview") {
		t.Fatal("ladder doc must state Secure GitHub stays preview")
	}
	if strings.Contains(doc, "production secure github") {
		t.Fatal("ladder doc must not assert production Secure GitHub")
	}
}

func TestBypassPacketDocIsOperatorTemplate(t *testing.T) {
	doc := strings.ToLower(repoDoc(t, "docs/deployment/secure-github-bypass-proof-packet.md"))
	for _, want := range []string{"operator", "attest", "egress", "github app", "does not prove"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("packet template missing %q", want)
		}
	}
	if strings.Contains(doc, "production secure github") {
		t.Fatal("packet template must not assert production Secure GitHub")
	}
}
