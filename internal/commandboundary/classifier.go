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
