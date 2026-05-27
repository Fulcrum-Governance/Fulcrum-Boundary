package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
)

func runShell(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary shell", stderr, commandHelp{
		Purpose: "Launch a project-local Command Boundary subshell.",
		Usage:   "boundary shell [--project-root PATH] [--no-install] [--print-env]",
		Common: []string{
			"boundary shell",
			"boundary shell --project-root /path/to/repo",
		},
		Notes: []string{
			"shell prepends .boundary/bin to PATH for the subshell only.",
			"shell does not edit global shell startup files or global PATH.",
			"commands without project-local shims remain outside Boundary.",
		},
	})
	projectRoot := fs.String("project-root", "", "project root for the subshell; defaults to the current directory")
	noInstall := fs.Bool("no-install", false, "do not create missing project shims before launching")
	printEnv := fs.Bool("print-env", false, "print the project shell environment instead of launching a shell")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	root, err := resolveProjectRoot(*projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "shell: %v\n", err)
		return 1
	}
	if !*noInstall {
		if _, err := commandboundary.InstallProjectShims(root, commandboundary.DefaultShimCommands()); err != nil {
			fmt.Fprintf(stderr, "shell: install project shims: %v\n", err)
			return 1
		}
	}
	banner, err := commandboundary.ShellBanner(root)
	if err != nil {
		fmt.Fprintf(stderr, "shell: %v\n", err)
		return 1
	}
	if *printEnv {
		preview, err := commandboundary.ShellPreview(root, os.Environ())
		if err != nil {
			fmt.Fprintf(stderr, "shell: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, banner)
		fmt.Fprint(stdout, preview)
		return 0
	}

	env, err := commandboundary.ShellEnvironment(root, os.Environ())
	if err != nil {
		fmt.Fprintf(stderr, "shell: %v\n", err)
		return 1
	}
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}
	fmt.Fprintln(stderr, banner)
	cmd := exec.Command(shellPath) // #nosec G204 G702 -- launches the operator's configured interactive shell with project-local env only.
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return shellExitCode(err)
	}
	return 0
}

func shellExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
