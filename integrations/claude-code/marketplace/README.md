# boundary-plugins (scaffold)

This directory is the ready-to-push content for a future, standalone
`fulcrum-governance/boundary-plugins` repository: a Claude Code plugin
marketplace listing the `boundary` plugin. No remote repository has been
created; nothing here has been pushed anywhere. This is staged content only.

At launch, the intended flow is:

1. Create the `fulcrum-governance/boundary-plugins` GitHub repository.
2. Push the contents of this directory (`.claude-plugin/marketplace.json` and
   this `README.md`) as its initial commit.
3. Users add it as a marketplace and install from it:

   ```
   /plugin marketplace add fulcrum-governance/boundary-plugins
   /plugin install boundary@boundary-plugins
   ```

Until that repository exists, install the plugin directly from this
(`fulcrum-governance/fulcrum-boundary`) repository instead; see
[`docs/integrations/CLAUDE_CODE_HOOK.md`](../../docs/integrations/CLAUDE_CODE_HOOK.md)
and [`scripts/install-claude-code.sh`](../../scripts/install-claude-code.sh).

## Contents

| Path | Purpose |
| --- | --- |
| `.claude-plugin/marketplace.json` | The marketplace manifest: one entry, `boundary`, sourced from this GitHub repository. |
| `README.md` | This file. |

## Keeping this in sync

The `plugins[0].version`, `description`, and `keywords` fields here should
track [`/.claude-plugin/plugin.json`](../../.claude-plugin/plugin.json) at the
repository root: that file is the plugin's own manifest and is the source of
truth. This scaffold does not run any check that keeps the two in sync; that is
a manual step until (or unless) one is added.
