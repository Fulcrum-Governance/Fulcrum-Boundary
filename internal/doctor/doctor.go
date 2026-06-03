package doctor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const SchemaVersion = "boundary.doctor.v1"

type Options struct {
	Surface string
	Root    string
}

type Result struct {
	SchemaVersion       string              `json:"schema_version"`
	Status              string              `json:"status"`
	ProjectRoot         string              `json:"project_root"`
	RequiresCredentials bool                `json:"requires_credentials"`
	RequiresNetwork     bool                `json:"requires_network"`
	MutatesLiveSystems  bool                `json:"mutates_live_systems"`
	ReportRedacted      bool                `json:"report_redacted,omitempty"`
	Environment         []Check             `json:"environment,omitempty"`
	Surfaces            []SurfaceDiagnostic `json:"surfaces"`
}

type SurfaceDiagnostic struct {
	Surface       string   `json:"surface"`
	Label         string   `json:"label"`
	Status        string   `json:"status"`
	Checks        []Check  `json:"checks"`
	BypassCaveats []string `json:"bypass_caveats"`
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func Run(opts Options) (*Result, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root = wd
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	surface := strings.ToLower(strings.TrimSpace(opts.Surface))
	if surface == "" {
		surface = "all"
	}
	var diagnostics []SurfaceDiagnostic
	switch surface {
	case "all":
		diagnostics = []SurfaceDiagnostic{
			mcpDiagnostic(absRoot),
			commandDiagnostic(absRoot),
			editDiagnostic(absRoot),
		}
	case "mcp":
		diagnostics = []SurfaceDiagnostic{mcpDiagnostic(absRoot)}
	case "command":
		diagnostics = []SurfaceDiagnostic{commandDiagnostic(absRoot)}
	case "edit":
		diagnostics = []SurfaceDiagnostic{editDiagnostic(absRoot)}
	default:
		return nil, fmt.Errorf("unknown doctor surface %q; expected all, mcp, command, or edit", opts.Surface)
	}

	return &Result{
		SchemaVersion:       SchemaVersion,
		Status:              "pass",
		ProjectRoot:         absRoot,
		RequiresCredentials: false,
		RequiresNetwork:     false,
		MutatesLiveSystems:  false,
		Environment:         environmentDiagnostics(),
		Surfaces:            diagnostics,
	}, nil
}

// RedactedReport returns a shareable copy of a doctor result. It preserves the
// diagnostic verdicts and caveats while removing the local project root.
func RedactedReport(result *Result) *Result {
	if result == nil {
		return nil
	}
	redacted := *result
	redacted.ProjectRoot = "<redacted>"
	redacted.ReportRedacted = true
	redacted.Environment = cloneChecks(result.Environment)
	redacted.Surfaces = make([]SurfaceDiagnostic, len(result.Surfaces))
	for i, surface := range result.Surfaces {
		redacted.Surfaces[i] = surface
		redacted.Surfaces[i].Checks = cloneChecks(surface.Checks)
		redacted.Surfaces[i].BypassCaveats = append([]string(nil), surface.BypassCaveats...)
	}
	return &redacted
}

func environmentDiagnostics() []Check {
	goPath, err := exec.LookPath("go")
	if err != nil || strings.TrimSpace(goPath) == "" {
		return []Check{
			warn("Go toolchain", "go command is not available on PATH; install Go 1.25+ before using go install"),
			warn("cgo / C toolchain", "not checked because go command is unavailable"),
			warn("go install PATH", "not checked because go command is unavailable"),
		}
	}

	env, err := readGoEnv()
	if err != nil {
		return []Check{
			warn("Go toolchain", "go command is available, but go env could not be read"),
			warn("cgo / C toolchain", "not checked because go env could not be read"),
			warn("go install PATH", "not checked because go env could not be read"),
		}
	}

	return []Check{
		goToolchainCheck(env["GOVERSION"]),
		cgoToolchainCheck(env["CGO_ENABLED"], env["CC"]),
		goInstallPathCheck(env["GOBIN"], env["GOPATH"]),
	}
}

func readGoEnv() (map[string]string, error) {
	out, err := exec.Command("go", "env", "GOVERSION", "CGO_ENABLED", "CC", "GOBIN", "GOPATH").Output()
	if err != nil {
		return nil, err
	}
	keys := []string{"GOVERSION", "CGO_ENABLED", "CC", "GOBIN", "GOPATH"}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	env := map[string]string{}
	for i, key := range keys {
		if i < len(lines) {
			env[key] = strings.TrimSpace(lines[i])
		}
	}
	return env, nil
}

func goToolchainCheck(version string) Check {
	version = strings.TrimSpace(version)
	if version == "" {
		return warn("Go toolchain", "go version could not be read; Boundary requires Go 1.25+")
	}
	if goVersionAtLeast(version, 1, 25) {
		return pass("Go toolchain", version+" detected; Boundary requires Go 1.25+")
	}
	return warn("Go toolchain", version+" detected; Boundary requires Go 1.25+")
}

func cgoToolchainCheck(cgoEnabled, cc string) Check {
	cgoEnabled = strings.TrimSpace(cgoEnabled)
	cc = strings.TrimSpace(cc)
	if cgoEnabled != "1" {
		return warn("cgo / C toolchain", "CGO_ENABLED="+emptyAsUnset(cgoEnabled)+"; Boundary's default build requires cgo enabled")
	}
	if cc == "" {
		return warn("cgo / C toolchain", "CGO_ENABLED=1, but no C compiler is configured in CC")
	}
	compiler := strings.Fields(cc)[0]
	if _, err := exec.LookPath(compiler); err != nil {
		return warn("cgo / C toolchain", "CGO_ENABLED=1, but the configured C compiler does not resolve on PATH")
	}
	return pass("cgo / C toolchain", "CGO_ENABLED=1 and the configured C compiler resolves on PATH")
}

func goInstallPathCheck(gobin, gopath string) Check {
	targetDir, source := goInstallDir(gobin, gopath)
	if targetDir == "" {
		return warn("go install PATH", "go install target directory could not be determined from GOBIN or GOPATH")
	}
	if _, err := exec.LookPath("boundary"); err == nil {
		return pass("go install PATH", "boundary command resolves on PATH")
	}
	if pathContainsDir(os.Getenv("PATH"), targetDir) {
		return pass("go install PATH", source+" is on PATH; run go install if boundary still does not resolve")
	}
	return warn("go install PATH", source+" is not on PATH; add it after go install so boundary resolves by name")
}

func goInstallDir(gobin, gopath string) (string, string) {
	if strings.TrimSpace(gobin) != "" {
		return filepath.Clean(gobin), "GOBIN"
	}
	if strings.TrimSpace(gopath) != "" {
		return filepath.Clean(filepath.Join(gopath, "bin")), "GOPATH/bin"
	}
	return "", ""
}

func pathContainsDir(pathValue, dir string) bool {
	if strings.TrimSpace(pathValue) == "" || strings.TrimSpace(dir) == "" {
		return false
	}
	target := filepath.Clean(dir)
	for _, entry := range filepath.SplitList(pathValue) {
		if filepath.Clean(entry) == target {
			return true
		}
	}
	return false
}

func goVersionAtLeast(version string, wantMajor, wantMinor int) bool {
	major, minor, ok := parseGoVersion(version)
	if !ok {
		return false
	}
	if major != wantMajor {
		return major > wantMajor
	}
	return minor >= wantMinor
}

func parseGoVersion(version string) (int, int, bool) {
	version = strings.TrimSpace(strings.TrimPrefix(version, "go"))
	if version == "" {
		return 0, 0, false
	}
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(trimNumeric(parts[0]))
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(trimNumeric(parts[1]))
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

func trimNumeric(value string) string {
	var out bytes.Buffer
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		out.WriteRune(r)
	}
	return out.String()
}

func emptyAsUnset(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unset"
	}
	return value
}

