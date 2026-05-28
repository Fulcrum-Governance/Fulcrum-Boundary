# Edit Boundary Bypass Model

Edit Boundary governs file mutations only when the proposed mutation routes
through Boundary as an edit envelope.

Governed routes:

- `boundary edit inspect`
- `boundary edit apply`
- future integrations that submit the same edit envelope to Boundary before
  writing files

Bypass routes:

- direct editor save;
- direct shell redirection such as `cat > file`;
- direct `cp`, `mv`, `rm`, or `git apply`;
- IDE writes without a Boundary integration;
- language-server or formatter writes not routed through Boundary;
- CI jobs unless explicitly wrapped;
- remote SSH edits;
- arbitrary processes that write files directly.

Bypass does not mean the design failed. It defines the deployment boundary.
Production-grade edit governance would require the protected workflow to route
file mutations through Boundary and to prevent or monitor alternate write paths.

## Relationship To Git

Git remains the source of repository state. Edit Boundary is not a replacement
for Git permissions, branch protection, code review, or operating-system access
control.

Edit Boundary can provide pre-apply classification and decision records for
routed patch application. Direct `git apply`, `git checkout`, or editor writes
remain outside the route unless wrapped by a higher-level integration.

## Direct File Edit Gap

The direct file edit gap is explicit: an agent with a separate file-writing
capability can modify files without invoking `boundary edit apply`. That is a
future Filesystem/Edit Boundary integration problem, not something the initial
preview should hide with broad claims.
