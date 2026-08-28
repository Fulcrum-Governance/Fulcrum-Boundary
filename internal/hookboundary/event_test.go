package hookboundary

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadEventAcceptsAWellFormedEvent(t *testing.T) {
	body := `{"hook_event_name":"PreToolUse","session_id":"s-1","cwd":"/repo","tool_name":"Bash","tool_input":{"command":"git status"}}`
	raw, err := ReadEvent(strings.NewReader(body))
	if err != nil {
		t.Fatalf("ReadEvent: %v", err)
	}
	if string(raw) != body {
		t.Fatalf("raw bytes changed:\n got %s\nwant %s", raw, body)
	}
}

func TestReadEventRejectsEmptyStdin(t *testing.T) {
	for _, body := range []string{"", "   \n\t "} {
		if _, err := ReadEvent(strings.NewReader(body)); err == nil {
			t.Fatalf("ReadEvent(%q) = nil error, want empty-event error", body)
		}
	}
}

func TestReadEventRejectsNilReader(t *testing.T) {
	if _, err := ReadEvent(nil); err == nil {
		t.Fatal("ReadEvent(nil) = nil error, want error")
	}
}

// fillReader yields an unbounded stream of one byte so an oversized event can be
// exercised without materializing it in the test.
type fillReader byte

func (f fillReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(f)
	}
	return len(p), nil
}

func TestReadEventRejectsAnOversizedEvent(t *testing.T) {
	huge := io.MultiReader(
		strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"`),
		io.LimitReader(fillReader('a'), MaxEventBytes),
		strings.NewReader(`"}}`),
	)
	raw, err := ReadEvent(huge)
	if err == nil {
		t.Fatal("ReadEvent(oversized) = nil error, want size error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want an exceeds-limit error", err)
	}
	if len(raw) != MaxEventBytes {
		t.Fatalf("returned %d bytes, want the %d-byte prefix for the shape hash", len(raw), MaxEventBytes)
	}
}

func TestReadEventPropagatesAReaderFailure(t *testing.T) {
	want := errors.New("stdin exploded")
	_, err := ReadEvent(errReader{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want it to wrap %v", err, want)
	}
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func TestParseEventReadsTheFieldsBoundaryUses(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PreToolUse","session_id":"sess-42","cwd":"/repo",` +
		`"tool_name":"Write","tool_input":{"file_path":"a.go","content":"x"},"transcript_path":"/tmp/t.json"}`)
	event, err := ParseEvent(raw)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if event.HookEventName != "PreToolUse" || event.SessionID != "sess-42" ||
		event.CWD != "/repo" || event.ToolName != "Write" {
		t.Fatalf("parsed event = %#v", event)
	}
	input, err := event.Input()
	if err != nil {
		t.Fatalf("Input: %v", err)
	}
	if input.FilePath != "a.go" {
		t.Fatalf("file_path = %q", input.FilePath)
	}
}

func TestParseEventRejectsMalformedJSON(t *testing.T) {
	cases := map[string]string{
		"truncated":  `{"tool_name":"Bash"`,
		"not object": `["Bash"]`,
		"wrong type": `{"tool_name":42}`,
		"bare text":  `not json at all`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseEvent([]byte(raw)); err == nil {
				t.Fatalf("ParseEvent(%q) = nil error, want parse error", raw)
			}
		})
	}
}

func TestEventInputToleratesAMissingToolInput(t *testing.T) {
	for name, raw := range map[string]string{
		"absent": `{"tool_name":"Bash"}`,
		"null":   `{"tool_name":"Bash","tool_input":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			event, err := ParseEvent([]byte(raw))
			if err != nil {
				t.Fatalf("ParseEvent: %v", err)
			}
			input, err := event.Input()
			if err != nil {
				t.Fatalf("Input: %v", err)
			}
			if input != (ToolInput{}) {
				t.Fatalf("input = %#v, want zero value", input)
			}
		})
	}
}

func TestEventInputRejectsANonObjectToolInput(t *testing.T) {
	event, err := ParseEvent([]byte(`{"tool_name":"Bash","tool_input":"git status"}`))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if _, err := event.Input(); err == nil {
		t.Fatal("Input() = nil error for a string tool_input, want error")
	}
}

func TestToolInputEditPathPrefersFilePathThenNotebookPath(t *testing.T) {
	cases := []struct {
		name  string
		input ToolInput
		want  string
	}{
		{"file path", ToolInput{FilePath: "a.go"}, "a.go"},
		{"notebook path", ToolInput{NotebookPath: "nb.ipynb"}, "nb.ipynb"},
		{"file path wins", ToolInput{FilePath: "a.go", NotebookPath: "nb.ipynb"}, "a.go"},
		{"blank file path falls through", ToolInput{FilePath: "  ", NotebookPath: "nb.ipynb"}, "nb.ipynb"},
		{"neither", ToolInput{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.input.EditPath(); got != tc.want {
				t.Fatalf("EditPath() = %q, want %q", got, tc.want)
			}
		})
	}
}
