package boundarycli

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestRun_HelpListsCommands(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	for _, want := range []string{"init", "inventory", "graph", "command", "policy generate", "serve", "demo postgres", "verify", "verify-record", "test", "doctor", "audit"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing %q: %s", want, stdout.String())
		}
	}
}

func TestRun_VersionFlagAliases(t *testing.T) {
	for _, alias := range []string{"--version", "-v"} {
		var stdout bytes.Buffer
		code := Run([]string{alias}, &stdout, &bytes.Buffer{})
		if code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", alias, code)
		}
		if !strings.Contains(stdout.String(), "Fulcrum Boundary ") {
			t.Fatalf("%s: missing version output: %s", alias, stdout.String())
		}
	}
}

func TestRun_HelpTopicRouting(t *testing.T) {
	var stdout, helpErr bytes.Buffer
	code := Run([]string{"help", "version"}, &stdout, &helpErr)
	if code != 0 {
		t.Fatalf("help version: expected exit 0, got %d", code)
	}
	if combined := stdout.String() + helpErr.String(); !strings.Contains(combined, "Print Boundary version and build metadata.") {
		t.Fatalf("help version: missing rich help purpose: %s", combined)
	}

	stdout.Reset()
	code = Run([]string{"help"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("bare help: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), `Use "boundary <command> --help"`) {
		t.Fatalf("bare help: expected root help: %s", stdout.String())
	}

	stdout.Reset()
	helpErr.Reset()
	code = Run([]string{"help", "demo", "postgres"}, &stdout, &helpErr)
	if code != 0 {
		t.Fatalf("help demo postgres: expected exit 0, got %d", code)
	}
	if combined := stdout.String() + helpErr.String(); !strings.Contains(combined, "Run the Postgres allow, deny, and bypass demo") {
		t.Fatalf("compound help topic must reach the leaf command's help: %s", combined)
	}

	var stderr bytes.Buffer
	code = Run([]string{"help", "no-such-command"}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatalf("help with unknown topic: expected non-zero exit")
	}
}

func TestRun_BareCommandHelpBackfill(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"init", "--help"}, "Inventory the MCP configs"},
		{[]string{"lock", "--help"}, "descriptor lockfile"},
		{[]string{"verify-lock", "--help"}, "report drift"},
		{[]string{"redteam", "--help"}, "synthetic red-team fixture packs"},
		{[]string{"serve", "--help"}, "governs routed tools"},
		{[]string{"verify", "--help"}, "Validate YAML policy files"},
		{[]string{"verify-record", "--help"}, "record.json is required"},
		{[]string{"audit", "--help"}, "Pretty-print structured decision records"},
		{[]string{"trust", "--help"}, "trust state Boundary consults"},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		code := Run(tc.args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v: expected exit 0, got %d", tc.args, code)
		}
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, tc.want) {
			t.Fatalf("%v: help missing %q:\n%s", tc.args, tc.want, combined)
		}
		if !strings.Contains(combined, "Usage:") {
			t.Fatalf("%v: help missing Usage section:\n%s", tc.args, combined)
		}
	}
}

