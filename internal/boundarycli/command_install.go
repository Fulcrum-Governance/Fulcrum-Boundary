package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
)

func runCommandInstall(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary command install", stderr, commandHelp{
		Purpose: "Install project-local Command Boundary shims.",
		Usage:   "boundary command install --project [--project-root PATH]",
		Common: []string{
			"boundary command install --project",
			"boundary command install --project --project-root /path/to/repo",
		},
		Notes: []string{
			"install creates shims under .boundary/bin only.",
			"install never edits global shell startup files or global PATH.",
		},
	})
	project := fs.Bool("project", false, "install project-local shims under .boundary/bin")
	projectRoot := fs.String("project-root", "", "project root for shim installation; defaults to the current directory")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if !*project {
		fmt.Fprintln(stderr, "command install: --project is required; global shim installation is not supported")
		return 1
	}
	root, err := resolveProjectRoot(*projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "command install: %v\n", err)
		return 1
	}
	result, err := commandboundary.InstallProjectShims(root, commandboundary.DefaultShimCommands())
	if err != nil {
		fmt.Fprintf(stderr, "command install: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Installed %d project shims in %s\n", len(result.Created), ".boundary/bin")
	fmt.Fprintln(stdout, "To use project shims, run:")
	fmt.Fprintln(stdout, `  export PATH="$PWD/.boundary/bin:$PATH"`)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Or launch:")
	fmt.Fprintln(stdout, "  boundary shell")
	return 0
}

func runCommandUninstall(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary command uninstall", stderr, commandHelp{
		Purpose: "Remove project-local Command Boundary shims.",
		Usage:   "boundary command uninstall --project [--project-root PATH]",
		Common: []string{
			"boundary command uninstall --project",
			"boundary command uninstall --project --project-root /path/to/repo",
		},
		Notes: []string{
			"uninstall removes only Boundary-generated project shims.",
			"unrecognized files under .boundary/bin are left in place.",
		},
	})
	project := fs.Bool("project", false, "remove project-local shims under .boundary/bin")
	projectRoot := fs.String("project-root", "", "project root for shim removal; defaults to the current directory")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if !*project {
		fmt.Fprintln(stderr, "command uninstall: --project is required; global shim installation is not supported")
		return 1
	}
	root, err := resolveProjectRoot(*projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "command uninstall: %v\n", err)
		return 1
	}
	result, err := commandboundary.UninstallProjectShims(root, commandboundary.DefaultShimCommands())
	if err != nil {
		fmt.Fprintf(stderr, "command uninstall: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Removed %d project shims from %s\n", len(result.Removed), ".boundary/bin")
	if len(result.Skipped) > 0 {
		fmt.Fprintf(stdout, "Skipped %d missing or non-Boundary files\n", len(result.Skipped))
	}
	return 0
}

func resolveProjectRoot(projectRoot string) (string, error) {
	if projectRoot != "" {
		return projectRoot, nil
	}
	return os.Getwd()
}
