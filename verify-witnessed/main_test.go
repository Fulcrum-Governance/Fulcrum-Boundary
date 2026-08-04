package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Fulcrum-Governance/Fulcrum-Boundary/verify-witnessed/verifier"
)

const (
	fulcrumKeyID = "ed25519:fulcrum-sth-golden-v1"
	fulcrumKey   = "ed25519:5QLVNf3J3whhk2HEt65F7ryZnBDEDbhO4BPwTCPZ8fM="
	alphaKeyID   = "ed25519:witness-alpha-golden-v1"
	alphaKey     = "ed25519:Pt4yuMyqTObnl3l5in9VumGMSWRYdWds/nyblo0VcV0="
	betaKeyID    = "ed25519:witness-beta-golden-v1"
	betaKey      = "ed25519:msQEJRe271khfax7KCgOAy+v9ahJRElElUkFlRjNRk4="
)

func TestJSONOutputMatchesFrozenFilesExactly(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
	}{
		{name: "valid", exitCode: 0},
		{name: "leaf-tamper", exitCode: 1},
		{name: "cosignature-tamper", exitCode: 1},
		{name: "partial-1-of-2", exitCode: 0},
		{name: "sth-signature-tamper", exitCode: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			gotCode := run([]string{
				cliCorpusPath(t, "bundles", test.name),
				"--keyring", cliCorpusPath(t, "public-keys-v1.json"),
				"--json",
			}, &stdout, &stderr)
			if gotCode != test.exitCode {
				t.Fatalf("run() exit = %d, want %d; stderr=%s", gotCode, test.exitCode, &stderr)
			}
			if stderr.Len() != 0 {
				t.Fatalf("run() stderr = %q, want empty", &stderr)
			}
			want := cliReadFile(t, cliCorpusPath(t, "expected", test.name+".json"))
			if !bytes.Equal(stdout.Bytes(), want) {
				t.Fatalf("run() output mismatch\n got: %s\nwant: %s", &stdout, want)
			}
		})
	}
}

func TestDefaultOutputIsOneCheckPerLineWithDirectKeys(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		cliCorpusPath(t, "bundles", "valid"),
		"--fulcrum-pubkey", fulcrumKeyID + "=" + fulcrumKey,
		"--witness-pubkey", alphaKeyID + "=" + alphaKey,
		"--witness-pubkey", betaKeyID + "=" + betaKey,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d; stderr=%s", code, &stderr)
	}
	if stderr.Len() != 0 {
		t.Fatalf("run() stderr = %q, want empty", &stderr)
	}

	lines := bytes.Split(bytes.TrimSuffix(stdout.Bytes(), []byte{'\n'}), []byte{'\n'})
	if got, want := len(lines), 12; got != want {
		t.Fatalf("default output lines = %d, want %d\n%s", got, want, &stdout)
	}
	for i, line := range lines {
		var check verifier.Check
		if err := json.Unmarshal(line, &check); err != nil {
			t.Fatalf("line %d is not a check object: %v", i+1, err)
		}
		if check.ID == "" || check.Status == "" {
			t.Fatalf("line %d lacks stable id/status: %s", i+1, line)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(line, &fields); err != nil {
			t.Fatal(err)
		}
		if _, collapsed := fields["verified"]; collapsed {
			t.Fatalf("line %d contains forbidden aggregate verified field", i+1)
		}
	}
}

func TestDirectAndKeyringSourcesMergeSafely(t *testing.T) {
	validBundle := cliCorpusPath(t, "bundles", "valid")
	keyring := cliCorpusPath(t, "public-keys-v1.json")
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{
			name: "identical duplicate",
			args: []string{validBundle, "--keyring", keyring,
				"--fulcrum-pubkey", fulcrumKeyID + "=" + fulcrumKey,
				"--witness-pubkey=" + alphaKeyID + "=" + alphaKey},
			wantCode: 0,
		},
		{
			name: "repeatable identical direct flag",
			args: append(directKeyArgs(validBundle),
				"--witness-pubkey", alphaKeyID+"="+alphaKey),
			wantCode: 0,
		},
		{
			name: "conflicting bytes",
			args: []string{validBundle, "--keyring", keyring,
				"--fulcrum-pubkey", fulcrumKeyID + "=" + alphaKey},
			wantCode: 2, wantStderr: "conflicting duplicate key ID",
		},
		{
			name: "conflicting role",
			args: []string{validBundle, "--keyring", keyring,
				"--witness-pubkey", fulcrumKeyID + "=" + fulcrumKey},
			wantCode: 2, wantStderr: "conflicting duplicate key ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			gotCode := run(test.args, &stdout, &stderr)
			if gotCode != test.wantCode {
				t.Fatalf("run() exit = %d, want %d; stderr=%s", gotCode, test.wantCode, &stderr)
			}
			if test.wantStderr != "" && !strings.Contains(stderr.String(), test.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", &stderr, test.wantStderr)
			}
		})
	}
}