func TestRun_VerifyJSONOutput(t *testing.T) {
	dir := t.TempDir()
	writeTestPolicy(t, dir)

	var stdout bytes.Buffer
	code := Run([]string{"verify", "--policies", dir, "--json"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}
	var payload struct {
		SchemaVersion string   `json:"schema_version"`
		OK            bool     `json:"ok"`
		Error         string   `json:"error"`
		PolicyFiles   int      `json:"policy_files"`
		Rules         int      `json:"rules"`
		Warnings      []string `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("verify --json did not parse: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.verify.v1" {
		t.Fatalf("schema_version = %q", payload.SchemaVersion)
	}
	if !payload.OK || payload.PolicyFiles != 1 || payload.Rules != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Warnings == nil {
		t.Fatalf("warnings must encode as an array, not null: %s", stdout.String())
	}

	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "broken.yaml"), []byte(":\tnot yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	code = Run([]string{"verify", "--policies", empty, "--json"}, &stdout, &bytes.Buffer{})
	if code == 0 {
		t.Fatalf("expected parse failure to exit non-zero")
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("failure JSON did not parse: %v\n%s", err, stdout.String())
	}
	if payload.OK || payload.Error == "" {
		t.Fatalf("failure payload must set ok=false with error: %+v", payload)
	}
}

func TestRun_VerifyRecordJSONOutput(t *testing.T) {
	dir := t.TempDir()
	writeTestPolicy(t, dir)
	requestBody := []byte(`{"agent_id":"agent-1","arguments":{"sql":"SELECT 1"},"tenant_id":"tenant-1","tool_name":"query"}`)
	requestHash, err := governance.ComputeRawRequestHash(requestBody)
	if err != nil {
		t.Fatal(err)
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
	recordPath := filepath.Join(dir, "record.json")
	writeRecordFile(t, recordPath, record)

	var stdout bytes.Buffer
	code := Run([]string{"verify-record", "--json", recordPath}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		OK            bool   `json:"ok"`
		Error         string `json:"error"`
		RecordID      string `json:"record_id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("verify-record --json did not parse: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.verify_record.v1" {
		t.Fatalf("schema_version = %q", payload.SchemaVersion)
	}
	if !payload.OK || payload.RecordID == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	record.Action = "deny"
	writeRecordFile(t, recordPath, record)
	stdout.Reset()
	code = Run([]string{"verify-record", "--json", recordPath}, &stdout, &bytes.Buffer{})
	if code == 0 {
		t.Fatalf("expected tampered record to exit non-zero")
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("failure JSON did not parse: %v\n%s", err, stdout.String())
	}
	if payload.OK || !strings.Contains(payload.Error, "decision_hash") {
		t.Fatalf("failure payload must set ok=false with decision_hash error: %+v", payload)
	}
}

func writeTestPolicy(t *testing.T, dir string) {
	t.Helper()
	policy := []byte(`name: test-policy
version: "1.0"
rules:
  - name: block-drop-table
    tool: query
    action: deny
    reason: blocked
    match:
      field: arguments.sql
      contains: DROP TABLE
`)
	if err := os.WriteFile(filepath.Join(dir, "postgres.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRun_SubcommandHelpExitsZero(t *testing.T) {
	code := Run([]string{"serve", "--help"}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRun_VerifyPolicyDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTestPolicy(t, dir)

	var stdout bytes.Buffer
	code := Run([]string{"verify", "--policies", dir}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "policy files: 1") {
		t.Fatalf("verify output missing file count: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "rules: 1") {
		t.Fatalf("verify output missing rule count: %s", stdout.String())
	}
}

func TestRun_VerifyPolicyDirectoryRejectsInvalidV1(t *testing.T) {
	dir := t.TempDir()
	policy := []byte(`schema_version: "1"
policy:
  name: broken
  version: "1.0.0"
  rules:
    - name: invalid
      tool: query
      action: deny
      conditions:
        - type: regex
          field: arguments.sql
          regex: "["
`)
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := Run([]string{"verify", "--policies", dir}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatalf("expected invalid v1 policy to fail verification")
	}
	if !strings.Contains(stderr.String(), "invalid regex") {
		t.Fatalf("expected schema error, got %s", stderr.String())
	}
}

func TestRun_VerifyRecordAcceptsValidAndRejectsTampered(t *testing.T) {
	dir := t.TempDir()
	writeTestPolicy(t, dir)
	requestBody := []byte(`{"agent_id":"agent-1","arguments":{"sql":"SELECT 1"},"tenant_id":"tenant-1","tool_name":"query"}`)
	requestHash, err := governance.ComputeRawRequestHash(requestBody)
	if err != nil {
		t.Fatal(err)
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

	requestPath := filepath.Join(dir, "request.json")
	recordPath := filepath.Join(dir, "record.json")
	writeJSONFile(t, requestPath, requestBody)
	writeRecordFile(t, recordPath, record)

	var stdout bytes.Buffer
	code := Run([]string{"verify-record", "--request", requestPath, "--policies", dir, "--binary-digest", "sha256:test-build", recordPath}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected valid record to verify")
	}
	if !strings.Contains(stdout.String(), "record verification: ok") {
		t.Fatalf("missing success output: %s", stdout.String())
	}

	record.Action = "deny"
	writeRecordFile(t, recordPath, record)
	var stderr bytes.Buffer
	code = Run([]string{"verify-record", "--request", requestPath, "--policies", dir, "--binary-digest", "sha256:test-build", recordPath}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatalf("expected tampered record to fail verification")
	}
	if !strings.Contains(stderr.String(), "decision_hash mismatch") {
		t.Fatalf("expected decision hash failure, got %s", stderr.String())
	}
}

func TestRun_VerifyRecordVerifiesSignature(t *testing.T) {
	dir := t.TempDir()

	// A deterministic seed -> signer -> public key, so the test is reproducible.
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	signer, err := governance.NewEd25519SignerFromSeed(seed, "")
	if err != nil {
		t.Fatal(err)
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	pubHex := hex.EncodeToString(pub)

	record := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:  governance.TransportMCP,
		ToolName:   "query",
		Action:     "deny",
		Reason:     "blocked",
		TrustScore: 1,
		TrustState: governance.TrustStateTrusted.String(),
		Timestamp:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	})
	signature, err := signer.Sign(record)
	if err != nil {
		t.Fatal(err)
	}
	record.Signature = signature
	record.SignatureKeyID = signer.KeyID()

	recordPath := filepath.Join(dir, "record.json")
	writeRecordFile(t, recordPath, record)

	// Valid signature with --public-key as a literal hex key: passes.
	var stdout bytes.Buffer
	code := Run([]string{"verify-record", "--verify-signature", "--public-key", pubHex, recordPath}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected signed record to verify, got exit %d: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "record verification: ok") {
		t.Fatalf("missing success output: %s", stdout.String())
	}

	// --public-key as a file path: also passes.
	pubFile := filepath.Join(dir, "key.pub")
	writeJSONFile(t, pubFile, []byte(pubHex+"\n"))
	stdout.Reset()
	code = Run([]string{"verify-record", "--verify-signature", "--public-key", pubFile, recordPath}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected file public key to verify, got exit %d: %s", code, stdout.String())
	}

	// --verify-signature without --public-key fails closed.
	var stderr bytes.Buffer
	code = Run([]string{"verify-record", "--verify-signature", recordPath}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatal("expected --verify-signature without --public-key to fail")
	}
	if !strings.Contains(stderr.String(), "requires --public-key") {
		t.Fatalf("expected missing-key error, got %s", stderr.String())
	}

	// Wrong public key fails closed.
	otherSeed := make([]byte, 32)
	for i := range otherSeed {
		otherSeed[i] = byte(0xff - i)
	}
	otherPub := ed25519.NewKeyFromSeed(otherSeed).Public().(ed25519.PublicKey)
	stderr.Reset()
	code = Run([]string{"verify-record", "--verify-signature", "--public-key", hex.EncodeToString(otherPub), recordPath}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatal("expected wrong public key to fail signature verification")
	}
	if !strings.Contains(stderr.String(), "signature verification failed") {
		t.Fatalf("expected signature failure, got %s", stderr.String())
	}

	// Tampering a covered field fails (decision_hash mismatch is caught first).
	record.Action = "allow"
	writeRecordFile(t, recordPath, record)
	stderr.Reset()
	code = Run([]string{"verify-record", "--verify-signature", "--public-key", pubHex, recordPath}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatal("expected tampered signed record to fail")
	}

	// Default verification (no --verify-signature) ignores the signature: an
	// unsigned record still verifies, so signing stays opt-in.
	unsigned := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:  governance.TransportMCP,
		ToolName:   "query",
		Action:     "deny",
		Reason:     "blocked",
		TrustScore: 1,
		TrustState: governance.TrustStateTrusted.String(),
		Timestamp:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	})
	unsignedPath := filepath.Join(dir, "unsigned.json")
	writeRecordFile(t, unsignedPath, unsigned)
	stdout.Reset()
	code = Run([]string{"verify-record", unsignedPath}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected unsigned record to verify without --verify-signature, got %d: %s", code, stdout.String())
	}
}

func TestGatewayMiddleware_AllowsSelectAndBlocksDrop(t *testing.T) {
	rules := []governance.StaticPolicyRule{
		{
			Name:   "block-drop-table",
			Tool:   "query",
			Action: "deny",
			Reason: "blocked",
			Match: &governance.StaticPolicyMatch{
				Field:           "arguments.sql",
				Contains:        "DROP TABLE",
				CaseInsensitive: true,
			},
			PolicyFile: "postgres.yaml",
		},
	}
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: rules,
		GatewayVersion: "test-version",
	}, nil, nil, nil)

	var downstreamCalls int
	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downstreamCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	middleware := governance.NewMiddleware(pipeline, downstream, governance.MiddlewareConfig{
		TransportType:  governance.TransportMCP,
		RequestBuilder: buildPostgresGovernanceRequest,
	})

	selectReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"tool_name":"query","arguments":{"sql":"SELECT * FROM users"}}`))
	selectRec := httptest.NewRecorder()
	middleware.ServeHTTP(selectRec, selectReq)
	if selectRec.Code != http.StatusOK {
		t.Fatalf("expected SELECT to pass, got %d body=%s", selectRec.Code, selectRec.Body.String())
	}

	dropReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"tool_name":"query","arguments":{"sql":"DROP TABLE users"}}`))
	dropRec := httptest.NewRecorder()
	middleware.ServeHTTP(dropRec, dropReq)
	if dropRec.Code != http.StatusForbidden {
		t.Fatalf("expected DROP TABLE to be blocked, got %d body=%s", dropRec.Code, dropRec.Body.String())
	}
	if !strings.Contains(dropRec.Body.String(), "block-drop-table") {
		t.Fatalf("deny body missing matched rule: %s", dropRec.Body.String())
	}
	if downstreamCalls != 1 {
		t.Fatalf("expected downstream to be called once, got %d", downstreamCalls)
	}
}

func writeJSONFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeRecordFile(t *testing.T, path string, record governance.DecisionRecordV1) {
	t.Helper()
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, path, body)
}

// TestRun_ServeHelpListsReceiptSeed asserts the serve help advertises the
// opt-in signing flag and keeps the honest authorship caveat alongside it.
func TestRun_ServeHelpListsReceiptSeed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"serve", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	combined := stdout.String() + stderr.String()
	for _, want := range []string{
		"--receipt-seed",
		"signing is off by default",
		"proves who signed the record, not the verdict",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("serve help missing %q:\n%s", want, combined)
		}
	}
}

// TestRun_ServeReceiptSeedFailsClosed verifies that requesting signing with a
// missing, short, or non-hex seed file makes `serve` exit 1 with the signing
// error on stderr and never reaches the listen step — i.e. it does not serve
// unsigned when signing was requested. To prove the listener is never opened,
// each case is handed a --listen address that is already bound: if runServe
// reached http.Server.ListenAndServe it would surface a bind error ("server
// error") or the "listening" banner, neither of which may appear.
func TestRun_ServeReceiptSeedFailsClosed(t *testing.T) {
	// Empty policy dir so policy load + trust backend succeed and the only
	// remaining failure is the seed (an empty dir reads as zero rules).
	policyDir := t.TempDir()

	shortSeed := filepath.Join(t.TempDir(), "short.seed")
	if err := os.WriteFile(shortSeed, []byte("deadbeef"), 0o600); err != nil {
		t.Fatal(err)
	}
	nonHexSeed := filepath.Join(t.TempDir(), "nonhex.seed")
	if err := os.WriteFile(nonHexSeed, []byte(strings.Repeat("z", 64)), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		seed string
	}{
		{"missing", filepath.Join(t.TempDir(), "does-not-exist.seed")},
		{"short", shortSeed},
		{"non-hex", nonHexSeed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Occupy a real port and feed it as --listen; ListenAndServe must
			// never run, so this address stays the only thing bound to it.
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("reserve port: %v", err)
			}
			defer ln.Close()

			var stdout, stderr bytes.Buffer
			code := Run([]string{
				"serve",
				"--policies", policyDir,
				"--listen", ln.Addr().String(),
				"--receipt-seed", tc.seed,
			}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("expected exit 1 (fail closed), got %d; stderr=%s", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "receipt signing requested but seed could not be loaded") {
				t.Fatalf("stderr missing fail-closed signing error:\n%s", stderr.String())
			}
			// Proof the listen step was never reached: no bind error, no banner.
			if strings.Contains(stderr.String(), "server error") || strings.Contains(stderr.String(), "listening on") {
				t.Fatalf("serve reached the listen step despite a bad seed:\n%s", stderr.String())
			}
		})
	}
}

