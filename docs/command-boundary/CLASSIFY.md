# Command Classification

`boundary command classify` classifies a command without executing it.

It produces a command class, risk level, recommended action, and reason for
later governance steps. Classification never executes the command.

## Usage

```bash
boundary command classify -- git status
boundary command classify -- git push origin main
boundary command classify -- rm -rf dist
boundary command classify -- cat .env
boundary command classify -- npm install
boundary command classify --json -- git push origin main
```

The `--` separates Boundary flags from the command being classified.

## Text Output

```text
Command: git push origin main
Class: C3 repo mutation
Risk: HIGH
Recommended action: require_approval
Reason: external repository mutation
```

## JSON Output

```json
{
  "schema_version": "boundary.command_classification.v1",
  "command": "git",
  "args_redacted": [
    "push",
    "origin",
    "main"
  ],
  "class": "C3",
  "risk": "HIGH",
  "recommended_action": "require_approval",
  "reason": "external repository mutation"
}
```

## Compound Command Lines

Passing a **single** argument that carries shell operators (`&&`, `||`, `;`, `|`,
`&`, a newline, a redirection, `$( … )`, backticks, or a `( … )` subshell)
classifies the line **per segment** instead of by its leading command:

```bash
boundary command classify -- 'git status && rm -rf dist'
```

```text
Command: rm -rf dist
Class: C4 destructive local mutation
Risk: CRITICAL
Recommended action: deny
Reason: destructive local mutation (most restrictive of 2 compound segments: rm -rf dist)
Parseable: true
Segments: 2
  1. [C0 observe/read | allow] git status
  2. [C4 destructive local mutation | deny] rm -rf dist
```

Passing an already-split argv (`-- git status`, `-- rm -rf dist`) is unchanged:
that form still classifies exactly one command.

The line is split on operators outside quotes. Command substitution, `( … )`
subshells, and `sh -c` string payloads are decomposed recursively to a fixed
depth cap; leading `NAME=VALUE` assignments and wrapper commands (`sudo`, `env`,
`exec`, `xargs`, …) are peeled so the command they hide is classified too. The
verdict is the **most restrictive** segment, ordered `deny` > `require_approval`
> `warn` > `allow`.

The JSON form of a compound line is a superset of the single-command payload —
the aggregate verdict stays at the top level, and `parseable`, `segments`, and
`line_schema_version` are added.

### Redirections are writes

An **output** redirection (`>`, `>>`, `>|`, `&>`, `&>>`) is a write the shell
performs, whatever the command in front of the operator does. The target gets its
own segment, with `origin: "redirect"`, so it cannot inherit the class of a read
command sitting in front of it:

```bash
boundary command classify -- 'cat notes.txt > important.db'
```

```text
Command: > important.db
Class: C1 local file write
Recommended action: warn
Segments: 2
  1. [C0 observe/read | allow] cat notes.txt
  2. [C1 local file write | warn] > important.db
```

A redirect target naming a secret-bearing path classifies `C6` instead. Nothing
stats the target, so whether the write truncates existing content, creates a new
file, or fails is not knowable here and is not asserted. **Input** redirections
(`<`, `<<<`) and descriptor duplications (`2>&1`) are not writes and are left in
the command's own arguments.

### Commands in argument position

A program named inside another command's **arguments** never appears as a command
word, so it would otherwise sit inert inside `args_redacted`. Two shapes are
decomposed and classified:

- `find`'s `-exec`, `-execdir`, `-ok`, and `-okdir`, up to the terminating `;` or
  `+` argument. So `find . -name '*.go' -exec rm -rf {} +` reports `C4` /`deny`,
  not `C0` for the search.
- `xargs`, whose payload is the argv after its flags — handled as a wrapper. So
  `git status --porcelain | xargs rm -rf` reports `C4` / `deny`.

This set is **not exhaustive**, and `parseable: true` does not claim otherwise:
see the limits below.

### When the line cannot be decomposed

The tokenizer models only the constructs it can decompose with confidence.
Heredocs, process substitution, `eval`, `source`, unbalanced quotes,
backslash-newline continuations, `$'…'` quoting, expansions that nest a
substitution, and nesting past the depth cap all set `parseable: false`.

A line that is not parseable reports `require_approval` with the reason
`compound command could not be safely decomposed`, and never `allow`. A segment
that *was* decomposed and classifies stricter still wins, so an undecomposed
remainder can raise the verdict but never lower it. Classification of a compound
line still never executes any part of it.

### What `parseable: true` does and does not claim

`parseable: true` says the tokenizer modelled every **construct** in the list
above. It does **not** say that every program the line can reach was classified.
An argument-position shape outside the two modelled above — `watch`, `parallel`,
`make`, `ssh`, an interpreter's inline `-e`/`-c` program, a path assembled at
runtime — leaves its payload unclassified while the line still reports
`parseable: true`, governed by the wrapper's own class. Treat the flag as a
statement about decomposition, not as a coverage claim.

## Redaction

Classification output redacts secret-looking arguments before printing or
encoding output. Redaction triggers include:

- `--token`
- `--api-key`
- `--password`
- `Authorization`
- `bearer`
- `secret`
- `.env` paths and values
- SSH private key paths
- inline URL credentials — `scheme://user:password@host` keeps the account name
  and replaces the password; `scheme://token@host` replaces the whole userinfo

Redaction preserves command shape while avoiding raw secret values. It is
pattern-based: it is not a guarantee that no secret-looking value survives in an
argument position no pattern covers.

## What It Proves

`boundary command classify` proves that Boundary can map routed command argv into
the Command Boundary taxonomy without executing the command.

By itself, classification does not prove:

- command execution governance;
- denial before execution;
- project shell routing;
- shim routing;
- global shell control;
- CI, cron, SSH, or editor task control.

Use `boundary command run`, `boundary shell`, and the project-local shim docs
for the routed execution paths.
