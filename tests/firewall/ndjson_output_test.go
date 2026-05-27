package firewall_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestBoundaryInventoryNDJSONOutputValidatesAgainstSchema(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github", "--token", "ghp_secret"],
      "env": {"GITHUB_TOKEN": "ghp_secret"},
      "tools": [
        {"name": "get_issue"},
        {"name": "create_or_update_file"},
        {"name": "merge_pull_request"}
      ]
    },
    "boundary-github": {
      "command": "boundary",
      "args": ["proxy", "mcp", "--server", "github-mcp-server"],
      "tools": [{"name": "create_or_update_file"}]
    },
    "remote": {
      "url": "https://user:pass@example.invalid/mcp?token=ghp_secret&safe=value"
    }
  }
}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"inventory", "--root", root, "--home", home, "--format", "ndjson"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inventory ndjson exit = %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, leaked := range []string{"ghp_secret", "user:pass", "pass@example.invalid"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("NDJSON output leaked secret material %q: %s", leaked, output)
		}
	}

	records := parseNDJSONRecords(t, output)
	if len(records) < 10 {
		t.Fatalf("expected rich NDJSON record stream, got %d records: %s", len(records), output)
	}
	schema := compileInventoryRecordSchema(t)
	types := map[string]bool{}
	var scanID string
	for i, record := range records {
		if err := schema.Validate(record); err != nil {
			body, _ := json.Marshal(record)
			t.Fatalf("record %d failed schema validation: %v\n%s", i+1, err, string(body))
		}
		recordType := stringField(t, record, "record_type")
		types[recordType] = true
		sequence := intField(t, record, "sequence")
		if sequence != i+1 {
			t.Fatalf("record %d sequence = %d, want %d", i+1, sequence, i+1)
		}
		currentScanID := stringField(t, record, "scan_id")
		if scanID == "" {
			scanID = currentScanID
		} else if currentScanID != scanID {
			t.Fatalf("record %d scan_id = %s, want %s", i+1, currentScanID, scanID)
		}
		if !strings.HasPrefix(stringField(t, record, "record_id"), scanID+":") {
			t.Fatalf("record %d id does not use scan id %s: %s", i+1, scanID, stringField(t, record, "record_id"))
		}
	}

	if got := stringField(t, records[0], "record_type"); got != "scan_start" {
		t.Fatalf("first record_type = %s, want scan_start", got)
	}
	if got := stringField(t, records[len(records)-1], "record_type"); got != "scan_summary" {
		t.Fatalf("last record_type = %s, want scan_summary", got)
	}
	for _, want := range []string{
		"scan_start",
		"agent_client",
		"mcp_config",
		"mcp_server",
		"tool_descriptor",
		"tool_capability",
		"risk_path",
		"policy_recommendation",
		"descriptor_lock_status",
		"install_status",
		"scan_summary",
	} {
		if !types[want] {
			t.Fatalf("NDJSON output missing record_type %q; got %#v", want, types)
		}
	}

	summary := objectField(t, records[len(records)-1], "scan_summary")
	if got := stringField(t, summary, "status"); got != "complete" {
		t.Fatalf("scan_summary.status = %s, want complete", got)
	}
	if got := boolField(t, summary, "complete"); !got {
		t.Fatalf("scan_summary.complete = false, want true")
	}
	counts := objectField(t, summary, "record_counts")
	if got := intField(t, counts, "scan_start"); got != 1 {
		t.Fatalf("scan_start count = %d, want 1", got)
	}
	if got := intField(t, counts, "scan_summary"); got != 1 {
		t.Fatalf("scan_summary count = %d, want 1", got)
	}
}

func TestBoundaryInventoryNDJSONOutWritesRecordStream(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "boundary-inventory.ndjson")
	writeFile(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [{"name": "merge_pull_request"}]
    }
  }
}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"inventory", "--root", root, "--home", home, "--format", "ndjson", "--out", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inventory ndjson --out exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "inventory report: "+outPath) {
		t.Fatalf("stdout missing report path: %s", stdout.String())
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	records := parseNDJSONRecords(t, string(body))
	if len(records) == 0 {
		t.Fatalf("no NDJSON records written to %s", outPath)
	}
	if got := stringField(t, records[0], "record_type"); got != "scan_start" {
		t.Fatalf("first output record = %s, want scan_start", got)
	}
	if got := stringField(t, records[len(records)-1], "record_type"); got != "scan_summary" {
		t.Fatalf("last output record = %s, want scan_summary", got)
	}
}

func parseNDJSONRecords(t *testing.T, output string) []map[string]any {
	t.Helper()
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		t.Fatal("empty NDJSON output")
	}
	lines := strings.Split(trimmed, "\n")
	records := make([]map[string]any, 0, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			t.Fatalf("blank NDJSON line at index %d", i)
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("line %d is not valid JSON: %v\n%s", i+1, err, line)
		}
		records = append(records, record)
	}
	return records
}

func compileInventoryRecordSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	schemaPath := filepath.Join(repoRoot, "schemas", "boundary-inventory-record.v1.json")
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile("file://" + schemaPath)
	if err != nil {
		t.Fatalf("compile inventory record schema: %v", err)
	}
	return schema
}

func stringField(t *testing.T, object map[string]any, field string) string {
	t.Helper()
	value, ok := object[field].(string)
	if !ok {
		t.Fatalf("field %s is %T, want string", field, object[field])
	}
	return value
}

func intField(t *testing.T, object map[string]any, field string) int {
	t.Helper()
	value, ok := object[field].(float64)
	if !ok {
		t.Fatalf("field %s is %T, want number", field, object[field])
	}
	return int(value)
}

func boolField(t *testing.T, object map[string]any, field string) bool {
	t.Helper()
	value, ok := object[field].(bool)
	if !ok {
		t.Fatalf("field %s is %T, want bool", field, object[field])
	}
	return value
}

func objectField(t *testing.T, object map[string]any, field string) map[string]any {
	t.Helper()
	value, ok := object[field].(map[string]any)
	if !ok {
		t.Fatalf("field %s is %T, want object", field, object[field])
	}
	return value
}
