# Edit Taxonomy

Edit Boundary classifies proposed file mutations before applying them. The
classification is conservative: the highest-risk target or hunk determines the
overall action.

| Class | Meaning | Examples | Default |
|---|---|---|---|
| E0 | metadata/no-op | empty diff, whitespace-only doc metadata | allow |
| E1 | safe content edit | README copy, docs pages, test fixtures with no secrets | allow |
| E2 | source/config mutation | Go source, JSON config, YAML config, policy examples | require approval |
| E3 | deployment/infrastructure mutation | Terraform, Kubernetes YAML, Helm charts, cloud config | require approval |
| E4 | secret-bearing edit | `.env`, private keys, tokens, credential files, raw secret additions | deny |
| E5 | destructive deletion or broad rewrite | mass delete, `DELETE` patches, large rewrites, path wipes | deny |
| E6 | execution behavior mutation | package scripts, CI workflows, Dockerfiles, Makefiles, shell scripts, hooks | require approval |
| E7 | outside project scope | absolute path, traversal, symlink escape, `.git/hooks/*` | deny |

## Precedence

When a patch has multiple classes, the highest-risk class wins:

```text
E7 > E5 > E4 > E6 > E3 > E2 > E1 > E0
```

E4, E5, and E7 are hard-deny classes. Approval must not override them.

## Path Signals

Examples of path-based signals:

| Signal | Class | Reason |
|---|---|---|
| `README.md` | E1 | documentation-only edit |
| `docs/**/*.md` | E1 | documentation-only edit |
| `*.go` | E2 | source change |
| `config/*.yaml` | E2 | config change |
| `.github/workflows/*.yml` | E6 | execution behavior change |
| `go.mod` or `go.sum` | E2 | dependency and module graph change |
| `.env` | E4 | credential-bearing path |
| `.ssh/id_rsa` | E4 | credential-bearing path |
| `terraform/**/*.tf` | E3 | infrastructure mutation |
| `scripts/deploy.sh` | E6 | execution behavior change |
| `.git/hooks/pre-commit` | E7 | repository control path |
| `../outside.txt` | E7 | outside project scope |

## Content Signals

Patch content can raise the class even when the path looks safe.

Examples that should raise to E4:

- private key blocks;
- bearer tokens;
- GitHub tokens;
- `API_KEY=...`;
- `password=...`;
- URL credentials;
- unredacted `.npmrc` or `.pypirc` credentials.

Examples that should raise to E5:

- broad deletion of many files;
- deletion of security policy files;
- large replacement hunks that exceed configured thresholds.

## Unsupported Patch Forms

Unsupported forms must fail closed or classify to require approval or deny.
Initial v0.6 implementation should be conservative around binary patches,
submodules, mode changes, combined diffs, duplicate file headers, malformed
hunks, and ambiguous rename/copy behavior.