// TestRun_ServeReceiptSeedValidParsesAndSigns is a flag-parse + wiring check for
// the success path: a valid 64-hex seed is accepted (no signing-load error) and
// the resulting signer signs decision records. It does not start the server
// (ListenAndServe blocks); the served-deny-emits-signed-record end-to-end is not
// covered here because runServe does not surface its handler or chosen port —
// signing emission itself is covered at the pipeline level. This builds the same
// signer runServe would build from the flag and exercises it directly.
func TestRun_ServeReceiptSeedValidParsesAndSigns(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	seedHex := hex.EncodeToString(priv.Seed())
	seedPath := filepath.Join(t.TempDir(), "valid.seed")
	if err := os.WriteFile(seedPath, []byte(seedHex+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	signer, err := governance.NewEd25519SignerFromSeedFile(seedPath, "")
	if err != nil {
		t.Fatalf("valid seed must load, got: %v", err)
	}
	rec := governance.DecisionRecordV1{DecisionHash: "sha256:" + strings.Repeat("ab", 32)}
	sig, err := signer.Sign(rec)
	if err != nil {
		t.Fatalf("signer must sign, got: %v", err)
	}
	if !strings.HasPrefix(sig, "ed25519:") {
		t.Fatalf("signature missing ed25519 prefix: %q", sig)
	}
	if signer.KeyID() == "" {
		t.Fatal("signer must expose a non-empty key id")
	}
}
