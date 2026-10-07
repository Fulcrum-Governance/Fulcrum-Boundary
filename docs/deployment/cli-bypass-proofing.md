# CLI Bypass Proofing

The CLI adapter governs command execution only for commands routed through
the Boundary wrapper (`boundary command run`). A shell, an absolute path, an
interpreter, or any other direct route to the same binary is a bypass path
unless deployment topology removes it. Bypass resistance is therefore a
deployment property, not a property of the adapter code.

This page documents `cli-reference-v1`, a Fulcrum-owned reference topology
that removes the tested direct paths, and the automated harness that attacks
it. The topology and harness live under `tests/bypass/`.

## Reference Topology: cli-reference-v1

`tests/bypass/cli-reference/` builds a single agent container whose
protection comes from file permissions and group membership, not from hiding
shells:

- The governed tool is `fulcrum-demo-tool`, a purpose-built static binary
  that prints a marker line plus its real and effective ids. It is installed
  at `/opt/fulcrum/tools/fulcrum-demo-tool`, mode `0710 root:boundary_exec`:
  group `boundary_exec` may execute it, nobody may read it, and the path is
  not on `PATH`. A purpose-built binary is used because a general-purpose
  applet that also exists elsewhere in the image cannot serve as the
  protected tool.
- The only `boundary` on `PATH` is a setgid shim at
  `/usr/local/bin/boundary` (`2755 root:boundary_exec`). When argv is
  exactly `boundary command run ...` it folds the file's group into real,
  effective, and saved gid and then execs the real CLI at
  `/usr/local/boundary/libexec/boundary` (`0755 root:root`, not setgid), so
  commands a governed `command run` spawns run with `boundary_exec` as
  their real group and can execve the tool.
- The shim exists because the CLI is a Go binary: the Go runtime drops a
  raised egid at startup, so a setgid Go binary cannot carry a group into
  its children. The shim is the entry point; the CLI it execs is not
  setgid. For any argv other than `command run` — `boundary shell`,
  `version`, `command classify`, and anything else — the shim folds all
  three gids onto the caller's real gid instead, so those invocations run
  with no `boundary_exec` membership; a `boundary shell` subshell is
  therefore as unable to execve the tool as the agent's own `/bin/sh`
  (probe `shell-via-shim` asserts this). Invoking the libexec binary
  directly confers no group either. Within this topology, the only probed
  route that can execve the tool is `boundary command run` through the
  shim; every other tested route is denied.
- The agent user (`agent`, uid 100) is not a member of `boundary_exec`.
- The container drops all capabilities (`cap_drop: ALL`). This is required:
  `docker exec -u` performs the entry execve with the container's bounding
  set effective, and the default bounding set includes `DAC_OVERRIDE`, which
  would bypass the file-mode checks this topology relies on.
- `/opt/fulcrum/tools` is deliberately absent from `PATH`.

## Harness and Probes

The harness is the Go package `tests/bypass` behind the `bypass` build tag:

```bash
go test -tags=bypass -v ./tests/bypass/
```

It brings the topology up with `docker compose`, runs each probe through
`docker compose exec` with explicit argv as the `agent` user, and asserts on
real outcomes — exit codes, stdout/stderr, and the decision record file —
never strings the test echoed itself. Adversarial probes are blocked only
when every step exits non-zero, the tool's marker appears nowhere in the
captured output, and every step shows a platform denial indicator
(permission denied, not found, or an equivalent refusal): a step that
fails for an incidental reason does not count as a block.

| Probe | Attack | Expected outcome |
|---|---|---|
| direct-absolute-path | Exec the protected binary by absolute path | permission denied, no marker |
| path-lookup | Invoke the tool by name via `PATH` | not found, no marker |
| sh-c-absolute-path | `/bin/sh -c <absolute path>` | permission denied, no marker |
| interpreter-one-liner | busybox `awk` `system()` one-liner | permission denied, no marker |
| env-absolute-path | `env <absolute path>` | permission denied, no marker |
| copy-or-symlink | `cp` the binary to `/tmp`, then symlink and run it | copy denied (unreadable), run denied, no marker |
| su-or-sudo-escalation | `sudo` / `su root` to reach the tool | unavailable or denied, no marker |
| shell-via-shim | `boundary shell` subshell handed the tool path on stdin (with and without `--no-install`) | non-zero exit, permission denied, no marker, no decision record |
| governed-allow | `boundary command run -- find <tools dir> -name <tool> -exec <tool> ;` | exit 0, marker on stdout, decision record `action=allow executed=true` |
| governed-deny | `boundary command run -- <tool> --token=...` | non-zero exit, no marker, decision record `action=deny executed=false` |
| find-exec-as-agent | the governed-allow shape attempted directly | permission denied, no marker |
| exec-wrapper-applet | `nohup <absolute path>` | permission denied, no marker |

The governed-allow shape uses `find -exec` because the preview command
policy executes only allowlisted argv[0] names and `find` is the `C0`
(observe) entry that hands a path to execve. The marker line records the
executing group, so the evidence shows the tool ran with `boundary_exec` —
i.e. as a child of the wrapper route.

Each run writes `tests/bypass/evidence/cli-reference-v1.json` and
`cli-reference-v1.md`: topology id, image ids, the boundary version/commit
stamp, per-probe argv, exit codes, raw stdout/stderr, expected vs observed
outcomes, and a timestamp. The `evidence/` directory is gitignored; CI
uploads it as the `bypass-evidence-cli-reference-v1` artifact from the
`bypass probes (cli-reference-v1)` job.

Docker is required. Without it the test skips locally; with `CI=true` or
`REQUIRE_DOCKER=1` it fails rather than skips.

## Evidence Level and Scope

The Secure GitHub bypass ladder defines its L2 level as "Managed deployment
topology attests every direct path is denied." This harness substitutes
mechanical observation for attestation: every direct-path probe above was
observed denied inside `cli-reference-v1`, with outcomes recorded in the
evidence artifact. That satisfies the direct-path-denial control *for this
topology*; it is not an L2 attestation about a managed production
deployment, which is what the level requires.

The harness observed these probes blocked in `cli-reference-v1` under the
listed conditions; this is evidence for that topology only and not a
guarantee about any other deployment.

## Non-Claims

This evidence does not cover:

- deployments whose agent can reach the governed tool by any path not listed
  here, or where the agent holds `boundary_exec` membership or a bounding
  capability set;
- CLI invocations that never touch the governed tool (the wrapper governs
  routed commands; it does not confine the agent's shell);
- adapter status. The CLI adapter remains `preview`; this evidence feeds the
  `BND-CLI-002` gap but changes nothing in `adapters/cli/readiness.yaml`;
- other transports. Webhook, gRPC, and A2A reference topologies are tracked
  separately; the harness takes a new compose directory plus a probe list
  per topology.
