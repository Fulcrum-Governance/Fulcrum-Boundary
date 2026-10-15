package commandboundary

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// FUL-653: `command run` classifies one argv, so `find -exec <cmd>` and
// wrapper forms like `xargs <cmd>` must be governed by the command they
// forward, not by the forwarding utility's own class.

// blocking reports whether an action blocks execution under the preview policy.
func blocking(action RecommendedAction) bool {
	return action == ActionDeny || action == ActionRequireApproval
}

func TestClassifyArgvFindExecForwardsToShellOrInterpreter(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"FUL-653 repro: -exec /bin/sh -c tool ;", []string{"find", "/tmp", "-maxdepth", "0", "-exec", "/bin/sh", "-c", "/opt/fulcrum/tools/fulcrum-demo-tool", ";"}},
		{"-exec sh -c with plus", []string{"find", ".", "-exec", "sh", "-c", "id", "+"}},
		{"-execdir bash", []string{"find", ".", "-execdir", "bash", "-c", "id", ";"}},
		{"-ok sh", []string{"find", ".", "-ok", "sh", "-c", "id", ";"}},
		{"-okdir sh", []string{"find", ".", "-okdir", "sh", "-c", "id", ";"}},
		{"-exec with no terminator", []string{"find", ".", "-exec", "/bin/sh"}},
		{"-exec python inline", []string{"find", ".", "-exec", "python3", "-c", "print(1)", ";"}},
		{"-exec node", []string{"find", ".", "-exec", "node", "-e", "1", ";"}},
		{"-exec unclassified absolute tool", []string{"find", "/opt/fulcrum/tools", "-name", "fulcrum-demo-tool", "-exec", "/opt/fulcrum/tools/fulcrum-demo-tool", ";"}},
		{"uppercase flag", []string{"find", ".", "-EXEC", "sh", "-c", "id", ";"}},
		{"absolute /usr/bin/find", []string{"/usr/bin/find", ".", "-exec", "sh", "-c", "id", ";"}},
		{"xargs sh", []string{"xargs", "-n", "1", "sh", "-c", "id"}},
		{"xargs python3", []string{"xargs", "python3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Classify(tt.argv)
			if err != nil {
				t.Fatalf("Classify(%q): %v", tt.argv, err)
			}
			if !blocking(got.RecommendedAction) {
				t.Fatalf("Classify(%q) = class %s action %s (%q), want deny or require_approval",
					tt.argv, got.Class, got.RecommendedAction, got.Reason)
			}
			if got.Class == ClassObserveRead {
				t.Fatalf("Classify(%q) stayed C0 observe", tt.argv)
			}
		})
	}
}

func TestClassifyArgvFindExecDestructivePayloadDenies(t *testing.T) {
	for _, argv := range [][]string{
		{"find", ".", "-exec", "rm", "-rf", "{}", "+"},
		{"find", ".", "-execdir", "rm", "-rf", "{}", ";"},
		{"find", ".", "-ok", "rm", "{}", ";"},
		{"find", ".", "-exec", "sh", "-c", "rm -rf /", ";"},
		{"xargs", "rm", "-rf"},
		{"env", "rm", "-rf", "dist"},
	} {
		got, err := Classify(argv)
		if err != nil {
			t.Fatalf("Classify(%q): %v", argv, err)
		}
		if got.RecommendedAction != ActionDeny || got.Class != ClassDestructiveMutation {
			t.Fatalf("Classify(%q) = class %s action %s, want %s deny", argv, got.Class, got.RecommendedAction, ClassDestructiveMutation)
		}
		if got.Command != argv[0] {
			t.Fatalf("Classify(%q).Command = %q, want the invoked command %q", argv, got.Command, argv[0])
		}
	}
}

// Forwarding to an observe command must not escalate, and find without an
// exec primary stays observe: the fix only raises verdicts.
func TestClassifyArgvFindWithoutForwardingUnchanged(t *testing.T) {
	for _, argv := range [][]string{
		{"find", ".", "-name", "*.go"},
		{"find", ".", "-name", "x", "-exec", "cat", "{}", ";"},
		{"find", ".", "-exec", "ls", "{}", "+"},
	} {
		got, err := Classify(argv)
		if err != nil {
			t.Fatalf("Classify(%q): %v", argv, err)
		}
		if got.Class != ClassObserveRead || got.RecommendedAction != ActionAllow {
			t.Fatalf("Classify(%q) = class %s action %s, want C0 allow", argv, got.Class, got.RecommendedAction)
		}
	}
}

