package boundarycli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/selftest"
)

func runSelftest(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary selftest", stderr, commandHelp{
		Purpose: "Run local no-credential Boundary release checks.",
		Usage:   "boundary selftest [--json] [--no-color]",
		Common: []string{
			"boundary selftest",
			"boundary selftest --json",
		},
		Notes: []string{
			"Selftest uses fixtures and local checks only; it does not use credentials, network, or live mutation.",
			"Passing selftest is a first-run signal, not live deployment conformance.",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	noColor := fs.Bool("no-color", false, "disable ANSI color in text output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	result, err := selftest.Run(context.Background(), selftest.Options{
		NoColor: *noColor,
		SecureGitHubLiveModeCheck: func(context.Context) error {
			var out bytes.Buffer
			var errOut bytes.Buffer
			code := runSecureGitHubServe([]string{"--fixture=false", "--dry-run"}, &out, &errOut)
			if code == 0 {
				return fmt.Errorf("secure github live mode unexpectedly succeeded")
			}
			if !strings.Contains(errOut.String(), "live GitHub App mode is not implemented") {
				return fmt.Errorf("unexpected secure github live-mode error: %s", strings.TrimSpace(errOut.String()))
			}
			return nil
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "selftest: %v\n", err)
		return 1
	}
	if *jsonOutput {
		if err := selftest.WriteJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "selftest: %v\n", err)
			return 1
		}
	} else if err := selftest.WriteText(stdout, result, selftest.RenderOptions{NoColor: *noColor}); err != nil {
		fmt.Fprintf(stderr, "selftest: %v\n", err)
		return 1
	}
	if !result.Passed {
		return 1
	}
	return 0
}
