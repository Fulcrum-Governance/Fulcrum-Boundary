# MCP Firewall Redteam Fixtures

Boundary redteam packs are safe fixture attacks for checking expected boundary
behavior. They run through the Boundary governance pipeline, emit decision
records, and avoid live upstream systems by default.

## Command

Run the default fixture pack:

```bash
boundary redteam
```

Emit machine-readable output:

```bash
boundary redteam --format json
```

List available packs:

```bash
boundary redteam --list
```

`fixture` is the only supported mode in this release. The command rejects live
mode instead of falling through to real services.

## Safety Boundary

Fixture mode:

- uses no real secrets
- performs no live upstream API calls
- mutates no real repository, database, filesystem, or messaging system
- uses synthetic tenant, agent, repository, branch, path, and payload values
- emits a local decision record for each scenario

The fixtures prove the tested request projection and policy path. They do not
prove live GitHub App conformance, full Secure GitHub production readiness, or
coverage of every prompt-injection attack.

## GitHub Lethal Trifecta

The implemented `github-lethal-trifecta` pack models the first Secure GitHub
proof path:

1. External GitHub issue content enters the agent context.
2. The envelope is marked with `tainted=true` and `taint_source=github.issue_body`.
3. The agent attempts a protected private-repo file mutation.
4. Boundary evaluates the request before any upstream GitHub call.
5. The fixture policy denies the write-after-taint path.
6. A decision record is emitted with the matched rule, reason, action, and
   decision hash.

The fixture request uses:

| Field | Value |
|---|---|
| `transport` | `mcp` |
| `tool_name` | `github.create_or_update_file` |
| `source_class` | `external_collaborator_content` |
| `target_sink` | `private_repo` |
| `mutation_class` | `private_repo_content_write` |
| `capability_class` | `W1` |
| `risk_class` | `W1` |

Expected output includes:

```text
redteam mode: fixture
pack: github-lethal-trifecta
live mutation: none
real secrets: none
scenario: github-write-after-taint
expected: DENY
actual: DENY
result: pass
decision record: rec_<hash-prefix>
decision hash: sha256:<hash>
```

## Pack Catalog

Implemented:

| Pack | Purpose |
|---|---|
| `github-lethal-trifecta` | Demonstrates private-repo write denial after untrusted GitHub content taints the envelope. |

Reserved stubs:

| Pack | Purpose |
|---|---|
| `secrets-exfil` | Secret-like value movement to an external sink. |
| `tool-poisoning` | Untrusted tool output influencing a later privileged action. |
| `rug-pull` | Descriptor or tool-surface changes that alter agent assumptions. |
| `postgres-destruction` | Destructive database actions. |
| `github-pr-exfil` | Private content moved to a pull request or review surface. |
| `filesystem-credential-read` | Local credential material read through filesystem tools. |
| `slack-exfil` | Private context published to a messaging system. |

Stub packs are listed so the redteam surface has a stable catalog, but running a
stub pack returns an explicit unavailable error until fixture scenarios are
implemented.

## Claim Boundary

Boundary runs fixture redteam packs for expected deny outcomes without real
secrets or live mutation. The GitHub lethal-trifecta fixture demonstrates
write-after-taint denial before upstream execution in fixture mode.

Do not describe fixture success as live exploit conformance.
Do not describe fixture success as universal MCP attack prevention.
Do not describe fixture success as production GitHub security.
Do not describe fixture success as proof that every dangerous tool path is
covered.
