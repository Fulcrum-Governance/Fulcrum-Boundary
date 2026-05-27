package adapter_conformance

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"gopkg.in/yaml.v3"
)

type readinessDeclaration struct {
	Adapter              string            `yaml:"adapter"`
	Status               string            `yaml:"status"`
	TargetStatus         string            `yaml:"target_status"`
	Lifecycle            map[string]string `yaml:"lifecycle"`
	DelegatedSteps       []delegatedStep   `yaml:"delegated_steps"`
	BypassModel          string            `yaml:"bypass_model"`
	FailClosedTransports []string          `yaml:"fail_closed_transports"`
	Evidence             readinessEvidence `yaml:"evidence"`
	Gaps                 []readinessGap    `yaml:"gaps"`
}

type delegatedStep struct {
	Step     string `yaml:"step"`
	Owner    string `yaml:"owner"`
	Contract string `yaml:"contract"`
}

type readinessEvidence struct {
	Tests []string `yaml:"tests"`
	Docs  []string `yaml:"docs"`
}

type readinessGap struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Spec        string `yaml:"spec"`
}

func TestEveryAdapterDeclaresReadiness(t *testing.T) {
	repoRoot := repoRoot(t)
	adapterDirs := adapterDirectories(t, repoRoot)
	readme := readFile(t, filepath.Join(repoRoot, "README.md"))
	matrix := readFile(t, filepath.Join(repoRoot, "docs", "ADAPTER_READINESS_MATRIX.md"))

	for _, dir := range adapterDirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			decl := loadDeclaration(t, dir)
			requireDeclaration(t, repoRoot, filepath.Base(dir), decl)
			requireREADMEListsMaturity(t, readme, decl)
			requireMatrixListsLifecycle(t, matrix, decl)
		})
	}
}

func TestProductionAdaptersPassConformanceRules(t *testing.T) {
	repoRoot := repoRoot(t)
	for _, dir := range adapterDirectories(t, repoRoot) {
		decl := loadDeclaration(t, dir)
		if decl.Status != string(governance.AdapterMaturityProduction) {
			continue
		}
		if len(decl.Evidence.Tests) == 0 {
			t.Fatalf("%s is production but has no conformance test evidence", decl.Adapter)
		}
		for step, state := range decl.Lifecycle {
			if state == string(governance.AdapterStepStub) {
				t.Fatalf("%s is production but lifecycle step %s is still stub", decl.Adapter, step)
			}
		}
		state := decl.Lifecycle[string(governance.AdapterStepBypassProof)]
		if state != string(governance.AdapterStepImplemented) && state != string(governance.AdapterStepDelegated) {
			t.Fatalf("%s is production but bypass_proof is %q", decl.Adapter, state)
		}
		if len(decl.FailClosedTransports) == 0 {
			t.Fatalf("%s is production but declares no fail-closed transports", decl.Adapter)
		}
	}
}

func adapterDirectories(t *testing.T, repoRoot string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot, "adapters"))
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(repoRoot, "adapters", entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "adapter.go")); err == nil {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		t.Fatal("no adapter directories discovered")
	}
	return dirs
}

func loadDeclaration(t *testing.T, adapterDir string) readinessDeclaration {
	t.Helper()
	path := filepath.Join(adapterDir, "readiness.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing readiness declaration %s: %v", path, err)
	}
	var decl readinessDeclaration
	if err := yaml.Unmarshal(data, &decl); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return decl
}

func requireDeclaration(t *testing.T, repoRoot, dirName string, decl readinessDeclaration) {
	t.Helper()
	if decl.Adapter != dirName {
		t.Fatalf("adapter declaration mismatch: dir=%s declaration=%s", dirName, decl.Adapter)
	}
	validStatuses := map[string]bool{
		string(governance.AdapterMaturityExperimental): true,
		string(governance.AdapterMaturityPreview):      true,
		string(governance.AdapterMaturityProduction):   true,
	}
	if !validStatuses[decl.Status] {
		t.Fatalf("%s has invalid status %q", decl.Adapter, decl.Status)
	}
	if decl.TargetStatus != "" && !validStatuses[decl.TargetStatus] {
		t.Fatalf("%s has invalid target_status %q", decl.Adapter, decl.TargetStatus)
	}
	validStates := map[string]bool{
		string(governance.AdapterStepImplemented):   true,
		string(governance.AdapterStepDelegated):     true,
		string(governance.AdapterStepNotApplicable): true,
		string(governance.AdapterStepStub):          true,
	}
	for _, step := range governance.AdapterLifecycleSteps {
		name := string(step)
		state, ok := decl.Lifecycle[name]
		if !ok {
			t.Fatalf("%s missing lifecycle step %s", decl.Adapter, name)
		}
		if !validStates[state] {
			t.Fatalf("%s lifecycle step %s has invalid state %q", decl.Adapter, name, state)
		}
	}
	for _, delegated := range decl.DelegatedSteps {
		if delegated.Step == "" || delegated.Owner == "" || delegated.Contract == "" {
			t.Fatalf("%s has incomplete delegated step: %+v", decl.Adapter, delegated)
		}
		if _, ok := decl.Lifecycle[delegated.Step]; !ok {
			t.Fatalf("%s delegated unknown step %s", decl.Adapter, delegated.Step)
		}
		if _, err := os.Stat(filepath.Join(repoRoot, delegated.Contract)); err != nil {
			t.Fatalf("%s delegated step contract %s missing: %v", decl.Adapter, delegated.Contract, err)
		}
	}
	if decl.BypassModel == "" {
		t.Fatalf("%s missing bypass_model", decl.Adapter)
	}
	for _, path := range append(decl.Evidence.Tests, decl.Evidence.Docs...) {
		if _, err := os.Stat(filepath.Join(repoRoot, path)); err != nil {
			t.Fatalf("%s evidence path %s missing: %v", decl.Adapter, path, err)
		}
	}
	for _, gap := range decl.Gaps {
		if gap.ID == "" || gap.Description == "" || gap.Spec == "" {
			t.Fatalf("%s has incomplete gap: %+v", decl.Adapter, gap)
		}
	}
}

func requireREADMEListsMaturity(t *testing.T, readme string, decl readinessDeclaration) {
	t.Helper()
	if !strings.Contains(readme, "### "+titleCase(decl.Status)) {
		t.Fatalf("README missing maturity heading for %s", decl.Status)
	}
	if !strings.Contains(readme, "`adapters/"+decl.Adapter+"`") {
		t.Fatalf("README missing adapter package adapters/%s", decl.Adapter)
	}
}

func requireMatrixListsLifecycle(t *testing.T, matrix string, decl readinessDeclaration) {
	t.Helper()
	if !strings.Contains(matrix, "| "+decl.Adapter+" | "+decl.Status+" |") {
		t.Fatalf("readiness matrix missing row for %s with status %s", decl.Adapter, decl.Status)
	}
	for _, step := range governance.AdapterLifecycleSteps {
		if !strings.Contains(matrix, string(step)) {
			t.Fatalf("readiness matrix missing lifecycle step %s", step)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
