package commandboundary

import (
	"errors"
	"strings"
)

func Classify(argv []string) (Classification, error) {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return Classification{}, errors.New("command is required")
	}

	command := strings.TrimSpace(argv[0])
	args := append([]string(nil), argv[1:]...)
	class, reason := classifyCommand(command, args)
	risk, action := postureFor(class)
	return Classification{
		SchemaVersion:     SchemaVersionClassification,
		Command:           command,
		ArgsRedacted:      RedactArgs(args),
		Class:             class,
		Risk:              risk,
		RecommendedAction: action,
		Reason:            reason,
	}, nil
}

func classifyCommand(command string, args []string) (class Class, reason string) {
	name := strings.ToLower(command)
	if hasSecretArgument(args) {
		return ClassCredentialAccess, "credential or secret access"
	}

	switch name {
	case "cat", "less", "more", "head", "tail":
		if hasSecretArgument(args) {
			return ClassCredentialAccess, "secret-like path read"
		}
		return ClassObserveRead, "file read"
	case "ls", "pwd", "whoami", "git-status":
		return ClassObserveRead, "observe command"
	case "touch", "cp":
		return ClassLocalFileWrite, "local file write"
	case "mv":
		if hasDestructiveFlag(args) {
			return ClassDestructiveMutation, "destructive local mutation"
		}
		return ClassLocalFileWrite, "local file write"
	case "rm", "unlink", "rmdir":
		return ClassDestructiveMutation, "destructive local mutation"
	case "find":
		if containsArg(args, "-delete") {
			return ClassDestructiveMutation, "destructive local mutation"
		}
		return ClassObserveRead, "filesystem search"
	case "chmod", "chown":
		if hasRecursiveFlag(args) {
			return ClassDestructiveMutation, "broad permission or ownership mutation"
		}
		return ClassLocalFileWrite, "local file metadata mutation"
	case "curl", "wget", "scp", "rsync":
		if hasSecretArgument(args) {
			return ClassCredentialAccess, "credential or secret path with network egress"
		}
		return ClassNetworkEgress, "network egress"
	case "echo", "printf":
		// Output-only: both write to stdout and nothing else. A redirected
		// `echo x > file` is not laundered through this case — the decomposer
		// classifies the redirect target as its own file-write segment.
		return ClassObserveRead, "output-only command"
	case "grep":
		return ClassObserveRead, "file content search"
	case "date":
		return classifyDate(args)
	case "boundary":
		return classifyBoundary(args)
	case "git":
		return classifyGit(args)
	case "gh":
		return classifyGH(args)
	case "npm", "pnpm", "yarn", "bun", "pip", "pip3":
		return classifyPackageManager(name, args)
	case "node", "python", "python3":
		return ClassPackageLifecycle, "local code execution"
	case "docker":
		return classifyDocker(args)
	case "kubectl", "terraform":
		return classifyInfrastructure(name, args)
	case "psql", "mysql", "redis-cli":
		if hasSecretArgument(args) {
			return ClassCredentialAccess, "credential or secret access"
		}
		return ClassInfrastructureMutation, "database access or mutation"
	default:
		return ClassPackageLifecycle, "unclassified command requires review"
	}
}

// classifyDate distinguishes reading the clock from setting it. Format
// operands (`+...`) and dash flags read; `-s`/`--set` write the system clock,
// and a bare operand is the BSD set-form (`date [[[mm]dd]HH]MM...`), so both
// escalate instead of riding the observe class.
func classifyDate(args []string) (class Class, reason string) {
	for _, arg := range args {
		if arg == "-s" || strings.HasPrefix(arg, "--set") {
			return ClassInfrastructureMutation, "system clock mutation"
		}
		if !strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "+") {
			return ClassPackageLifecycle, "unclassified command requires review"
		}
	}
	return ClassObserveRead, "time observation"
}

