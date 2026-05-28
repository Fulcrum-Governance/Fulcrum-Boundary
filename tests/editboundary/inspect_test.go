package editboundary_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

func TestEditInspectTextOutputDoesNotApplyPatch(t *testing.T) {
	tempDir := t.TempDir()
	sentinel := filepath.Join(tempDir, "docs", "example.md")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("# Example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patchPath := filepath.Join("..", "..", "fixtures", "editboundary", "docs.diff")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "inspect", "--patch", patchPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Edit Boundary Inspection") || !strings.Contains(got, "Highest class: E1 safe content edit") {
		t.Fatalf("unexpected output:\n%s", got)
	}
	content, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# Example\n" {
		t.Fatalf("inspect applied patch unexpectedly: %q", content)
	}
}

func TestEditInspectJSONOutputRedactsSecretPath(t *testing.T) {
	patchPath := filepath.Join("..", "..", "fixtures", "editboundary", "secret.diff")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "inspect", "--patch", patchPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), ".env") || strings.Contains(stdout.String(), "example-secret") {
		t.Fatalf("secret path or value leaked:\n%s", stdout.String())
	}
	var inspection editboundary.Inspection
	if err := json.Unmarshal(stdout.Bytes(), &inspection); err != nil {
		t.Fatalf("parse json: %v\n%s", err, stdout.String())
	}
	if inspection.HighestClass != editboundary.ClassSecretBearing || inspection.RecommendedAction != editboundary.ActionDeny {
		t.Fatalf("inspection = %#v", inspection)
	}
}

func TestEditInspectPackageScriptsRequireApproval(t *testing.T) {
	patchPath := filepath.Join("..", "..", "fixtures", "editboundary", "package-scripts.diff")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "inspect", "--patch", patchPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Highest class: E6 execution behavior mutation") || !strings.Contains(out, "Recommended action: require_approval") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestEditInspectRejectsMissingOrAmbiguousSource(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "inspect"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected missing source to fail")
	}
	if !strings.Contains(stderr.String(), "exactly one patch source is required") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}
