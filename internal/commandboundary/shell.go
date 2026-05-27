package commandboundary

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvCommandMode = "BOUNDARY_COMMAND_MODE"
	EnvProjectRoot = "BOUNDARY_PROJECT_ROOT"
)

func ShellEnvironment(projectRoot string, environ []string) ([]string, error) {
	root, err := cleanProjectRoot(projectRoot)
	if err != nil {
		return nil, err
	}
	binDir := ProjectBinDir(root)
	env := append([]string(nil), environ...)
	path := lookupEnv(env, "PATH")
	if path == "" {
		path = binDir
	} else {
		path = binDir + string(os.PathListSeparator) + path
	}
	env = upsertEnv(env, "PATH", path)
	env = upsertEnv(env, EnvCommandMode, "project")
	env = upsertEnv(env, EnvProjectRoot, root)
	return env, nil
}

func ShellBanner(projectRoot string) (string, error) {
	root, err := cleanProjectRoot(projectRoot)
	if err != nil {
		return "", err
	}
	relBin, err := filepath.Rel(root, ProjectBinDir(root))
	if err != nil || strings.HasPrefix(relBin, "..") {
		relBin = ProjectBinDir(root)
	}
	return fmt.Sprintf(`Boundary Command Shell
Project: %s
Shims: %s
Commands with shims route through Boundary.
Direct commands without shims are outside Boundary.
Exit with Ctrl-D.
`, root, relBin), nil
}

func ShellPreview(projectRoot string, environ []string) (string, error) {
	root, err := cleanProjectRoot(projectRoot)
	if err != nil {
		return "", err
	}
	env, err := ShellEnvironment(root, environ)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`export PATH=%q
export %s=%q
export %s=%q
`, lookupEnv(env, "PATH"), EnvCommandMode, lookupEnv(env, EnvCommandMode), EnvProjectRoot, lookupEnv(env, EnvProjectRoot)), nil
}

func lookupEnv(environ []string, key string) string {
	prefix := key + "="
	for _, item := range environ {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return ""
}

func upsertEnv(environ []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(environ)+1)
	replaced := false
	for _, item := range environ {
		if strings.HasPrefix(item, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, item)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}
