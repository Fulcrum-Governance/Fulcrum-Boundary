package proofreceipt

import (
	"os"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func mkRecord(t *testing.T) governance.DecisionRecordV1 {
	t.Helper()
	return governance.BuildDecisionRecord(governance.AuditEvent{
		Transport: governance.TransportMCP, ToolName: "create_or_update_file",
		Action: "deny", Reason: "protected path", TrustScore: 1,
		TrustState: governance.TrustStateTrusted.String(),
	})
}

func TestNewBindsAndVerifies(t *testing.T) {
	rec := mkRecord(t)
	r := New(rec, "fulcrum-proof-checker/0.1.0", "sha256:abc",
		[]Invariant{{TheoremID: "THM-BUDGET-LOCAL", Predicate: "spent_before + requested <= limit",
			InputsHash: CanonicalInputsHash(map[string]any{"k": "v"}), Result: ResultPass}}, time.Time{})
	if r.ReceiptVersion != ReceiptVersion {
		t.Fatalf("receipt_version = %q, want %q", r.ReceiptVersion, ReceiptVersion)
	}
	if r.DecisionHash != rec.DecisionHash {
		t.Fatalf("decision_hash = %q, want bound %q", r.DecisionHash, rec.DecisionHash)
	}
	if r.RecordedAt.IsZero() {
		t.Fatal("RecordedAt must default to now")
	}
	if err := r.VerifyBinding(rec); err != nil {
		t.Fatalf("binding must verify: %v", err)
	}
}

func TestVerifyBindingRejectsWrongHashAndVersion(t *testing.T) {
	rec := mkRecord(t)
	r := New(rec, "c", "h", nil, time.Now())
	tampered := r
	tampered.DecisionHash = "sha256:deadbeef"
	if err := tampered.VerifyBinding(rec); err == nil {
		t.Fatal("wrong decision_hash must fail binding")
	}
	badVer := r
	badVer.ReceiptVersion = "proof-receipt-v9.9"
	if err := badVer.VerifyBinding(rec); err == nil {
		t.Fatal("wrong receipt_version must fail binding")
	}
}

func TestCanonicalInputsHashKeyOrderInvariant(t *testing.T) {
	a := CanonicalInputsHash(map[string]any{"a": 1, "b": 2})
	b := CanonicalInputsHash(map[string]any{"b": 2, "a": 1})
	if a != b {
		t.Fatalf("inputs hash must ignore key order: %s != %s", a, b)
	}
	if len(a) < 8 || a[:7] != "sha256:" {
		t.Fatalf("inputs hash must be sha256:-prefixed, got %q", a)
	}
}

func TestReadJSONRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/bad.json"
	// Valid receipt object plus a forbidden extra field.
	data := []byte(`{
  "receipt_version": "proof-receipt-v0.1",
  "decision_hash": "sha256:abc",
  "checker_id": "c",
  "checker_build_hash": "h",
  "invariants": [],
  "recorded_at": "2024-01-01T00:00:00Z",
  "decision_mode": "proved"
}`)
	if err := writeFile(t, path, data); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := ReadJSON(path); err == nil {
		t.Fatal("ReadJSON must reject unknown field decision_mode")
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	rec := mkRecord(t)
	inv := Invariant{
		TheoremID:  "THM-BUDGET-LOCAL",
		Predicate:  "spent_before + requested <= limit",
		InputsHash: CanonicalInputsHash(map[string]any{"k": "v"}),
		Result:     ResultPass,
	}
	r := New(rec, "fulcrum-proof-checker/0.1.0", "sha256:buildhash", []Invariant{inv}, time.Time{})

	dir := t.TempDir()
	path := dir + "/receipt.json"
	if err := WriteJSON(path, r); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	got, err := ReadJSON(path)
	if err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if err := got.VerifyBinding(rec); err != nil {
		t.Fatalf("VerifyBinding after round-trip: %v", err)
	}
	if got.ReceiptVersion != r.ReceiptVersion {
		t.Errorf("ReceiptVersion: got %q want %q", got.ReceiptVersion, r.ReceiptVersion)
	}
	if got.DecisionHash != r.DecisionHash {
		t.Errorf("DecisionHash: got %q want %q", got.DecisionHash, r.DecisionHash)
	}
	if len(got.Invariants) != 1 {
		t.Errorf("Invariants len: got %d want 1", len(got.Invariants))
	}
}

func TestReadJSONRejectsTrailingData(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/trailing.json"
	// Valid receipt followed by a rogue second object on a new line.
	data := []byte("{\"receipt_version\":\"proof-receipt-v0.1\",\"decision_hash\":\"sha256:abc\",\"checker_id\":\"c\",\"checker_build_hash\":\"h\",\"invariants\":[],\"recorded_at\":\"2024-01-01T00:00:00Z\"}\n{\"decision_mode\":\"proved\"}\n")
	if err := writeFile(t, path, data); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := ReadJSON(path); err == nil {
		t.Fatal("ReadJSON must reject trailing data after the receipt object")
	}
}

// writeFile is a test helper to write bytes to a file.
func writeFile(t *testing.T, path string, data []byte) error {
	t.Helper()
	return os.WriteFile(path, data, 0o600)
}
