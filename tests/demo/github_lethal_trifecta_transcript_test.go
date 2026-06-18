package demo_test

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

// liveDemoStdout runs the no-retention demo (no --out, so no
// "decision record path:"/"log:" lines) and returns its stdout.
func liveDemoStdout(t *testing.T) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := boundarycli.Run([]string{"demo", "github-lethal-trifecta"}, &stdout, &stderr); code != 0 {
		t.Fatalf("demo exit = %d, stderr=%s", code, stderr.String())
	}
	return stdout.String()
}

// volatilePrefixes are lines whose value (random record id, sha256 digest) differs
// every run; they are matched by label, not by exact value.
var volatilePrefixes = []string{
	"decision record id: rec_",
	"decision hash: sha256:",
	"- [pass] decision_record_emitted: rec_",
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(strings.TrimSpace(s), p) {
			return true
		}
	}
	return false
}

// assertSubsetOfLive checks every non-prompt, non-blank, non-volatile line of the
// committed asset appears verbatim in live stdout; volatile lines match by label.
func assertSubsetOfLive(t *testing.T, asset, live string) {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(asset))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "$ ") || strings.TrimSpace(line) == "" {
			continue // shell prompt / blank: not program output
		}
		if hasAnyPrefix(line, volatilePrefixes) {
			label := strings.TrimSpace(line)
			label = label[:strings.IndexByte(label, ':')+1]
			if !strings.Contains(live, label) {
				t.Errorf("live output missing volatile label %q", label)
			}
			continue
		}
		if !strings.Contains(live, line) {
			t.Errorf("committed line not present in live output:\n  %q", line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan asset: %v", err)
	}
}

// TestRenderedTranscriptIsNotStaleAndMatchesLive is the genuine red: the readable
// transcript asset is the file that drifted. It must (a) never carry the legacy
// unqualified "decision record:" label (the stale line at transcript.txt:15), and
// (b) have its non-volatile lines be a subset of live demo stdout.
func TestRenderedTranscriptIsNotStaleAndMatchesLive(t *testing.T) {
	body, err := os.ReadFile("../../docs/assets/github-lethal-trifecta-demo.transcript.txt")
	if err != nil {
		t.Fatalf("read transcript asset: %v", err)
	}
	asset := string(body)
	// The legacy label is "decision record:" NOT followed by "id:"/"path:"/"log:".
	scanner := bufio.NewScanner(strings.NewReader(asset))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "decision record:") {
			t.Errorf("transcript carries the stale unqualified label %q; the writer emits 'decision record id:'", line)
		}
	}
	assertSubsetOfLive(t, asset, liveDemoStdout(t))
}

// TestExampleTranscriptMatchesLiveOutput is the acknowledged-green regression
// guard: examples/cli/demo-github-lethal-trifecta.txt is already a clean subset of
// live stdout (line 13 already reads "decision record id:"). This pins it so it
// can never regress to the unqualified label.
func TestExampleTranscriptMatchesLiveOutput(t *testing.T) {
	body, err := os.ReadFile("../../examples/cli/demo-github-lethal-trifecta.txt")
	if err != nil {
		t.Fatalf("read example transcript: %v", err)
	}
	assertSubsetOfLive(t, string(body), liveDemoStdout(t))
}

func TestRenderedDemoAssetsExist(t *testing.T) {
	for _, p := range []string{
		"../../docs/assets/github-lethal-trifecta-demo.gif",
		"../../docs/assets/github-lethal-trifecta-demo.mp4",
		"../../docs/assets/github-lethal-trifecta-demo.poster.png",
	} {
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("rendered demo asset missing: %s (%v)", p, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("rendered demo asset is empty: %s", p)
		}
	}
}
