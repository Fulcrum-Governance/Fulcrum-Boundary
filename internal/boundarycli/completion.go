package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// topLevelCommands is the authoritative list of `boundary` top-level command
// names, in the order they appear in the root help. It is the single source the
// completion scripts enumerate; completion_test.go asserts it stays in sync with
// the dispatch switch in cli.go so a new command cannot be added without its
// completion entry. Help-only spellings (`help`, `--help`, `--version`, `-v`)
// are intentionally omitted — they are aliases, not commands a user tab-completes
// to discover the surface.
var topLevelCommands = []string{
	"version",
	"init",
	"inventory",
	"graph",
	"dashboard",
	"install",
	"uninstall",
	"lock",
	"verify-lock",
	"redteam",
	"selftest",
	"secure",
	"command",
	"edit",
	"shell",
	"policy",
	"mcp",
	"serve",
	"demo",
	"verify",
	"verify-record",
	"explain",
	"replay",
	"test",
	"doctor",
	"evidence",
	"audit",
	"trust",
	"completion",
}

// compoundSubcommands maps each compound top-level command to its second-level
// subcommand names. These are the only nested verbs the static scripts complete;
// they mirror the inner dispatch switches (command.go, edit.go, secure.go,
// firewall.go, mcp.go, evidence.go, cli.go's runTrust/runDemo). Flags are not
// completed — the scripts are deliberately command-only so they never drift from
// per-command flag changes.
var compoundSubcommands = map[string][]string{
	"policy":    {"generate"},
	"mcp":       {"proxy"},
	"secure":    {"github"},
	"demo":      {"action-boundary", "postgres", "github-lethal-trifecta", "command-secret-exfil", "tamper-evidence", "trust-degradation"},
	"evidence":  {"bundle", "verify"},
	"trust":     {"show", "reset"},
	"command":   {"classify", "run", "install", "uninstall"},
	"edit":      {"inspect", "apply"},
	"inventory": {"ingest"},
}

func runCompletion(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary completion", stderr, commandHelp{
		Purpose: "Print a shell completion script for the boundary command to stdout.",
		Usage:   "boundary completion <bash|zsh|fish>",
		Common: []string{
			"boundary completion bash > /usr/local/etc/bash_completion.d/boundary",
			"boundary completion zsh > \"${fpath[1]}/_boundary\"",
			"boundary completion fish > ~/.config/fish/completions/boundary.fish",
		},
		Notes: []string{
			"The script is static: it completes top-level command names and the subcommands of compound commands, not flags.",
			"Because it is static, regenerate it after upgrading boundary (via your shell rc or, for a brew install, on `brew upgrade`) so new commands complete.",
			"Completion is a convenience surface only; it does not change which routes Boundary governs.",
		},
	})
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: boundary completion <bash|zsh|fish>")
		return 1
	}

	switch fs.Arg(0) {
	case "bash":
		fmt.Fprint(stdout, bashCompletionScript())
	case "zsh":
		fmt.Fprint(stdout, zshCompletionScript())
	case "fish":
		fmt.Fprint(stdout, fishCompletionScript())
	default:
		fmt.Fprintf(stderr, "completion: unknown shell %q (valid: bash, zsh, fish)\n", fs.Arg(0))
		return 1
	}
	return 0
}

