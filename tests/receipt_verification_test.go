package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestReceiptVerificationUsesCanonicalRequestAndPolicyHashes(t *testing.T) {
	dir := t.TempDir()
	writeReceiptPolicy(t, dir, `schema_version: "1"
policy:
  name: receipt-test
  version: "1.0.0"
  rules:
    - name: allow-select
      tool: query
      action: allow
      conditions:
        - type: equals
          field: arguments.sql_class
          value: READ
`)

	requestBody := []byte(`{"tool_name":"query","tenant_id":"tenant-1","arguments":{"sql_class":"READ","sql":"SELECT 1"},"agent_id":"agent-1"}`)
	reorderedRequestBody := []byte(`{"agent_id":"agent-1","arguments":{"sql":"SELECT 1","sql_class":"READ"},"tenant_id":"tenant-1","tool_name":"query"}`)
	requestHash, err := governance.ComputeRawRequestHash(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	reorderedHash, err := governance.ComputeRawRequestHash(reorderedRequestBody)
	if err != nil {
		t.Fatal(err)
	}
	if requestHash != reorderedHash {
		t.Fatalf("request hash should ignore JSON key order: %s != %s", requestHash, reorderedHash)
	}

	policyHash, err := governance.PolicyBundleHashFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:           governance.TransportMCP,
		ToolName:            "query",
		Action:              "allow",
		PolicyBundleHash:    policyHash,
		RequestHash:         requestHash,
		BoundaryBuildDigest: "sha256:test-build",
		TrustScore:          1,
		TrustState:          governance.TrustStateTrusted.String(),
	})

	if err := governance.VerifyDecisionRecord(record, reorderedRequestBody, dir, "sha256:test-build"); err != nil {
		t.Fatalf("valid record did not verify: %v", err)
	}

	tampered := record
	tampered.Action = "deny"
	if err := governance.VerifyDecisionRecord(tampered, reorderedRequestBody, dir, "sha256:test-build"); err == nil {
		t.Fatalf("tampered record verified")
	}
}

func TestPolicyBundleHashIgnoresFileMetadata(t *testing.T) {
	dir := t.TempDir()
	body := `schema_version: "1"
policy:
  name: receipt-test
  version: "1.0.0"
  rules:
    - name: allow-select
      tool: query
      action: allow
`
	writeReceiptPolicy(t, dir, body)
	first, err := governance.PolicyBundleHashFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy.yaml")
	if err := os.Chtimes(path, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	second, err := governance.PolicyBundleHashFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("metadata changed policy hash: %s != %s", first, second)
	}
}

func writeReceiptPolicy(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "policy.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
