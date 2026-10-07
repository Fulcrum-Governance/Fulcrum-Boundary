# cli-reference-v1: reference topology for CLI adapter bypass-resistance
# evidence (docs/deployment/cli-bypass-proofing.md).
#
# Protection model: the governed tool is executable only by group
# boundary_exec and readable by no one but root. The sole `boundary` on the
# agent's PATH is a setgid boundary_exec shim that folds the group into
# real/effective/saved gid and execs the real boundary CLI — but only when
# argv is `command run`; every other argv runs with the caller's real gid —
# so inside this topology the only probed route that can execve the tool is
# a governed `boundary command run` through the shim (or root); every other
# tested route is denied. The unprivileged "agent" user is not a member of
# boundary_exec, and the agent container drops all capabilities.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG BOUNDARY_COMMIT=unknown
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-X github.com/fulcrum-governance/fulcrum-boundary/internal/versioninfo.Commit=${BOUNDARY_COMMIT}" \
      -o /out/boundary ./cmd/boundary \
 && CGO_ENABLED=0 go build -trimpath -o /out/fulcrum-demo-tool ./tests/bypass/cli-reference/demotool
# The wrapper shim must not be Go: the Go runtime drops a raised egid at
# startup, which would strip the boundary_exec group this topology relies on.
RUN apk add --no-cache gcc musl-dev \
 && cc -O2 -static -o /out/boundary-shim ./tests/bypass/cli-reference/shim/main.c

FROM alpine:3.21
RUN addgroup -S boundary_exec \
 && adduser -S -D -h /home/agent -s /bin/sh agent \
 && install -d -o root -g boundary_exec -m 0750 /opt/fulcrum/tools \
 && install -d -o root -g root -m 0755 /usr/local/boundary/libexec

# Governed tool: root-owned, group boundary_exec, execute-only (0710).
# Execute-only — not 0750 — so group members can execve it but cannot read
# it: a routed file-read or copy of the binary is denied as firmly as an
# ungoverned one.
COPY --from=build --chown=root:boundary_exec --chmod=0710 \
  /out/fulcrum-demo-tool /opt/fulcrum/tools/fulcrum-demo-tool

# Real boundary CLI: world-executable but not setgid (a Go binary cannot
# carry a raised egid anyway). Running it directly confers no group, so the
# libexec binary alone cannot reach the governed tool.
COPY --from=build --chown=root:root --chmod=0755 \
  /out/boundary /usr/local/boundary/libexec/boundary

# The wrapper entry point on PATH: setgid boundary_exec. The shim raises the
# group only for `command run` argv — `boundary shell`, `version`, and every
# other subcommand exec with the caller's real gid — so inside this topology
# only governed `command run` children hold boundary_exec.
COPY --from=build --chown=root:boundary_exec --chmod=2755 \
  /out/boundary-shim /usr/local/bin/boundary

# The tools directory is deliberately absent from PATH.
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

USER root
ENTRYPOINT ["/bin/sleep", "infinity"]
