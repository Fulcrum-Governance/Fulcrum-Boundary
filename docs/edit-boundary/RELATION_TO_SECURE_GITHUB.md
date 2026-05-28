# Relation To Secure GitHub

Secure GitHub and Edit Boundary address different mutation points.

Secure GitHub is an MCP profile that governs routed GitHub tool calls. Its
preview evidence focuses on write-after-taint denial before GitHub mutation and
no-mutation proof for denied writes.

Edit Boundary governs proposed local file mutations before they are applied to a
project worktree.

## Shared Discipline

Edit Boundary should reuse the proof posture established by Secure GitHub:

- deny before mutation;
- record the decision;
- store no raw secrets in evidence;
- distinguish fixture proof from live conformance;
- preserve deployment bypass caveats.

## No Claim Expansion

Adding Edit Boundary must not broaden Secure GitHub claims. Secure GitHub remains
preview and opt-in. Production GitHub protection still requires deployment
bypass evidence and operator-owned live evidence where applicable.
