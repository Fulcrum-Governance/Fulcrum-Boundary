# Fulcrum Boundary — flagship demo recording (Lane 1: GitHub write-after-taint)
#
# Reproducible source for the README hero asset. Recreate the rendered assets by
# running Charm VHS against this source file from the repo root.
#
# This records a REAL run of `boundary demo github-lethal-trifecta`.
# All output is genuine binary output. Only typing cadence and the hold-beats
# (Sleep) are authored. The first command shows the full real run; the second
# re-surfaces the real verdict header (lines 1-14 of the same command) so the
# DENY + decision hash + limits block are readable at rest.
# No credentials, no network, no live GitHub mutation.

Output docs/assets/github-lethal-trifecta-demo.gif
Output docs/assets/github-lethal-trifecta-demo.mp4

Require boundary

Set Shell zsh
Set FontFamily "JetBrains Mono"
Set FontSize 20
Set Width 1440
Set Height 810
Set Padding 26
Set Margin 0
Set Theme { "name": "Boundary", "background": "#0b0f14", "foreground": "#d7dee8", "cursor": "#5fd0c5", "selection": "#1d2733", "black": "#0b0f14", "brightBlack": "#3b4754", "red": "#e06c75", "brightRed": "#ff7b86", "green": "#8fd694", "brightGreen": "#a6e3ab", "yellow": "#e5c07b", "brightYellow": "#f0d090", "blue": "#61afef", "brightBlue": "#7cc0ff", "magenta": "#c678dd", "brightMagenta": "#d79bea", "cyan": "#5fd0c5", "brightCyan": "#7fe3da", "white": "#d7dee8", "brightWhite": "#ffffff" }
Set WindowBar Colorful
Set BorderRadius 12

Hide
Type "clear" Sleep 200ms Enter
Show

# 1) The real run — full genuine output streams through.
Sleep 700ms
Type@85ms "boundary demo github-lethal-trifecta"
Sleep 400ms
Enter
Sleep 2.6s

# 2) Rest on the real verdict header so the money lines hold on screen.
Type@70ms "clear" Enter
Sleep 250ms
Type@55ms "boundary demo github-lethal-trifecta | sed -n '1,14p'"
Sleep 400ms
Enter
Sleep 5.5s

# 3) Show the evidence pack — the primary artifact (manifest + hashed receipt).
Type@70ms "clear" Enter
Sleep 250ms
Type@55ms "boundary demo github-lethal-trifecta --evidence-pack ./gh-pack && ls gh-pack"
Sleep 400ms
Enter
Sleep 4.0s
