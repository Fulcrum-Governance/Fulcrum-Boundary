package interceptors_test

import (
	"os"
	"path/filepath"
	"testing"

	sqlguard "github.com/fulcrum-governance/boundary/interceptors/sql"
	"gopkg.in/yaml.v3"
)

type evasionCorpus struct {
	Cases []struct {
		ID   string `yaml:"id"`
		SQL  string `yaml:"sql"`
		Want string `yaml:"want"`
	} `yaml:"cases"`
}

func TestPostgresEvasionCorpus(t *testing.T) {
	path := filepath.Join("..", "..", "interceptors", "sql", "evasion_corpus", "postgres.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read evasion corpus: %v", err)
	}
	var corpus evasionCorpus
	if err := yaml.Unmarshal(body, &corpus); err != nil {
		t.Fatalf("parse evasion corpus: %v", err)
	}
	if len(corpus.Cases) < 30 {
		t.Fatalf("expected at least 30 corpus cases, got %d", len(corpus.Cases))
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			got := sqlguard.ClassifyPostgres(tc.SQL)
			if got.Class != sqlguard.Class(tc.Want) {
				t.Fatalf("classify %q = %s (%v), want %s", tc.SQL, got.Class, got.ParseError, tc.Want)
			}
		})
	}
}
