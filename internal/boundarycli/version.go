package boundarycli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/versioninfo"
)

func runVersion(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary version", stderr, commandHelp{
		Purpose: "Print Boundary version and build metadata.",
		Usage:   "boundary version [--json]",
		Common: []string{
			"boundary version",
			"boundary version --json",
		},
		Notes: []string{
			"Missing build metadata is reported as unknown instead of failing.",
			"Release builds can set Version, Commit, and BuildDate with ldflags.",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "version: unexpected argument %q\n", fs.Arg(0))
		return 1
	}

	info := currentVersionInfo()
	if *jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			fmt.Fprintf(stderr, "version: %v\n", err)
			return 1
		}
		return 0
	}
	writeVersion(stdout, info)
	return 0
}

func currentVersionInfo() versioninfo.Info {
	return versioninfo.Current(versioninfo.Metadata{Version: Version})
}

func currentGatewayVersion() string {
	return currentVersionInfo().Version
}

func writeVersion(w io.Writer, info versioninfo.Info) {
	fmt.Fprintf(w, "Fulcrum Boundary %s\n", info.Version)
	fmt.Fprintf(w, "commit: %s\n", info.Commit)
	fmt.Fprintf(w, "build_date: %s\n", info.BuildDate)
	fmt.Fprintf(w, "go: %s\n", info.GoVersion)
	fmt.Fprintf(w, "module: %s\n", info.Module)
}
