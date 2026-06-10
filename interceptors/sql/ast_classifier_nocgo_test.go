//go:build !cgo

package sql_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	sqlguard "github.com/fulcrum-governance/fulcrum-boundary/interceptors/sql"
	"gopkg.in/yaml.v3"
)

// These tests only build when CGO_ENABLED=0. They pin the fail-safe contract
// of the no-cgo stub: with the PostgreSQL AST parser unavailable, every
// statement routes to the UNKNOWN bucket — which the Postgres guard denies
// fail-closed — and never to a class the cgo classifier would treat as more
// permissive.

func TestNoCgoClassifierRoutesEverythingToUnknown(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"read the cgo build allows", "SELECT id FROM users"},
		{"destructive the cgo build denies", "DROP TABLE users"},
		{"unparsable stays unknown", "DR/**/OP TABLE users"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sqlguard.ClassifyPostgres(tc.sql)
			if got.Class != sqlguard.ClassUnknown {
				t.Fatalf("classify %q = %s, want %s (fail-safe bucket)", tc.sql, got.Class, sqlguard.ClassUnknown)
			}
			if !strings.Contains(got.ParseError, "CGO disabled") {
				t.Fatalf("classify %q parse error %q does not state the CGO-disabled reason", tc.sql, got.ParseError)
			}
		})
	}
	if got := sqlguard.ClassifyPostgres("   "); got.Class != sqlguard.ClassUnknown {
		t.Fatalf("classify blank SQL = %s, want %s", got.Class, sqlguard.ClassUnknown)
	}
}

func TestNoCgoClassifierRoutesEvasionCorpusToUnknown(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("evasion_corpus", "postgres.yaml"))
	if err != nil {
		t.Fatalf("read evasion corpus: %v", err)
	}
	var corpus struct {
		Cases []struct {
			ID  string `yaml:"id"`
			SQL string `yaml:"sql"`
		} `yaml:"cases"`
	}
	if err := yaml.Unmarshal(body, &corpus); err != nil {
		t.Fatalf("parse evasion corpus: %v", err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("evasion corpus is empty")
	}
	for _, tc := range corpus.Cases {
		got := sqlguard.ClassifyPostgres(tc.SQL)
		if got.Class != sqlguard.ClassUnknown {
			t.Fatalf("corpus case %s: classify %q = %s, want %s — the no-cgo stub must never assign a more permissive class", tc.ID, tc.SQL, got.Class, sqlguard.ClassUnknown)
		}
	}
}

func TestNoCgoGuardDeniesFailClosed(t *testing.T) {
	guard := sqlguard.NewPostgresInterceptor()
	req := &governance.GovernanceRequest{
		ToolName:  "query",
		Arguments: map[string]any{"sql": "SELECT id FROM users"},
	}
	result, err := guard(context.Background(), req)
	if err != nil {
		t.Fatalf("guard returned error: %v", err)
	}
	if result == nil || result.Allowed || result.Action != "deny" {
		t.Fatalf("guard result = %#v, want fail-closed deny", result)
	}
	if !strings.Contains(result.Reason, "CGO disabled") {
		t.Fatalf("deny reason %q does not state the CGO-disabled cause", result.Reason)
	}
	if req.Arguments["sql_class"] != string(sqlguard.ClassUnknown) {
		t.Fatalf("expected UNKNOWN annotation, got %#v", req.Arguments["sql_class"])
	}
}
