package boundarycli

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// sentinelCommands are command names every shell script must mention. They span
// a plain command (version), a compound parent (demo), and the new command
// itself (completion), so a script that silently dropped a category fails.
var sentinelCommands = []string{"version", "demo", "completion", "trust"}

func TestCompletion_BashOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "bash"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("completion bash: exit %d, stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if strings.TrimSpace(out) == "" {
		t.Fatal("completion bash: empty output")
	}
	for _, want := range sentinelCommands {
		if !strings.Contains(out, want) {
			t.Fatalf("completion bash: missing sentinel %q:\n%s", want, out)
		}
	}
	// A compound subcommand must appear too, proving level-2 completion wiring.
	if !strings.Contains(out, "github-lethal-trifecta") {
		t.Fatalf("completion bash: missing compound subcommand:\n%s", out)
	}
	if !strings.Contains(out, "complete -F _boundary_completion boundary") {
		t.Fatalf("completion bash: missing complete registration:\n%s", out)
	}
}

func TestCompletion_ZshOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "zsh"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("completion zsh: exit %d, stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if strings.TrimSpace(out) == "" {
		t.Fatal("completion zsh: empty output")
	}
	if !strings.HasPrefix(out, "#compdef boundary") {
		t.Fatalf("completion zsh: missing #compdef header:\n%s", out)
	}
	for _, want := range sentinelCommands {
		if !strings.Contains(out, want) {
			t.Fatalf("completion zsh: missing sentinel %q:\n%s", want, out)
		}
	}
}

func TestCompletion_FishOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "fish"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("completion fish: exit %d, stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if strings.TrimSpace(out) == "" {
		t.Fatal("completion fish: empty output")
	}
	for _, want := range sentinelCommands {
		if !strings.Contains(out, want) {
			t.Fatalf("completion fish: missing sentinel %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "__fish_seen_subcommand_from demo") {
		t.Fatalf("completion fish: missing compound gating:\n%s", out)
	}
}

// TestCompletion_BashParses pipes the generated bash script through `bash -n`
// (parse-only, no execution). A syntactically broken script must never ship.
func TestCompletion_BashParses(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not on PATH; skipping bash -n syntax check: %v", err)
	}
	var stdout bytes.Buffer
	if code := Run([]string{"completion", "bash"}, &stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("completion bash: exit %d", code)
	}
	cmd := exec.Command(bashPath, "-n")
	cmd.Stdin = bytes.NewReader(stdout.Bytes())
	var cmdErr bytes.Buffer
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bash -n rejected generated script: %v\n%s\n--- script ---\n%s", err, cmdErr.String(), stdout.String())
	}
}

// TestCompletion_ZshParses runs the generated zsh script through `zsh -n` when
// zsh is available, otherwise it skips with a log (per the lane spec).
func TestCompletion_ZshParses(t *testing.T) {
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skipf("zsh not on PATH; skipping zsh -n syntax check: %v", err)
	}
	var stdout bytes.Buffer
	if code := Run([]string{"completion", "zsh"}, &stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("completion zsh: exit %d", code)
	}
	cmd := exec.Command(zshPath, "-n")
	cmd.Stdin = bytes.NewReader(stdout.Bytes())
	var cmdErr bytes.Buffer
	cmd.Stderr = &cmdErr
	if err := cmd.Run(); err != nil {
		t.Fatalf("zsh -n rejected generated script: %v\n%s\n--- script ---\n%s", err, cmdErr.String(), stdout.String())
	}
}

func TestCompletion_UnknownShellExitsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "powershell"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for unknown shell, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown shell") {
		t.Fatalf("expected unknown-shell error, got: %s", stderr.String())
	}
	// The error must enumerate the valid options (house style).
	for _, opt := range []string{"bash", "zsh", "fish"} {
		if !strings.Contains(stderr.String(), opt) {
			t.Fatalf("unknown-shell error must enumerate %q: %s", opt, stderr.String())
		}
	}
}

func TestCompletion_MissingShellExitsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 with no shell argument, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage on missing shell, got: %s", stderr.String())
	}
}

func TestCompletion_HelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"completion", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("completion --help: exit %d", code)
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "Usage:") {
		t.Fatalf("completion --help missing Usage section:\n%s", combined)
	}
	if !strings.Contains(combined, "regenerate") {
		t.Fatalf("completion --help missing static/regenerate note:\n%s", combined)
	}
}

// TestCompletion_TopLevelMatchesDispatch is the drift guard: the authoritative
// topLevelCommands slice must equal the set of real command arms in Run()'s
// dispatch switch in cli.go. It parses cli.go's AST, collects every `case "X":`
// string literal, drops the help/version alias arms (which are not
// tab-completion commands), and compares the result to topLevelCommands. Adding a
// command to the dispatch without adding it here (or vice versa) fails this test.
func TestCompletion_TopLevelMatchesDispatch(t *testing.T) {
	// Alias arms handled by Run() that are intentionally absent from completion:
	// they are help/version spellings, not commands a user completes to.
	aliasArms := map[string]bool{
		"--help":    true,
		"-h":        true,
		"help":      true,
		"--version": true,
		"-v":        true,
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cli.go", nil, 0)
	if err != nil {
		t.Fatalf("parse cli.go: %v", err)
	}

	// Scope the walk to Run()'s body only. cli.go also holds unrelated switches
	// (trust-state colorizers, action labels, inner subcommand dispatchers) whose
	// case literals are not top-level commands; inspecting the whole file would
	// fold them in. The top-level command surface is exactly the switch arms in
	// Run, so we parse those and nothing else.
	var runBody *ast.BlockStmt
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "Run" {
			runBody = fn.Body
			break
		}
	}
	if runBody == nil {
		t.Fatal("could not locate Run() in cli.go; parser or file layout changed")
	}

	dispatch := map[string]bool{}
	ast.Inspect(runBody, func(n ast.Node) bool {
		clause, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range clause.List {
			lit, ok := expr.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			val := strings.Trim(lit.Value, `"`)
			if val == "" || aliasArms[val] {
				continue
			}
			dispatch[val] = true
		}
		return true
	})

	if len(dispatch) == 0 {
		t.Fatal("parsed no dispatch case arms from Run() in cli.go; parser or file layout changed")
	}

	declared := map[string]bool{}
	for _, name := range topLevelCommands {
		if declared[name] {
			t.Fatalf("topLevelCommands has duplicate %q", name)
		}
		declared[name] = true
	}

	var missingFromDeclared, missingFromDispatch []string
	for name := range dispatch {
		if !declared[name] {
			missingFromDeclared = append(missingFromDeclared, name)
		}
	}
	for name := range declared {
		if !dispatch[name] {
			missingFromDispatch = append(missingFromDispatch, name)
		}
	}
	sort.Strings(missingFromDeclared)
	sort.Strings(missingFromDispatch)
	if len(missingFromDeclared) > 0 {
		t.Errorf("dispatch arms missing from topLevelCommands (add them): %v", missingFromDeclared)
	}
	if len(missingFromDispatch) > 0 {
		t.Errorf("topLevelCommands entries with no dispatch arm (remove or wire them): %v", missingFromDispatch)
	}
}

// TestCompletion_RootHelpListsCompletion guards that the command stays advertised
// in the root help surface alongside its dispatch arm.
func TestCompletion_RootHelpListsCompletion(t *testing.T) {
	var stdout bytes.Buffer
	if code := Run([]string{"--help"}, &stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("root help exit %d", code)
	}
	if !strings.Contains(stdout.String(), "completion") {
		t.Fatalf("root help missing completion command:\n%s", stdout.String())
	}
}
