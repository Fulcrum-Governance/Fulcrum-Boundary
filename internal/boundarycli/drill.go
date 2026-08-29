package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// drillDirName is the scratch directory /boundary:drill creates, always
// resolved relative to the working directory. It is a sibling of the real
// .boundary evidence tree on purpose — see the drill skill's safety note.
const drillDirName = ".boundary-drill"

// drillFixtureRelPath is the one file the drill writes, relative to
// drillDirName. Cleanup removes exactly this file and the directories that
// held it, and nothing else.
var drillFixtureRelPath = filepath.Join("vault", "fixture.txt")

func runDrill(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printDrillHelp(stdout)
		return 0
	}
	switch args[0] {
	case "cleanup":
		return runDrillCleanup(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown drill subcommand %q\n\n", args[0])
		printDrillHelp(stderr)
		return 1
	}
}

func printDrillHelp(w io.Writer) {
	fmt.Fprintf(w, `Boundary drill utilities

Purpose:
  Support /boundary:drill, the first-run proof that the PreToolUse hook denies
  a destructive command and records the denial.

Usage:
  boundary drill <subcommand>

Commands:
  cleanup         Remove the drill's own fixture (%s) and nothing else

Notes:
  - cleanup is scoped by construction: it deletes exactly %s,
    then removes the emptied directories with rmdir semantics. Anything else
    found under %s is listed and left in place.
  - It never follows a symlink and never does a recursive delete.
`, filepath.Join(drillDirName, drillFixtureRelPath), filepath.Join(drillDirName, drillFixtureRelPath), drillDirName)
}

// runDrillCleanup removes the drill fixture, and only the drill fixture.
//
// The contract is deliberately narrower than `rm -rf .boundary-drill`, which
// Command Boundary correctly denies as a C4 destructive mutation: this verb
// deletes the exact fixture path the drill wrote, removes the two directories
// that held it only if that emptied them (os.Remove has rmdir semantics on a
// directory), refuses symlinks outright, and leaves anything it does not
// recognize exactly where it found it — reported, not removed. A cleanup that
// found unexpected content exits 1 so a script can tell "cleaned" from
// "declined to clean".
func runDrillCleanup(args []string, stdout, stderr io.Writer) int {
	flags := newHelpFlagSet("boundary drill cleanup", stderr, commandHelp{
		Purpose: "Remove the /boundary:drill fixture — and only the fixture — leaving no " + drillDirName + " residue.",
		Usage:   "boundary drill cleanup",
		Common: []string{
			"boundary drill cleanup",
		},
		Notes: []string{
			"Deletes exactly " + filepath.Join(drillDirName, drillFixtureRelPath) + ", then removes the emptied directories; nothing recursive, nothing forced.",
			"A symlinked " + drillDirName + " or vault/ is refused rather than followed.",
			"Content this cleanup does not recognize is listed and left in place, and the exit code is 1 so the outcome is visible.",
			"Running it with no " + drillDirName + " present is a no-op that exits 0.",
			"Under a live-wired hook this command classifies C1 (local file write) and warns: a deleting command stays visible, even Boundary's own.",
		},
	})
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if flags.NArg() > 0 {
		fmt.Fprintln(stderr, "drill cleanup: unexpected argument; this verb takes none and removes only "+filepath.Join(drillDirName, drillFixtureRelPath))
		return 1
	}

	info, err := os.Lstat(drillDirName)
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(stdout, "drill cleanup: nothing to remove (no "+drillDirName+" in this directory)")
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "drill cleanup: %v\n", err)
		return 1
	}
	if info.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintln(stderr, "drill cleanup: "+drillDirName+" is a symlink; refusing to follow it")
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintln(stderr, "drill cleanup: "+drillDirName+" is not a directory; refusing to touch it")
		return 1
	}

	vaultDir := filepath.Join(drillDirName, filepath.Dir(drillFixtureRelPath))
	if vaultInfo, vaultErr := os.Lstat(vaultDir); vaultErr == nil && vaultInfo.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintln(stderr, "drill cleanup: "+vaultDir+" is a symlink; refusing to follow it")
		return 1
	}

	fixture := filepath.Join(drillDirName, drillFixtureRelPath)
	if _, fixtureErr := os.Lstat(fixture); fixtureErr == nil {
		if removeErr := os.Remove(fixture); removeErr != nil {
			fmt.Fprintf(stderr, "drill cleanup: remove %s: %v\n", fixture, removeErr)
			return 1
		}
	}
	// rmdir semantics: os.Remove on a directory fails unless it is empty, so
	// a directory holding anything this cleanup did not delete stays put.
	_ = os.Remove(vaultDir)
	_ = os.Remove(drillDirName)

	if _, err := os.Lstat(drillDirName); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(stdout, "drill cleanup: removed "+drillDirName+" (the drill's fixture and nothing else)")
		return 0
	}

	leftover := listDrillLeftovers()
	fmt.Fprintln(stderr, "drill cleanup: left "+drillDirName+" in place — it holds content this cleanup does not own: "+
		leftover+". Remove it yourself if you created it.")
	return 1
}

// listDrillLeftovers names what remains under the drill directory, relative to
// it, sorted, so the refusal message says exactly what was left and where.
func listDrillLeftovers() string {
	var names []string
	_ = filepath.WalkDir(drillDirName, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == drillDirName {
			return nil
		}
		rel, relErr := filepath.Rel(drillDirName, path)
		if relErr != nil {
			return nil
		}
		if entry.IsDir() {
			rel += string(os.PathSeparator)
		}
		names = append(names, rel)
		return nil
	})
	if len(names) == 0 {
		return "(unreadable)"
	}
	sort.Strings(names)
	return quoteJoin(names)
}

// quoteJoin renders a short quoted, comma-separated list.
func quoteJoin(names []string) string {
	out := ""
	for i, name := range names {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%q", name)
	}
	return out
}
