package hookboundary

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// HookEventName is the only Claude Code hook event this package decides. An
// event that names a different hook is a wiring fault, not a decision.
const HookEventName = "PreToolUse"

// MaxEventBytes bounds how much of stdin ReadEvent will consume. Claude Code
// tool inputs carry file content (a Write payload, a MultiEdit body), so the cap
// is generous; it exists so a runaway or hostile producer cannot make the hook
// allocate without limit. An event larger than this is a fault, resolved through
// FailMode, not a silent allow.
const MaxEventBytes = 8 << 20 // 8 MiB

// Event is the subset of a Claude Code PreToolUse hook event Boundary reads.
// Unknown fields in the event JSON are ignored, so a client that adds fields
// does not break the hook.
//
// ToolInput is kept as raw JSON and decoded per route (see Event.Input) so this
// package never materializes the parts of a tool input it has no business
// reading — a Write tool's file content above all.
type Event struct {
	// HookEventName is the hook that fired. Empty is tolerated (older clients);
	// a value other than HookEventName is a fault.
	HookEventName string `json:"hook_event_name"`
	// SessionID is the Claude Code session the tool call belongs to. It is
	// untrusted input: it is sanitized and length-capped before it reaches a
	// record, and it is never used to build a filesystem path.
	SessionID string `json:"session_id"`
	// CWD is the working directory Claude Code reports for the session. It is
	// descriptive context only — records are written relative to the hook
	// process's own working directory, never to a path taken from the event.
	CWD string `json:"cwd"`
	// ToolName is the tool Claude Code selected, e.g. "Bash" or "Write".
	ToolName string `json:"tool_name"`
	// ToolInput is the tool's proposed arguments, decoded per route.
	ToolInput json.RawMessage `json:"tool_input"`
}

// ToolInput is the subset of a PreToolUse tool_input Boundary reads: the Bash
// command line and the edit target path. Content fields (a Write payload, the
// MultiEdit edit list, environment values) are deliberately absent — they are
// never parsed, never classified, and never persisted.
type ToolInput struct {
	// Command is the Bash tool's proposed command line.
	Command string `json:"command"`
	// FilePath is the Edit/Write/MultiEdit target path.
	FilePath string `json:"file_path"`
	// NotebookPath is the NotebookEdit target path.
	NotebookPath string `json:"notebook_path"`
}

// EditPath returns the edit route's target path: file_path when present,
// otherwise notebook_path. It returns "" when the input names neither, which the
// caller treats as a fault (there is nothing to classify).
func (in ToolInput) EditPath() string {
	if path := strings.TrimSpace(in.FilePath); path != "" {
		return path
	}
	return strings.TrimSpace(in.NotebookPath)
}

// ReadEvent reads one PreToolUse event from r, consuming at most MaxEventBytes.
// It returns the raw bytes so the caller can bind a fault record to the exact
// input shape Boundary observed (governance.ComputeRawShapeHash).
//
// It returns an error when r fails, when the input is empty, or when the input
// exceeds MaxEventBytes. All three are faults, not decisions. On the oversize
// error the returned bytes are the MaxEventBytes PREFIX, which is genuinely all
// Boundary observed; a shape hash over it binds to that prefix, not to the whole
// input the producer sent.
func ReadEvent(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, errors.New("no PreToolUse event reader")
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxEventBytes+1))
	if err != nil {
		return raw, fmt.Errorf("read PreToolUse event: %w", err)
	}
	if len(raw) > MaxEventBytes {
		return raw[:MaxEventBytes], fmt.Errorf("PreToolUse event exceeds %d bytes", MaxEventBytes)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return raw, errors.New("empty PreToolUse event on stdin")
	}
	return raw, nil
}

// ParseEvent decodes the fields of a PreToolUse event Boundary reads. Unknown
// fields are ignored by design. It returns an error when raw is not a JSON
// object or a read field has the wrong type.
func ParseEvent(raw []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(raw, &event); err != nil {
		return Event{}, fmt.Errorf("parse PreToolUse event: %w", err)
	}
	return event, nil
}

// Input decodes the event's tool_input into the subset Boundary reads. A missing
// or null tool_input decodes to the zero ToolInput without error — the caller
// decides whether "nothing to classify" is a fault for that route. A tool_input
// that is not a JSON object is an error.
func (e Event) Input() (ToolInput, error) {
	trimmed := bytes.TrimSpace(e.ToolInput)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ToolInput{}, nil
	}
	var input ToolInput
	if err := json.Unmarshal(trimmed, &input); err != nil {
		return ToolInput{}, fmt.Errorf("parse tool_input: %w", err)
	}
	return input, nil
}
