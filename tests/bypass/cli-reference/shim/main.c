/*
 * boundary shim for the cli-reference-v1 bypass-resistance reference
 * topology (tests/bypass/cli-reference).
 *
 * This binary is installed setgid boundary_exec as /usr/local/bin/boundary
 * (the only `boundary` on the agent's PATH). It exists because the boundary
 * CLI is a Go binary: the Go runtime detects a raised egid at startup
 * (getgid() != getegid()) and drops it, so a setgid Go binary cannot carry
 * the boundary_exec group into the governed commands it spawns.
 *
 * Elevation is gated on the governed route. When argv is exactly
 * `boundary command run ...` the shim folds real/effective/saved gid onto
 * the file's group via setresgid; the exec'd CLI starts with
 * gid == egid == boundary_exec and the commands `command run` spawns hold
 * the group needed to execve the protected tool. For every other argv —
 * `boundary shell`, `version`, `command classify`, anything else — the
 * shim instead folds all three gids onto the caller's real gid, so the CLI
 * runs with no boundary_exec membership at all: a `boundary shell`
 * subshell can no more execve the tool than the agent's own /bin/sh can,
 * which is exactly what the shell-via-shim probe asserts. Invoking the
 * libexec binary directly likewise confers no group. Inside this topology
 * the only probed route that can execve the tool is `boundary command run`
 * through this shim; every tested alternative is denied.
 */
#define _GNU_SOURCE
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#define BOUNDARY_REAL "/usr/local/boundary/libexec/boundary"

int main(int argc, char **argv)
{
	gid_t g;

	if (argc >= 3 && strcmp(argv[1], "command") == 0 && strcmp(argv[2], "run") == 0) {
		/* Governed route: raise boundary_exec into r/e/s gid. */
		g = getegid();
	} else {
		/* Any other subcommand runs with the caller's real gid only.
		 * setresgid can map all three gids onto the real gid because the
		 * raised egid authorizes the change; the saved gid is dropped too,
		 * so the group cannot be regained after exec. */
		g = getgid();
	}
	if (setresgid(g, g, g) != 0) {
		perror("boundary shim: setresgid");
		return 1;
	}
	execv(BOUNDARY_REAL, argv);
	perror("boundary shim: execv");
	return 127;
}
