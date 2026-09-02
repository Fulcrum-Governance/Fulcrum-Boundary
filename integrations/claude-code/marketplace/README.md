# boundary-plugins

Claude Code plugin marketplace for
[Fulcrum Boundary](https://github.com/Fulcrum-Governance/Fulcrum-Boundary).
The marketplace lists the `boundary` plugin and resolves its source from the
Fulcrum Boundary repository.

Boundary evaluates tool calls delivered through a configured Claude Code
`PreToolUse` hook before execution. It can deny a routed call before the tool
runs and writes hash-verifiable session receipts. Boundary governs only routed
calls; direct or bypass routes remain outside it. Receipt verification proves
covered-field hash integrity, not authenticity, correctness, execution, or
total coverage.

## Install

Add this marketplace and install the plugin from Claude Code:

```
/plugin marketplace add fulcrum-governance/boundary-plugins
/plugin install boundary@boundary-plugins
```

The plugin source is pinned to the immutable v0.13.1 release commit.

For setup, behavior, and limitations, use the immutable v0.13.1 source
documentation:

- [Claude Code hook guide](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/v0.13.1/docs/integrations/CLAUDE_CODE_HOOK.md)
- [Direct-install script](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/v0.13.1/scripts/install-claude-code.sh)

## Contents

| Path | Purpose |
| --- | --- |
| `.claude-plugin/marketplace.json` | The marketplace manifest: one `boundary` entry sourced from the Fulcrum Boundary repository. |
| `README.md` | Installation, proof-boundary, and update guidance for this marketplace. |

## Update contract

Before publishing an update, keep `plugins[0].name`, `description`, `homepage`,
`license`, and `keywords` aligned with the
[canonical plugin manifest](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/.claude-plugin/plugin.json).
Keep `plugins[0].source.ref` and `source.sha` pinned to that version's immutable
release tag and commit. The Fulcrum Boundary test suite enforces both contracts
for the retained scaffold.