// compoundCommandNames returns the compound command names in sorted order so the
// generated scripts are deterministic across runs (Go map iteration is not).
func compoundCommandNames() []string {
	names := make([]string, 0, len(compoundSubcommands))
	for name := range compoundSubcommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// bashCompletionScript renders a static bash completion function for `boundary`.
// Word 1 completes against the top-level command names; word 2 completes against
// the compound command's subcommands (empty for non-compound commands). It is
// written to pass `bash -n` (a parse-only check) so a syntactically broken script
// can never ship.
func bashCompletionScript() string {
	var b strings.Builder
	b.WriteString("# bash completion for boundary (static; regenerate after upgrades)\n")
	b.WriteString("_boundary_completion() {\n")
	b.WriteString("    local cur prev\n")
	b.WriteString("    COMPREPLY=()\n")
	b.WriteString("    cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	b.WriteString("    prev=\"${COMP_WORDS[COMP_CWORD-1]}\"\n")
	fmt.Fprintf(&b, "    local commands=%q\n", strings.Join(topLevelCommands, " "))
	b.WriteString("    if [ \"$COMP_CWORD\" -eq 1 ]; then\n")
	b.WriteString("        COMPREPLY=( $(compgen -W \"${commands}\" -- \"${cur}\") )\n")
	b.WriteString("        return 0\n")
	b.WriteString("    fi\n")
	b.WriteString("    if [ \"$COMP_CWORD\" -eq 2 ]; then\n")
	b.WriteString("        case \"${prev}\" in\n")
	for _, name := range compoundCommandNames() {
		subs := strings.Join(compoundSubcommands[name], " ")
		fmt.Fprintf(&b, "            %s) COMPREPLY=( $(compgen -W %q -- \"${cur}\") ) ;;\n", name, subs)
	}
	b.WriteString("        esac\n")
	b.WriteString("        return 0\n")
	b.WriteString("    fi\n")
	b.WriteString("    return 0\n")
	b.WriteString("}\n")
	b.WriteString("complete -F _boundary_completion boundary\n")
	return b.String()
}

// zshCompletionScript renders a static zsh completion function for `boundary`.
// It uses _describe for the top-level commands and a small case on word 2 for the
// compound commands' subcommands. It is written to pass `zsh -n`.
func zshCompletionScript() string {
	var b strings.Builder
	b.WriteString("#compdef boundary\n")
	b.WriteString("# zsh completion for boundary (static; regenerate after upgrades)\n")
	b.WriteString("_boundary() {\n")
	b.WriteString("    local -a commands\n")
	fmt.Fprintf(&b, "    commands=(%s)\n", quoteZshWords(topLevelCommands))
	b.WriteString("    if (( CURRENT == 2 )); then\n")
	b.WriteString("        _describe 'command' commands\n")
	b.WriteString("        return\n")
	b.WriteString("    fi\n")
	b.WriteString("    if (( CURRENT == 3 )); then\n")
	b.WriteString("        case \"${words[2]}\" in\n")
	for _, name := range compoundCommandNames() {
		subs := quoteZshWords(compoundSubcommands[name])
		fmt.Fprintf(&b, "            %s) compadd %s ;;\n", name, subs)
	}
	b.WriteString("        esac\n")
	b.WriteString("        return\n")
	b.WriteString("    fi\n")
	b.WriteString("}\n")
	b.WriteString("_boundary \"$@\"\n")
	return b.String()
}

// fishCompletionScript renders static fish completion rules for `boundary`. Each
// top-level command and each compound subcommand becomes a `complete` line;
// `__fish_use_subcommand` / `__fish_seen_subcommand_from` gate the two levels.
func fishCompletionScript() string {
	var b strings.Builder
	b.WriteString("# fish completion for boundary (static; regenerate after upgrades)\n")
	for _, name := range topLevelCommands {
		fmt.Fprintf(&b, "complete -c boundary -f -n __fish_use_subcommand -a %s\n", name)
	}
	for _, name := range compoundCommandNames() {
		for _, sub := range compoundSubcommands[name] {
			fmt.Fprintf(&b, "complete -c boundary -f -n '__fish_seen_subcommand_from %s' -a %s\n", name, sub)
		}
	}
	return b.String()
}

// quoteZshWords joins words into a single space-separated list with each word
// single-quoted, suitable for a zsh array literal or compadd argument list. None
// of the boundary command names contain a single quote, so plain wrapping is
// sufficient and keeps the rendered script readable.
func quoteZshWords(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = "'" + w + "'"
	}
	return strings.Join(quoted, " ")
}