// classifyBoundary classifies an invocation of Boundary's own CLI.
//
// This is an exact-form allowlist, not trust in the binary's name: only the
// first-party verbs the documented first-run workflow uses, in the exact
// shapes it uses them, classify as safe. Every other verb — and every listed
// verb carrying a flag this table does not recognize — falls through to the
// C7 catch-all exactly like any unrecognized command, so an unknown
// `boundary` subcommand cannot ride on the reputation of the verbs here, and
// a flag that redirects writes elsewhere (`hook pretooluse --dir`) stays
// unrecognized on purpose.
//
// The listed verbs earn their classes:
//   - version, verify-record, explain, and the help forms read and print;
//     they mutate nothing.
//   - hook doctor reads wiring and evidence; its one write is its own
//     self-erasing writability probe, and its reason says so.
//   - hook pretooluse decides a piped event and appends the decision to
//     Boundary's own evidence log; it executes nothing and touches no user
//     file, so it carries the observe class with a reason that states the
//     record append.
//   - drill cleanup deletes the drill's own fixture and nothing else, and a
//     command that deletes must stay visible, so it keeps the C1
//     local-file-write class (warn) rather than passing silently.
func classifyBoundary(args []string) (class Class, reason string) {
	if len(args) == 0 {
		return ClassObserveRead, "Boundary CLI help"
	}
	verb := strings.ToLower(args[0])
	rest := args[1:]
	switch verb {
	case "--help", "-h", "help":
		if len(rest) == 0 {
			return ClassObserveRead, "Boundary CLI help"
		}
	case "version":
		if boundaryVersionShape.matches(rest) {
			return ClassObserveRead, "Boundary version self-report"
		}
	case "verify-record":
		if boundaryVerifyRecordShape.matches(rest) {
			return ClassObserveRead, "Boundary record verification (reads and rehashes a record)"
		}
	case "explain":
		if boundaryExplainShape.matches(rest) {
			return ClassObserveRead, "Boundary record rendering (read-only)"
		}
	case "hook":
		return classifyBoundaryHook(rest)
	case "drill":
		if len(rest) == 1 && strings.EqualFold(rest[0], "cleanup") {
			return ClassLocalFileWrite, "scoped drill cleanup (removes only the drill's own fixture under .boundary-drill/)"
		}
	}
	return ClassPackageLifecycle, "unclassified command requires review"
}

// classifyBoundaryHook classifies `boundary hook ...` sub-verbs. Same
// exact-form rule as classifyBoundary: recognized shapes only, everything
// else falls back to the catch-all.
func classifyBoundaryHook(args []string) (class Class, reason string) {
	if len(args) == 0 {
		return ClassObserveRead, "Boundary hook help"
	}
	sub := strings.ToLower(args[0])
	rest := args[1:]
	switch sub {
	case "--help", "-h", "help":
		if len(rest) == 0 {
			return ClassObserveRead, "Boundary hook help"
		}
	case "doctor":
		if boundaryHookDoctorShape.matches(rest) {
			return ClassObserveRead, "Boundary hook diagnostics (read-only, plus its own self-erasing writability probe)"
		}
	case "pretooluse":
		if boundaryHookPretoolUseShape.matches(rest) {
			return ClassObserveRead, "Boundary hook decision path (classifies a piped event and appends Boundary's own decision record; executes nothing)"
		}
	}
	return ClassPackageLifecycle, "unclassified command requires review"
}

// boundaryVerbShape is the exact argument shape a first-party verb is
// recognized in: the boolean flags it documents, the value-taking flags it
// documents, and how many positional arguments it takes. It exists so the
// allowlist recognizes only the shapes the CLI intentionally supports —
// anything outside the shape (an unknown flag, a boolean flag written with a
// value, a value flag missing its value, a wrong positional count) is NOT
// first-party and falls back to the C7 catch-all.
type boundaryVerbShape struct {
	boolFlags   map[string]bool
	valueFlags  map[string]bool
	positionals int
}

// The shapes below mirror the corresponding flag sets in
// internal/boundarycli, in their documented double-dash spellings, and must
// change in lockstep with them.
var (
	boundaryVersionShape = boundaryVerbShape{
		boolFlags: map[string]bool{"--json": true},
	}
	boundaryExplainShape = boundaryVerbShape{
		boolFlags:   map[string]bool{"--json": true},
		positionals: 1,
	}
	boundaryVerifyRecordShape = boundaryVerbShape{
		boolFlags: map[string]bool{"--json": true, "--verify-signature": true},
		valueFlags: map[string]bool{
			"--request":       true,
			"--policies":      true,
			"--binary-digest": true,
			"--public-key":    true,
		},
		positionals: 1,
	}
	boundaryHookDoctorShape = boundaryVerbShape{
		boolFlags: map[string]bool{"--json": true},
	}
	boundaryHookPretoolUseShape = boundaryVerbShape{
		boolFlags: map[string]bool{"--print-record": true},
	}
)

// matches walks args against the shape. Value flags are accepted in both
// documented spellings, `--flag value` and `--flag=value`; flags and
// positionals may intersperse (the CLIs parse them that way). A boolean flag
// carrying `=value`, a value flag with a missing or empty value, an
// unrecognized flag, and a wrong positional count all fail the shape.
func (s boundaryVerbShape) matches(args []string) bool {
	positionals := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positionals++
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		switch {
		case s.boolFlags[name]:
			if hasValue {
				return false
			}
		case s.valueFlags[name]:
			if hasValue {
				if value == "" {
					return false
				}
				continue
			}
			i++
			if i >= len(args) || strings.HasPrefix(args[i], "-") {
				return false
			}
		default:
			return false
		}
	}
	return positionals == s.positionals
}

