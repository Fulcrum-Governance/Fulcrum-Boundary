# Secure GitHub Live Bypass Model

Secure GitHub governs GitHub tool calls only when they route through the Secure
GitHub Boundary profile. Live conformance proves the governed route for the
configured test repository. It does not prove that a deployment has removed all
other GitHub access paths.

## In Scope

- `boundary secure github conformance read`
- `boundary secure github conformance denied-write`
- GitHub App installation auth held by the conformance process
- The configured repository and issue
- Secure GitHub's routed read-then-denied-write path

## Bypass Paths

These remain outside Boundary unless the operator removes or controls them:

- direct GitHub API calls with any token available to the agent;
- direct upstream GitHub MCP server access;
- SSH keys or deploy keys available to the agent;
- `gh`, `git`, or other command paths that are not routed through Boundary;
- CI jobs, cron jobs, editor tasks, or local scripts with independent GitHub
  credentials;
- browser sessions or OAuth flows outside the Secure GitHub profile.

## Production Gate

Do not mark Secure GitHub production until a deployment-specific bypass proof
shows that protected GitHub writes can only reach GitHub through Boundary, and
that the GitHub App credential is not available to the agent outside the
governed route.

