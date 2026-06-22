package claims

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestClaimsLedgerMatchesReleaseTruth cross-checks the machine-readable claims
// ledger against the human-authored release-truth table. claims_test.go only
// proves the YAML is internally consistent; nothing caught the drift where
// docs/RELEASE_TRUTH_PUBLIC.md marked a claim "delivered" while the ledger still
// said "partial". This test closes that gap: it parses the "## Claims Status"
// table in RELEASE_TRUTH_PUBLIC.md and asserts the ledger status is consistent —
// a claim release-truth reports delivered must not be partial/planned/false in
// the ledger.
//
// It is a CONSISTENCY check, not a completeness check: the table is the recorded
// claim delta for the current release, so claims present on only one side are
// reported, never failed.
var releaseTruthClaimID = regexp.MustCompile(`^BND-CLAIM-[A-Z0-9-]+$`)

func TestClaimsLedgerMatchesReleaseTruth(t *testing.T) {
	repoRoot := mustAbs(t)
	releaseTruth := readFile(t, filepath.Join(repoRoot, "docs", "RELEASE_TRUTH_PUBLIC.md"))
	truthStatus := parseClaimsStatusTable(t, releaseTruth)
	if len(truthStatus) == 0 {
		t.Fatal("parsed zero BND-CLAIM-* rows from the '## Claims Status' table in docs/RELEASE_TRUTH_PUBLIC.md — the table heading or format changed and the cross-check would silently pass")
	}

	ledger := loadLedger(t, repoRoot)
	ledgerStatus := map[string]string{}
	for _, c := range ledger.Claims {
		ledgerStatus[c.ID] = c.Status
	}

	known := map[string]bool{"delivered": true, "partial": true, "planned": true, "false": true}
	checked := 0
	for id, truth := range truthStatus {
		if !known[truth] {
			t.Fatalf("RELEASE_TRUTH_PUBLIC.md claim %s has unrecognized status %q — update this test if a new status label was introduced", id, truth)
		}
		yaml, ok := ledgerStatus[id]
		if !ok {
			// Table-only claim: report, do not fail (consistency, not completeness).
			t.Logf("note: %s appears in RELEASE_TRUTH_PUBLIC.md (%s) but not in the claims ledger", id, truth)
			continue
		}
		checked++
		if truth == "delivered" && yaml != "delivered" {
			t.Errorf("claim %s: RELEASE_TRUTH_PUBLIC.md says delivered but claims/boundary_claims.yaml says %q — promote the ledger entry or correct release truth", id, yaml)
		}
	}
	if checked == 0 {
		t.Fatal("no RELEASE_TRUTH claim id resolved against the ledger — the id format or parse drifted, so the cross-check is vacuous")
	}
}

// parseClaimsStatusTable extracts {claimID: bareStatus} from the markdown table
// under the "## Claims Status" heading. The status cell ("delivered (v0.10.1)")
// is normalized to its bare token ("delivered").
func parseClaimsStatusTable(t *testing.T, md string) map[string]string {
	t.Helper()
	out := map[string]string{}
	inSection := false
	for line := range strings.SplitSeq(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inSection = strings.EqualFold(trimmed, "## Claims Status")
			continue
		}
		if !inSection || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := splitTableRow(trimmed)
		if len(cells) < 2 {
			continue
		}
		id := cells[0]
		if !releaseTruthClaimID.MatchString(id) {
			continue // header row, separator row, or prose row
		}
		status, _, _ := strings.Cut(cells[1], "(")
		out[id] = strings.ToLower(strings.TrimSpace(status))
	}
	return out
}

// splitTableRow splits a "| a | b | c |" row into trimmed cells, dropping the
// empty fields produced by the leading and trailing pipes.
func splitTableRow(row string) []string {
	parts := strings.Split(strings.Trim(row, "|"), "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}