func classifyGit(args []string) (class Class, reason string) {
	if len(args) == 0 {
		return ClassObserveRead, "git help or status"
	}
	switch strings.ToLower(args[0]) {
	case "status", "diff", "log", "show", "branch", "remote":
		return ClassObserveRead, "repository observation"
	case "commit", "tag", "add", "rm", "mv", "merge", "rebase", "cherry-pick":
		return ClassRepositoryMutation, "repository mutation"
	case "push":
		return ClassRepositoryMutation, "external repository mutation"
	case "clean":
		return ClassDestructiveMutation, "destructive repository cleanup"
	case "clone", "fetch", "pull":
		return ClassNetworkEgress, "repository network egress"
	default:
		return ClassRepositoryMutation, "repository command requires review"
	}
}

func classifyGH(args []string) (class Class, reason string) {
	if len(args) == 0 {
		return ClassObserveRead, "GitHub CLI help or status"
	}
	if hasSecretArgument(args) {
		return ClassCredentialAccess, "credential or secret access"
	}
	switch strings.ToLower(args[0]) {
	case "auth":
		return ClassCredentialAccess, "credential or secret access"
	case "pr":
		return classifyGHPR(args[1:])
	case "repo", "release", "workflow", "run", "issue":
		if len(args) > 1 && isObserveSubcommand(args[1]) {
			return ClassObserveRead, "GitHub observation"
		}
		return ClassRepositoryMutation, "GitHub repository mutation"
	default:
		return ClassNetworkEgress, "GitHub network egress"
	}
}

func classifyGHPR(args []string) (class Class, reason string) {
	if len(args) == 0 || isObserveSubcommand(args[0]) {
		return ClassObserveRead, "pull request observation"
	}
	switch strings.ToLower(args[0]) {
	case "create":
		return ClassRepositoryMutation, "repository mutation"
	case "merge", "close", "edit", "review", "comment", "ready", "reopen":
		if containsArg(args, "--admin") {
			return ClassRepositoryMutation, "privileged repository mutation"
		}
		return ClassRepositoryMutation, "repository mutation"
	default:
		return ClassRepositoryMutation, "repository mutation"
	}
}

func classifyPackageManager(name string, args []string) (class Class, reason string) {
	if hasSecretArgument(args) {
		return ClassCredentialAccess, "credential or secret access"
	}
	if len(args) == 0 {
		return ClassPackageLifecycle, "package lifecycle command"
	}
	sub := strings.ToLower(args[0])
	switch name {
	case "npm", "pnpm", "yarn", "bun":
		if sub == "install" || sub == "add" || sub == "update" || sub == "run" || sub == "exec" {
			return ClassPackageLifecycle, "package lifecycle execution"
		}
	case "pip", "pip3":
		if sub == "install" || sub == "download" {
			return ClassPackageLifecycle, "package lifecycle execution"
		}
	}
	if isObserveSubcommand(sub) {
		return ClassObserveRead, "package metadata observation"
	}
	return ClassPackageLifecycle, "package lifecycle command"
}

func classifyDocker(args []string) (class Class, reason string) {
	if hasSecretArgument(args) {
		return ClassCredentialAccess, "credential or secret access"
	}
	if len(args) == 0 || isObserveSubcommand(args[0]) || strings.EqualFold(args[0], "ps") || strings.EqualFold(args[0], "images") {
		return ClassObserveRead, "runtime observation"
	}
	return ClassInfrastructureMutation, "runtime mutation"
}

func classifyInfrastructure(name string, args []string) (class Class, reason string) {
	if hasSecretArgument(args) {
		return ClassCredentialAccess, "credential or secret access"
	}
	if len(args) == 0 {
		return ClassInfrastructureMutation, name + " command requires review"
	}
	sub := strings.ToLower(args[0])
	if isObserveSubcommand(sub) || sub == "plan" || sub == "get" || sub == "describe" {
		return ClassObserveRead, name + " observation"
	}
	return ClassInfrastructureMutation, "infrastructure mutation"
}

func hasSecretArgument(args []string) bool {
	for _, arg := range args {
		lower := strings.ToLower(arg)
		if isSensitiveFlag(lower) || strings.HasPrefix(lower, "--token=") || strings.HasPrefix(lower, "--api-key=") || strings.HasPrefix(lower, "--password=") || isSensitiveArg(arg) {
			return true
		}
	}
	return false
}

func hasRecursiveFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-R" || arg == "-r" || strings.Contains(arg, "R") && strings.HasPrefix(arg, "-") {
			return true
		}
	}
	return false
}

func hasDestructiveFlag(args []string) bool {
	for _, arg := range args {
		if strings.Contains(arg, "--force") || arg == "-f" {
			return true
		}
	}
	return false
}

func containsArg(args []string, target string) bool {
	for _, arg := range args {
		if strings.EqualFold(arg, target) {
			return true
		}
	}
	return false
}

func isObserveSubcommand(sub string) bool {
	switch strings.ToLower(sub) {
	case "status", "list", "ls", "view", "show", "get", "describe", "logs", "log", "diff", "help", "--help", "-h":
		return true
	default:
		return false
	}
}
