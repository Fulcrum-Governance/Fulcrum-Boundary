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
 * The shim folds real/effective/saved gid onto the file's group via
 * setresgid, then execs the real boundary CLI. The child starts with
 * gid == egid == boundary_exec, the runtime drops nothing, and commands
 * executed under `boundary command run` hold the group needed to execve the
 * protected tool. Invoking the libexec binary directly confers no group, so
 * the shim remains the sole route.
 */
#define _GNU_SOURCE
#include <stdio.h>
#include <unistd.h>

#define BOUNDARY_REAL "/usr/local/boundary/libexec/boundary"

int main(int argc, char **argv)
{
	(void)argc;
	gid_t g = getegid();

	if (setresgid(g, g, g) != 0) {
		perror("boundary shim: setresgid");
		return 1;
	}
	execv(BOUNDARY_REAL, argv);
	perror("boundary shim: execv");
	return 127;
}
