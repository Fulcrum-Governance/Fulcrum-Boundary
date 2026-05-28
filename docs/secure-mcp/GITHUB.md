# Secure GitHub MCP

Secure GitHub MCP is a preview Boundary profile for governing a small GitHub
MCP tool set before forwarding. The default release path is a fixture proof: it
demonstrates that untrusted GitHub content can taint an envelope and that a
later protected private-repo mutation is denied before a GitHub call is made.

The live conformance preview adds an opt-in GitHub App path. With
operator-owned credentials, Boundary can read real GitHub context, mark the
session tainted, and deny a protected write-after-taint action before any
upstream GitHub mutation client call executes.

Secure GitHub remains preview until deployment bypass proof is recorded.

## Status

| Field | Value |
|---|---|
| Profile ID | `secure-github` |
| Maturity | `preview` |
| Evidence mode | fixture by default; opt-in live conformance harness |
| Live GitHub mutation | none by default; denied-write conformance must report `github_mutation_called=false` |
| Production gate | deployment bypass proof plus operator deployment evidence |

## Commands

Write the fixture profile and starter policy bundle:

```bash
boundary secure github setup --out .boundary/secure-github
```

Inspect the serve configuration without starting a listener:

```bash
boundary secure github serve --fixture --dry-run
```

Run the fixture JSON-RPC HTTP profile:

```bash
boundary secure github serve --fixture --listen 127.0.0.1:8940
```

Run opt-in live conformance checks:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance read
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance denied-write
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance all --out /tmp/boundary-secure-github
```

Without `BOUNDARY_GITHUB_CONFORMANCE=true`, conformance commands skip without
network calls. `boundary secure github serve --fixture=false` still fails closed;
live serving is not part of this preview.

## MVP Tool Set

| Tool | Class | Source | Sink | Mutation |
|---|---|---|---|---|
| `get_issue` | `R0` | `external_collaborator` or `allowlisted_resource` | `none` | `none` |
| `get_pull_request` | `R0` | `external_collaborator` or `allowlisted_resource` | `none` | `none` |
| `get_file_contents` | `R0` | `public_resource` or `allowlisted_resource` | `none` | `none` |
| `create_issue` | `W0` | `agent_generated` | `private_repo` or `public_repo` | `issue_or_pr_create` |
| `create_pull_request` | `W0` | `agent_generated` | `private_repo` or `public_repo` | `issue_or_pr_create` |
| `create_or_update_file` | `W1` | `agent_generated` | `private_repo` | `private_repo_content_write` |
| `push_files` | `W1` | `agent_generated` | `private_repo` | `private_repo_content_write` |
| `merge_pull_request` | `W2` | `agent_generated` | `private_repo` | `merge_or_release` |

Unsupported tools return an MCP-shaped unsupported error and do not forward.

## Envelope Model

The adapter binds each request to a GitHub execution envelope:

- `profile_id`
- `profile_status`
- `session_id`
- `request_id`
- `envelope_id`
- `tenant_id`
- `agent_id`
- `tool`
- `target_repo`
- `capability_class`
- `source_class`
- `target_sink`
- `mutation_class`
- `tainted`
- `taint_source`
- `one_repo_per_session`
- `repo_scope_violation`

The fixture collaborator model treats external issue or pull request content as
untrusted unless the request marks the actor as owner, member, or collaborator.
When such content is read, the session records taint. A later `W1` or `W2`
private-repo mutation sees that taint and is denied before upstream execution.

## Policies

The preview profile ships three in-process static rules:

- deny one-repo-per-session violations
- deny `W1` private-repo mutations after taint
- deny `W2` private-repo mutations after taint

The primary fixture rule is `deny-github-write-after-taint-fixture`. It denies
`create_or_update_file` and `push_files` class writes after external GitHub
taint. The critical mutation rule denies `merge_pull_request` class writes
after taint.

## Denial Shape

Denied calls return JSON-RPC error code `-32001`:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32001,
    "message": "Boundary denied GitHub tool call",
    "data": {
      "profile_id": "secure-github",
      "profile_status": "preview",
      "action": "deny",
      "target_repo": "fixture-org/fixture-private-repo",
      "taint_sources": ["github.issue_body"],
      "target_sink": "private_repo",
      "capability_class": "W1",
      "mutation_class": "private_repo_content_write",
      "upstream_called": false
    }
  }
}
```

The data object also carries the decision record for fixture evidence.

## Limitations

- The default profile is fixture-backed and does not call GitHub.
- The opt-in conformance harness calls GitHub only when explicitly enabled with
  operator-owned GitHub App credentials.
- Live conformance proves the configured read and denied-write no-mutation path;
  it does not prove deployment bypass resistance.
- One-repo-per-session enforcement is in-memory for the preview fixture.
- Direct GitHub API calls or direct upstream GitHub MCP calls are bypasses unless
  deployment topology removes those paths.
- The profile governs the MVP tool set above, not the full GitHub MCP tool
  catalog.

## Live Conformance Docs

- [GitHub App auth](./GITHUB_APP_AUTH.md)
- [GitHub App permissions](./GITHUB_APP_PERMISSIONS.md)
- [Live conformance](./GITHUB_LIVE_CONFORMANCE.md)
- [Live read conformance](./GITHUB_LIVE_READ_CONFORMANCE.md)
- [Live denied-write conformance](./GITHUB_LIVE_DENIED_WRITE_CONFORMANCE.md)
- [Live bypass model](./GITHUB_LIVE_BYPASS_MODEL.md)
- [Live evidence](./GITHUB_LIVE_EVIDENCE.md)
