# Relation To Command Boundary

Command Boundary and Edit Boundary cover different routed action paths.

Command Boundary governs commands when they route through:

- `boundary command run`;
- `boundary shell`;
- project-local shims.

Edit Boundary governs proposed file mutations when they route through:

- `boundary edit inspect`;
- `boundary edit apply`;
- future edit-envelope integrations.

Both surfaces should use the shared governance pipeline and decision-record
discipline. They must not fork separate policy semantics.

## Gap Between Them

Command Boundary can deny a routed command such as `rm -rf dist`. It does not
see a direct editor write unless that write is represented as a governed command
or edit envelope.

Edit Boundary can deny a routed patch that writes `.env`. It does not see a
direct shell command or direct editor save unless the workflow routes that
mutation through Boundary.

Together, they move Boundary toward broader coding-agent action governance, but
both remain route-scoped preview surfaces.
