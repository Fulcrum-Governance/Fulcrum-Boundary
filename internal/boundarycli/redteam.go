package boundarycli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/redteam"
)

func runRedteam(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary redteam", stderr)
	packID := fs.String("pack", redteam.DefaultPackID, "redteam fixture pack to run")
	mode := fs.String("mode", redteam.ModeFixture, "redteam mode; only fixture is supported")
	format := fs.String("format", "text", "output format: text or json")
	list := fs.Bool("list", false, "list available redteam packs")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if *list {
		summaries := redteam.AvailablePacks()
		if strings.EqualFold(*format, "json") {
			if err := redteam.WriteJSON(stdout, summaries); err != nil {
				fmt.Fprintf(stderr, "redteam list: %v\n", err)
				return 1
			}
			return 0
		}
		if err := redteam.WritePackList(stdout, summaries); err != nil {
			fmt.Fprintf(stderr, "redteam list: %v\n", err)
			return 1
		}
		return 0
	}

	result, err := redteam.Run(context.Background(), redteam.RunOptions{
		PackID: *packID,
		Mode:   *mode,
	})
	if err != nil {
		fmt.Fprintf(stderr, "redteam: %v\n", err)
		return 1
	}
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "text", "":
		if err := redteam.WriteText(stdout, result); err != nil {
			fmt.Fprintf(stderr, "redteam: %v\n", err)
			return 1
		}
	case "json":
		if err := redteam.WriteJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "redteam: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "redteam: unsupported format %q\n", *format)
		return 1
	}
	if !result.Passed {
		return 1
	}
	return 0
}