func cloneChecks(checks []Check) []Check {
	if checks == nil {
		return nil
	}
	return append([]Check(nil), checks...)
}

func mcpDiagnostic(root string) SurfaceDiagnostic {
	checks := []Check{
		pass("policy verifier", "local YAML policy verification is available through boundary verify --policies"),
		optionalPath(root, ".boundary/firewall", "firewall workspace"),
		warn("route caveat", "MCP protection applies only to client configs routed through Boundary"),
	}
	return SurfaceDiagnostic{
		Surface: "mcp",
		Label:   "MCP",
		Status:  surfaceStatus(checks),
		Checks:  checks,
		BypassCaveats: []string{
			"Direct upstream MCP server access is outside Boundary unless operators remove or block that path.",
			"Inventory and descriptor locks are local evidence; they do not prove a live deployment route is enforced.",
		},
	}
}

func commandDiagnostic(root string) SurfaceDiagnostic {
	checks := []Check{
		pass("classifier", "boundary command classify and boundary command run are available for routed command paths"),
		optionalPath(root, ".boundary/bin", "project command shims"),
		warn("route caveat", "direct shell execution is outside Boundary unless commands route through the wrapper or shims"),
	}
	return SurfaceDiagnostic{
		Surface: "command",
		Label:   "Command Boundary",
		Status:  surfaceStatus(checks),
		Checks:  checks,
		BypassCaveats: []string{
			"Direct shell, scripts, cron, SSH, and CI jobs are bypasses unless explicitly routed through Boundary.",
			"Command Boundary does not provide shell sandboxing.",
		},
	}
}

func editDiagnostic(root string) SurfaceDiagnostic {
	checks := []Check{
		pass("classifier", "boundary edit inspect and boundary edit apply are available for routed edit envelopes"),
		optionalPath(root, ".boundary/edit", "edit evidence workspace"),
		warn("route caveat", "direct editor writes and direct git apply are outside Boundary unless routed through edit envelopes"),
	}
	return SurfaceDiagnostic{
		Surface: "edit",
		Label:   "Edit Boundary",
		Status:  surfaceStatus(checks),
		Checks:  checks,
		BypassCaveats: []string{
			"Direct editor writes, direct filesystem mutation, and direct git apply are bypasses.",
			"Edit Boundary does not provide filesystem sandboxing.",
		},
	}
}

func optionalPath(root, relPath, name string) Check {
	path := filepath.Join(root, relPath)
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return warn(name, relPath+" is not present; run the relevant setup command when you want this surface active")
		}
		return warn(name, fmt.Sprintf("could not inspect %s: %v", relPath, err))
	}
	if !stat.IsDir() {
		return warn(name, relPath+" exists but is not a directory")
	}
	return pass(name, relPath+" is present")
}

func pass(name, detail string) Check {
	return Check{Name: name, Status: "pass", Detail: detail}
}

func warn(name, detail string) Check {
	return Check{Name: name, Status: "warn", Detail: detail}
}

func surfaceStatus(checks []Check) string {
	for _, check := range checks {
		if check.Status == "warn" {
			return "warn"
		}
	}
	return "pass"
}
