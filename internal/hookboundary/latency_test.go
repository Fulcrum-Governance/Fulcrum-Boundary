package hookboundary

import (
	"sort"
	"strings"
	"testing"
	"time"
)

// allowEvent is the hot path a coding session hits constantly: a routed shell
// tool call that Boundary allows.
const allowEvent = `{"hook_event_name":"PreToolUse","session_id":"sess-bench","cwd":"/repo",` +
	`"tool_name":"Bash","tool_input":{"command":"git status"}}`

// allowPathBudget is the per-call latency the allow path must stay under. The
// hook sits in front of every routed tool call, so a budget that a human would
// notice is a product defect, not a tuning detail.
const allowPathBudget = 50 * time.Millisecond

// TestDecideAllowPathStaysUnderLatencyBudget measures the real allow path — real
// classifier, real record sink writing to a temp directory — and asserts the p95
// stays under allowPathBudget. Nothing is stubbed: the measurement includes the
// JSON parse, the classification, the canonical-JSON hashing, and both record
// writes, because that is what the hook actually costs per tool call.
func TestDecideAllowPathStaysUnderLatencyBudget(t *testing.T) {
	const iterations = 200
	cfg := Config{Dir: t.TempDir(), BoundaryVersion: "v-test"}

	// One warm-up decision so the first-call directory creation and lazy
	// package initialization are not attributed to the measured sample.
	if result := Decide(cfg, strings.NewReader(allowEvent)); result.Record == nil {
		t.Fatalf("warm-up produced no record; fault=%v sink=%v", result.Fault, result.SinkError)
	}

	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		start := time.Now()
		result := Decide(cfg, strings.NewReader(allowEvent))
		samples = append(samples, time.Since(start))
		if result.Verdict != VerdictAllow {
			t.Fatalf("iteration %d verdict = %q, want allow", i, result.Verdict)
		}
		if result.Record == nil {
			t.Fatalf("iteration %d persisted no record; sink=%v", i, result.SinkError)
		}
		if result.Stdout != nil {
			t.Fatalf("iteration %d emitted stdout on a silent allow: %q", i, result.Stdout)
		}
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[(len(samples)*95)/100]
	if p95 >= allowPathBudget {
		t.Fatalf("allow-path p95 = %v, want under %v (median %v, max %v)",
			p95, allowPathBudget, samples[len(samples)/2], samples[len(samples)-1])
	}
	t.Logf("allow path over %d decisions: p50=%v p95=%v max=%v",
		iterations, samples[len(samples)/2], p95, samples[len(samples)-1])
}

// BenchmarkDecideAllowPath reports the same path go test -bench can track over
// time. It writes real records into a temp directory, so the reported cost is
// the whole hook, not the classifier alone.
func BenchmarkDecideAllowPath(b *testing.B) {
	cfg := Config{Dir: b.TempDir(), BoundaryVersion: "v-test"}
	reader := strings.NewReader(allowEvent)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(allowEvent)
		if result := Decide(cfg, reader); result.Verdict != VerdictAllow {
			b.Fatalf("verdict = %q", result.Verdict)
		}
	}
}

// BenchmarkClassifyBashLine isolates the classifier so a regression can be
// attributed to routing rather than to record I/O.
func BenchmarkClassifyBashLine(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ClassifyBashLine("git status"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkClassifyCompoundBashLine tracks the decomposition path, which does
// strictly more work than a single command and is the one the hook now takes
// for every chained line.
func BenchmarkClassifyCompoundBashLine(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ClassifyBashLine("git status && npm ci && rm -rf dist"); err != nil {
			b.Fatal(err)
		}
	}
}
