package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const verifierModule = "github.com/Fulcrum-Governance/Fulcrum-Boundary/verify-witnessed"

func TestSourceAndDependencyIndependence(t *testing.T) {
	root := moduleRoot(t)
	allowedExternal := map[string]bool{
		"golang.org/x/mod/sumdb/tlog": true,
	}
	disallowedStandard := map[string]bool{
		"database/sql": true,
		"net":          true,
		"net/http":     true,
		"net/rpc":      true,
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			return nil
		}
		for _, imported := range parsed.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Errorf("unquote import in %s: %v", path, err)
				continue
			}
			if disallowedStandard[name] {
				t.Errorf("production source imports forbidden network/database package %q in %s", name, path)
			}
			if strings.HasPrefix(name, verifierModule+"/") || allowedExternal[name] || !strings.Contains(strings.Split(name, "/")[0], ".") {
				continue
			}
			t.Errorf("production source imports unapproved external package %q in %s", name, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module source: %v", err)
	}

	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	text := string(goMod)
	if !strings.Contains(text, "\ngo 1.26.5\n") {
		t.Error("go.mod does not pin Go 1.26.5")
	}
	if strings.Contains(text, "replace ") || strings.Contains(text, "replace(") {
		t.Error("go.mod contains a forbidden replace directive")
	}
	if strings.Count(text, "golang.org/x/mod") != 1 || strings.Contains(text, "github.com/Fulcrum-Governance/fulcrum-io") {
		t.Errorf("go.mod dependencies are not standalone:\n%s", text)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filename)
}
