package doctor

import (
	"fmt"
	"os"
	"path/filepath"
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
		Surfaces:            diagnostics,
	}, nil
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