func TestCommandInputAndOutputErrors(t *testing.T) {
	validBundle := cliCorpusPath(t, "bundles", "valid")
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing directory"},
		{name: "unknown option", args: []string{"--unknown"}},
		{name: "multiple directories", args: []string{validBundle, validBundle}},
		{name: "missing option value", args: []string{validBundle, "--keyring"}},
		{name: "malformed direct key", args: []string{validBundle, "--witness-pubkey", "bad"}},
		{name: "no keys", args: []string{validBundle}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(test.args, &stdout, &stderr); code != 2 {
				t.Fatalf("run() exit = %d, want 2", code)
			}
			if stderr.Len() == 0 {
				t.Fatal("run() did not explain input error")
			}
		})
	}

	var help bytes.Buffer
	if code := run([]string{"--help"}, &help, io.Discard); code != 0 || help.String() != usageText {
		t.Fatalf("help = (exit %d, %q)", code, &help)
	}

	if code := run(append(directKeyArgs(validBundle), "--json"), failingWriter{}, io.Discard); code != 1 {
		t.Fatalf("run() with failing output exit = %d, want 1", code)
	}
	if code := run(directKeyArgs(filepath.Join(t.TempDir(), "missing")), io.Discard, io.Discard); code != 1 {
		t.Fatalf("run() with missing bundle exit = %d, want 1", code)
	}
}

func TestAirgappedSubprocess(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestAirgappedHelper$")
	command.Env = []string{
		"FULCRUM_VERIFY_HELPER=1",
		"FULCRUM_VERIFY_BUNDLE=" + cliCorpusPath(t, "bundles", "valid"),
		"FULCRUM_VERIFY_KEYRING=" + cliCorpusPath(t, "public-keys-v1.json"),
		"HOME=" + t.TempDir(),
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("air-gapped subprocess failed: %v\n%s", err, output)
	}
}

func TestAirgappedHelper(t *testing.T) {
	if os.Getenv("FULCRUM_VERIFY_HELPER") != "1" {
		return
	}
	for _, name := range []string{
		"DATABASE_URL", "REDIS_URL", "NATS_URL", "FULCRUM_API_KEY",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "GOOGLE_APPLICATION_CREDENTIALS",
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "GOPROXY",
	} {
		if _, present := os.LookupEnv(name); present {
			t.Fatalf("network or credential environment variable %s is present", name)
		}
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{
		os.Getenv("FULCRUM_VERIFY_BUNDLE"),
		"--keyring", os.Getenv("FULCRUM_VERIFY_KEYRING"),
		"--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("air-gapped run exit = %d; stderr=%s", code, &stderr)
	}
	want := cliReadFile(t, cliCorpusPath(t, "expected", "valid.json"))
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("air-gapped output does not match frozen expected result")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("test output failure")
}

func directKeyArgs(bundle string) []string {
	return []string{
		bundle,
		"--fulcrum-pubkey", fulcrumKeyID + "=" + fulcrumKey,
		"--witness-pubkey", alphaKeyID + "=" + alphaKey,
		"--witness-pubkey", betaKeyID + "=" + betaKey,
	}
}

func cliCorpusPath(t *testing.T, elements ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	base := filepath.Clean(filepath.Join(filepath.Dir(filename), "testdata", "witnessed-log-v1"))
	return filepath.Join(append([]string{base}, elements...)...)
}

func cliReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func Example_lineOutput() {
	check := verifier.Check{ID: "source_hash_to_leaf", Status: verifier.StatusPass}
	data, _ := json.Marshal(check)
	fmt.Println(string(data))
	// Output: {"id":"source_hash_to_leaf","status":"pass"}
}
