package editboundary

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

type Operation string

const (
	OperationNoop   Operation = "noop"
	OperationAdd    Operation = "add"
	OperationModify Operation = "modify"
	OperationDelete Operation = "delete"
	OperationRename Operation = "rename"
)

type FileChange struct {
	OldPath      string
	NewPath      string
	Operation    Operation
	AddedLines   []string
	DeletedLines []string
	Binary       bool
	ModeChanged  bool
}

func ParsePatch(patch []byte) ([]FileChange, error) {
	if len(bytes.TrimSpace(patch)) == 0 {
		return nil, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(patch))
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	var changes []FileChange
	var current *FileChange
	finish := func() {
		if current == nil {
			return
		}
		finalizeChange(current)
		if current.OldPath != "" || current.NewPath != "" || current.Binary {
			changes = append(changes, *current)
		}
		current = nil
	}
	ensure := func() *FileChange {
		if current == nil {
			current = &FileChange{Operation: OperationModify}
		}
		return current
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			finish()
			oldPath, newPath := parseDiffGitLine(line)
			current = &FileChange{OldPath: oldPath, NewPath: newPath, Operation: OperationModify}
		case strings.HasPrefix(line, "--- "):
			ch := ensure()
			ch.OldPath = trimPatchPath(strings.TrimSpace(strings.TrimPrefix(line, "--- ")))
		case strings.HasPrefix(line, "+++ "):
			ch := ensure()
			ch.NewPath = trimPatchPath(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
		case strings.HasPrefix(line, "new file mode "):
			ensure().Operation = OperationAdd
		case strings.HasPrefix(line, "deleted file mode "):
			ensure().Operation = OperationDelete
		case strings.HasPrefix(line, "rename from "):
			ch := ensure()
			ch.Operation = OperationRename
			ch.OldPath = trimPatchPath(strings.TrimSpace(strings.TrimPrefix(line, "rename from ")))
		case strings.HasPrefix(line, "rename to "):
			ch := ensure()
			ch.Operation = OperationRename
			ch.NewPath = trimPatchPath(strings.TrimSpace(strings.TrimPrefix(line, "rename to ")))
		case strings.HasPrefix(line, "old mode ") || strings.HasPrefix(line, "new mode "):
			ensure().ModeChanged = true
		case strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch"):
			ch := ensure()
			ch.Binary = true
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			ensure().AddedLines = append(ensure().AddedLines, strings.TrimPrefix(line, "+"))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			ensure().DeletedLines = append(ensure().DeletedLines, strings.TrimPrefix(line, "-"))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse patch: %w", err)
	}
	finish()
	if len(changes) == 0 {
		return nil, fmt.Errorf("parse patch: no file changes found")
	}
	return changes, nil
}

func parseDiffGitLine(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return "", ""
	}
	return trimPatchPath(fields[2]), trimPatchPath(fields[3])
}

func trimPatchPath(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"`)
	if raw == "/dev/null" {
		return raw
	}
	raw = strings.TrimPrefix(raw, "a/")
	raw = strings.TrimPrefix(raw, "b/")
	return raw
}

func finalizeChange(ch *FileChange) {
	oldNull := ch.OldPath == "/dev/null"
	newNull := ch.NewPath == "/dev/null"
	switch {
	case oldNull && !newNull:
		ch.Operation = OperationAdd
	case !oldNull && newNull:
		ch.Operation = OperationDelete
	case ch.Operation == "":
		ch.Operation = OperationModify
	}
	if ch.NewPath == "" && ch.OldPath != "" && ch.Operation != OperationDelete {
		ch.NewPath = ch.OldPath
	}
	if ch.OldPath == "" && ch.NewPath != "" {
		ch.OldPath = ch.NewPath
	}
	if ch.Operation == "" {
		ch.Operation = OperationModify
	}
}

func (c FileChange) TargetPath() string {
	if c.Operation == OperationDelete {
		return c.OldPath
	}
	if c.NewPath != "" && c.NewPath != "/dev/null" {
		return c.NewPath
	}
	return c.OldPath
}
