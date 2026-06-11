package sql

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// corpusSeedFile is the evasion corpus the SQL guard ships with. Its statements
// are reused as fuzz seeds so the corpus and the fuzzer stay in sync: every case
// the guard already reasons about becomes a starting point the fuzzer mutates.
const corpusSeedFile = "evasion_corpus/postgres.yaml"

// sqlFuzzSeeds returns the SQL statements that seed FuzzSQLClassifier: every
// statement from the shipped evasion corpus plus a small fixed set of degenerate
// inputs (empty, whitespace, NUL bytes, comment-only, multi-statement) that
// exercise the fail-safe boundary directly. Corpus reads are best-effort: a
// missing or unparsable corpus only drops the corpus-derived seeds, it never
// fails the seeding (the inline seeds still run).
func sqlFuzzSeeds(t *testing.T) []string {
	t.Helper()
	seeds := []string{
		"",
		" ",
		"\t\n",
		"\x00",
		"SELECT id FROM users",
		"DROP TABLE users",
		"/* only a comment */",
		"SELECT 1; DROP TABLE users",
		"DR/**/OP TABLE users",
		"SELECT @@@",
		"SELECT $$DROP TABLE users$$",
		"\xff\xfe\xfd",
	}

	body, err := os.ReadFile(corpusSeedFile)
	if err != nil {
		t.Logf("evasion corpus unavailable (%v); using inline seeds only", err)
		return seeds
	}
	var corpus struct {
		Cases []struct {
			SQL string `yaml:"sql"`
		} `yaml:"cases"`
	}
	if err := yaml.Unmarshal(body, &corpus); err != nil {
		t.Logf("evasion corpus unparsable (%v); using inline seeds only", err)
		return seeds
	}
	for _, c := range corpus.Cases {
		seeds = append(seeds, c.SQL)
	}
	return seeds
}

// FuzzSQLClassifier fuzzes arbitrary bytes through the public Postgres AST
// classifier entry point, ClassifyPostgres. Two contracts are asserted on every
// input:
//
//  1. Liveness — ClassifyPostgres must never panic, regardless of how malformed
//     the bytes are (invalid UTF-8, partial statements, NUL bytes, enormous
//     nesting). A panic here would be a denial-of-service on the guard.
//  2. Fail-safe — the classifier must never silently allow. The only positive
//     check the fuzzer can make without a Postgres oracle is the documented
//     floor: empty or whitespace-only input MUST classify as UNKNOWN (the bucket
//     the guard denies fail-closed), never as READ/WRITE/ADMIN/DESTRUCTIVE. Any
//     other class is an acceptable outcome for non-empty input; UNKNOWN is always
//     acceptable. This makes the test valid under both the cgo classifier and the
//     CGO_ENABLED=0 stub, which routes everything to UNKNOWN.
//
// The returned Class is also asserted to be one of the five known buckets, so a
// future classifier change that returns an unlabeled class is caught.
func FuzzSQLClassifier(f *testing.F) {
	for _, seed := range sqlFuzzSeeds(&testing.T{}) {
		f.Add(seed)
	}

	knownClasses := map[Class]bool{
		ClassRead:        true,
		ClassWrite:       true,
		ClassAdmin:       true,
		ClassDestructive: true,
		ClassUnknown:     true,
	}

	f.Fuzz(func(t *testing.T, sqlText string) {
		// Contract 1: never panics. (A panic fails the fuzz iteration.)
		got := ClassifyPostgres(sqlText)

		// The class is always one of the five documented buckets, never empty.
		if !knownClasses[got.Class] {
			t.Fatalf("ClassifyPostgres returned unknown class %q for %q", got.Class, sqlText)
		}

		// Contract 2: fail-safe floor. Empty / whitespace-only input must land in
		// the fail-safe UNKNOWN bucket — it must never be classified as something
		// the guard would treat more permissively.
		if strings.TrimSpace(sqlText) == "" {
			if got.Class != ClassUnknown {
				t.Fatalf("empty/whitespace SQL classified as %q, want UNKNOWN (fail-safe)", got.Class)
			}
			if !got.Unknown() {
				t.Fatalf("empty/whitespace SQL: Unknown() = false, want true (fail-safe)")
			}
		}
	})
}
