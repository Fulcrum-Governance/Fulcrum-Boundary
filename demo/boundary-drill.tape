# Fulcrum Boundary — Claude Code hook demo ("boundary-drill")
#
# Reproducible source for the README hero GIF. Recreate the rendered asset
# from the repo root with:
#
#   make demo-tape
#
# or directly (after `make build`, so the freshly built binary is on PATH):
#
#   PATH="$PWD/bin:$PATH" vhs demo/boundary-drill.tape
#
# Every command below is a REAL invocation of the `boundary` binary built
# from this checkout. All output is genuine binary output; only typing
# cadence and the hold-beats (Sleep) are authored.
#
# The staged "git status && rm -rf ~/" command is never run by a real shell —
# it exists only as text inside a JSON payload piped to `boundary hook
# pretooluse`, exactly the shape Claude Code's own PreToolUse hook wrapper
# delivers on stdin (see skills/drill/SKILL.md for why piping the event this
# way, rather than issuing the command for real, is the faithful way to
# demonstrate a pre-execution deny). Decision records land in a disposable
# ./.boundary-demo-tape-scratch/ directory, removed before and after this
# recording — never a project's real ./.boundary/hook/ tree. No credentials,
# no network, no live mutation.

Output demo/boundary-drill.gif

Require boundary

Set Shell zsh
Set FontFamily "JetBrains Mono"
Set FontSize 18
Set Width 1200
Set Height 700
Set Padding 24
Set Margin 0
Set Theme { "name": "Boundary", "background": "#0b0f14", "foreground": "#d7dee8", "cursor": "#5fd0c5", "selection": "#1d2733", "black": "#0b0f14", "brightBlack": "#3b4754", "red": "#e06c75", "brightRed": "#ff7b86", "green": "#8fd694", "brightGreen": "#a6e3ab", "yellow": "#e5c07b", "brightYellow": "#f0d090", "blue": "#61afef", "brightBlue": "#7cc0ff", "magenta": "#c678dd", "brightMagenta": "#d79bea", "cyan": "#5fd0c5", "brightCyan": "#7fe3da", "white": "#d7dee8", "brightWhite": "#ffffff" }
Set WindowBar Colorful
Set BorderRadius 12

Hide
Type `rm -rf .boundary-demo-tape-scratch && clear`
Enter
Sleep 300ms
Show

# 1) Install status — a real version check against the built binary.
Sleep 500ms
Type@70ms `boundary version`
Sleep 300ms
Enter
Sleep 2.2s

# 2) A Claude-Code-shaped PreToolUse event, staged and piped to the exact
#    binary verb Claude Code's own hook wrapper execs into. The destructive
#    command is text inside this JSON — never a real shell invocation.
Type@60ms `clear`
Enter
Sleep 250ms
Type@28ms `printf '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && rm -rf ~/"}}' | boundary hook pretooluse --dir .boundary-demo-tape-scratch`
Sleep 400ms
Enter
Sleep 4.0s

# 3) Verify the record this deny just wrote, independently.
Type@60ms `clear`
Enter
Sleep 250ms
Type@42ms `boundary verify-record "$(ls -t .boundary-demo-tape-scratch/records/*.json | head -1)"`
Sleep 400ms
Enter
Sleep 2.6s

# 4) The receipt itself — one line, written before the decision reached
#    stdout in step 2.
Type@60ms `clear`
Enter
Sleep 250ms
Type@42ms `tail -n 1 .boundary-demo-tape-scratch/decision-records.jsonl`
Sleep 400ms
Enter
Sleep 3.5s

Hide
Type `rm -rf .boundary-demo-tape-scratch`
Enter
Sleep 200ms
