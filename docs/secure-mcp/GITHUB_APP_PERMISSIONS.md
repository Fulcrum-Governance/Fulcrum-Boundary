# Secure GitHub App Permissions

Use an isolated GitHub App installation and a dedicated test repository for
Secure GitHub live conformance.

## Minimal Read Conformance

The live-read path requires:

| Permission | Access | Reason |
|---|---|---|
| Metadata | read | Required by GitHub for repository access. |
| Issues | read | Read the configured issue and hash its title/body for taint evidence. |

The read conformance path does not require write permissions.

## Denied Write-After-Taint Conformance

The denied-write path proves that Boundary denies the protected mutation before
the GitHub mutation client is reached. It can run in two safe configurations:

| Configuration | Permissions | What it proves |
|---|---|---|
| Conservative | Metadata read, Issues read | Boundary denies before a mutation call, independent of whether GitHub would authorize the mutation. |
| Strong isolated test | Metadata read, Issues read, Contents write on a disposable test repository | Boundary denies before a mutation call even when the GitHub App would otherwise be capable of writing repository contents. |

Use the strong configuration only on an isolated repository created for
conformance. Do not run first-time conformance against production repositories.

## Not Required

Do not grant broad organization administration, secrets, workflow, Actions
administration, deployments, or pull-request write permissions for this preview
conformance path.

## Production Gate

Permissions alone do not make Secure GitHub production. Production status still
requires deployment bypass proof that agents cannot reach direct GitHub API
credentials, direct upstream GitHub MCP servers, SSH keys, or other credentialed
GitHub write paths outside Boundary.

