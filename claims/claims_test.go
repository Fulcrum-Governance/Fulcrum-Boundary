package claims

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type claimLedger struct {
	Claims []claim `yaml:"claims"`
}

type claim struct {
	ID             string         `yaml:"id"`
	Claim          string         `yaml:"claim"`
	Status         string         `yaml:"status"`
	Evidence       evidence       `yaml:"evidence"`
	PublicLanguage publicLanguage `yaml:"public_language"`
	Gaps           []gap          `yaml:"gaps"`
}

type evidence struct {
	Tests []evidenceRef `yaml:"tests"`
	Docs  []evidenceRef `yaml:"docs"`
}

type evidenceRef struct {
	Path      string `yaml:"path"`
	Section   string `yaml:"section"`
	Assertion string `yaml:"assertion"`
}

type publicLanguage struct {
	Allowed   []string `yaml:"allowed"`
	Forbidden []string `yaml:"forbidden"`
}

type gap struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Spec        string `yaml:"spec"`
}

func TestBoundaryClaimsLedger(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	ledger := loadLedger(t, repoRoot)
	readme := readFile(t, filepath.Join(repoRoot, "README.md"))

	seen := map[string]bool{}
	validStatus := map[string]bool{
		"delivered": true,
		"partial":   true,
		"planned":   true,
		"false":     true,
	}
	buildTaskID := regexp.MustCompile(`^BND-[A-Z0-9]+-[0-9]{3}$`)

	for _, c := range ledger.Claims {
		if c.ID == "" {
			t.Fatal("claim missing id")
		}
		if seen[c.ID] {
			t.Fatalf("duplicate claim id %s", c.ID)
		}
		seen[c.ID] = true
		if c.Claim == "" {
			t.Fatalf("%s missing claim text", c.ID)
		}
		if !validStatus[c.Status] {
			t.Fatalf("%s has invalid status %q", c.ID, c.Status)
		}

		switch c.Status {
		case "delivered":
			requireEvidence(t, repoRoot, c, "test", c.Evidence.Tests)
			requireEvidence(t, repoRoot, c, "doc", c.Evidence.Docs)
		case "partial":
			if len(c.Gaps) == 0 {
				t.Fatalf("%s is partial but lists no gaps", c.ID)
			}
			for _, gap := range c.Gaps {
				if !buildTaskID.MatchString(gap.ID) {
					t.Fatalf("%s partial gap has invalid build-task id %q", c.ID, gap.ID)
				}
				if gap.Description == "" || gap.Spec == "" {
					t.Fatalf("%s gap %s must include description and spec reference", c.ID, gap.ID)
				}
			}
		case "false":
			if strings.Contains(readme, c.Claim) {
				t.Fatalf("false claim %q appears in README.md", c.Claim)
			}
		}
	}
}

func loadLedger(t *testing.T, repoRoot string) claimLedger {
	t.Helper()
	data := readFile(t, filepath.Join(repoRoot, "claims", "boundary_claims.yaml"))
	var ledger claimLedger
	if err := yaml.Unmarshal([]byte(data), &ledger); err != nil {
		t.Fatalf("parse claims ledger: %v", err)
	}
	if len(ledger.Claims) == 0 {
		t.Fatal("claims ledger is empty")
	}
	return ledger
}

func requireEvidence(t *testing.T, repoRoot string, c claim, kind string, refs []evidenceRef) {
	t.Helper()
	if len(refs) == 0 {
		t.Fatalf("%s delivered claim needs at least one %s evidence reference", c.ID, kind)
	}
	for _, ref := range refs {
		if ref.Path == "" {
			t.Fatalf("%s has empty %s evidence path", c.ID, kind)
		}
		if _, err := os.Stat(filepath.Join(repoRoot, ref.Path)); err != nil {
			t.Fatalf("%s %s evidence path %s is not present: %v", c.ID, kind, ref.Path, err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