// The argv path and the line path must agree on the action for forwarding
// shapes both model.
func TestClassifyArgvAgreesWithLineForFindExec(t *testing.T) {
	for _, tc := range []struct {
		argv []string
		line string
	}{
		{[]string{"find", ".", "-exec", "rm", "-rf", "{}", "+"}, "find . -exec rm -rf {} +"},
		{[]string{"find", ".", "-exec", "sh", "-c", "id", ";"}, `find . -exec sh -c id \;`},
		{[]string{"xargs", "rm", "-rf"}, "xargs rm -rf"},
	} {
		argvGot, err := Classify(tc.argv)
		if err != nil {
			t.Fatal(err)
		}
		lineGot, err := ClassifyLine(tc.line)
		if err != nil {
			t.Fatal(err)
		}
		if argvGot.RecommendedAction != lineGot.Aggregate.RecommendedAction {
			t.Fatalf("argv %q action %s != line %q action %s", tc.argv, argvGot.RecommendedAction, tc.line, lineGot.Aggregate.RecommendedAction)
		}
	}
}

// End to end through the `command run` executor: the forwarded command never
// reaches the runner, the verdict is deny or require_approval, and a decision
// record (receipt) is written with executed=false and the escalated class.
func TestExecutorFindExecForwardingBlocksWithRecord(t *testing.T) {
	for _, argv := range [][]string{
		{"find", "/tmp", "-maxdepth", "0", "-exec", "/bin/sh", "-c", "/opt/fulcrum/tools/fulcrum-demo-tool", ";"},
		{"find", ".", "-execdir", "python3", "-c", "print(1)", "+"},
		{"find", ".", "-ok", "sh", "-c", "id", ";"},
		{"find", ".", "-exec", "rm", "-rf", "{}", "+"},
		{"xargs", "sh", "-c", "id"},
	} {
		recordPath := filepath.Join(t.TempDir(), "records.jsonl")
		runner := &fakeRunner{}
		executor := Executor{Runner: runner, RecordPath: recordPath}
		result, err := executor.Run(context.Background(), RunRequest{Argv: argv, CWD: t.TempDir()})
		if err != nil {
			t.Fatalf("run %q: %v", argv, err)
		}
		if result.Executed || len(runner.calls) != 0 {
			t.Fatalf("run %q executed (calls=%d), want blocked", argv, len(runner.calls))
		}
		if a := result.Decision.Action; a != "deny" && a != "require_approval" {
			t.Fatalf("run %q action = %q, want deny or require_approval", argv, a)
		}
		record := readSingleCommandRecord(t, recordPath)
		if record.Executed {
			t.Fatalf("record for %q executed = true", argv)
		}
		if record.Action != result.Decision.Action {
			t.Fatalf("record action %q != decision action %q", record.Action, result.Decision.Action)
		}
		if record.Class == ClassObserveRead {
			t.Fatalf("record for %q kept C0 observe class", argv)
		}
	}
}

// Deeply nested forwarding must stay cheap and must never allow. Before the
// classifyOwn split, each nesting level re-decomposed its payload through
// Classify, so cost roughly doubled per level (about 0.4s at 20 levels,
// 1.1s at 22). 22 levels keeps that regression a fast assertion failure
// instead of a test-binary timeout.
func TestClassifyNestedForwardingIsBoundedAndNeverAllows(t *testing.T) {
	for _, wrapper := range [][]string{{"find", ".", "-exec"}, {"xargs"}, {"env"}} {
		argv := []string{}
		for i := 0; i < 22; i++ {
			argv = append(argv, wrapper...)
		}
		argv = append(argv, "sh")
		start := time.Now()
		got, err := Classify(argv)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if !blocking(got.RecommendedAction) {
			t.Fatalf("22x %q nesting = %s, want deny or require_approval", wrapper[0], got.RecommendedAction)
		}
		if elapsed > 100*time.Millisecond {
			t.Fatalf("22x %q nesting took %v; nested forwarding classification must not be exponential", wrapper[0], elapsed)
		}
		if _, err := ClassifyLine(joinWords(argv)); err != nil {
			t.Fatal(err)
		}
	}
}

func joinWords(words []string) string {
	out := ""
	for i, w := range words {
		if i > 0 {
			out += " "
		}
		out += w
	}
	return out
}
