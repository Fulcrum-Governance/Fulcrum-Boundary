// Command fulcrum-demo-tool is the governed tool binary for the
// cli-reference-v1 bypass-resistance reference topology
// (tests/bypass/cli-reference). It exists only to show whether a process
// managed to execute it: every invocation prints a single marker line plus the
// caller's real and effective ids, so the harness can see which identity ran
// it. It reads nothing, writes only stdout, and holds no state.
//
// The marker string must stay in sync with cliMarker in
// tests/bypass/cli_reference_test.go.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("FULCRUM-DEMO-TOOL-MARKER uid=%d euid=%d gid=%d egid=%d\n",
		os.Getuid(), os.Geteuid(), os.Getgid(), os.Getegid())
}
