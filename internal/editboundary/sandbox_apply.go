package editboundary

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ApplySummary struct {
	FilesTouched []string
}

type Applier interface {
	Apply(root string, patch []byte) (ApplySummary, error)
}

type InternalApplier struct{}

func (InternalApplier) Apply(root string, patch []byte) (ApplySummary, error) {
	changes, err := ParsePatch(patch)
	if err != nil {
		return ApplySummary{}, err
	}
	var summary ApplySummary
	for _, change := range changes {
		target, err := secureTarget(root, change.TargetPath())
		if err != nil {
			return summary, err
		}
		if change.Binary || change.ModeChanged || change.Operation == OperationRename {
			return summary, fmt.Errorf("unsupported patch operation for %s", RedactPath(change.TargetPath()))
		}
		switch change.Operation {
		case OperationAdd:
			if err := applyAdd(target, change); err != nil {
				return summary, err
			}
		case OperationModify, OperationNoop:
			if err := applyModify(target, change); err != nil {
				return summary, err
			}
		case OperationDelete:
			if err := os.Remove(target); err != nil {
				return summary, fmt.Errorf("delete %s: %w", RedactPath(change.TargetPath()), err)
			}
		default:
			return summary, fmt.Errorf("unsupported patch operation %s", change.Operation)
		}
		summary.FilesTouched = append(summary.FilesTouched, RedactPath(change.TargetPath()))
	}
	return summary, nil
}

func secureTarget(root, raw string) (string, error) {
	check := CheckProjectPath(raw)
	if !check.Safe || check.Path == "" || check.Path == "/dev/null" {
		if check.Reason == "" {
			check.Reason = "path is outside project scope"
		}
		return "", errors.New(check.Reason)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(check.Path)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes project root: %s", RedactPath(raw))
	}
	return targetAbs, nil
}

func applyAdd(target string, change FileChange) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("add target already exists: %s", RedactPath(change.TargetPath()))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	body := linesToBytes(addedLinesFromHunks(change.Hunks, change.AddedLines), true)
	// #nosec G703 -- target path is constrained by secureTarget before writes.
	return os.WriteFile(target, body, 0o600)
}

func applyModify(target string, change FileChange) error {
	// #nosec G304 -- target path was validated to remain within the project root.
	body, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("read %s: %w", RedactPath(change.TargetPath()), err)
	}
	lines, trailingNewline := splitPatchLines(body)
	for _, hunk := range change.Hunks {
		oldSeq, newSeq := hunkSequences(hunk)
		index := findSequence(lines, oldSeq)
		if index < 0 {
			return fmt.Errorf("patch context did not match %s", RedactPath(change.TargetPath()))
		}
		next := make([]string, 0, len(lines)-len(oldSeq)+len(newSeq))
		next = append(next, lines[:index]...)
		next = append(next, newSeq...)
		next = append(next, lines[index+len(oldSeq):]...)
		lines = next
	}
	// #nosec G703 -- target path is constrained by secureTarget before writes.
	return os.WriteFile(target, linesToBytes(lines, trailingNewline || len(change.AddedLines) > 0), 0o600)
}

func splitPatchLines(body []byte) ([]string, bool) {
	if len(body) == 0 {
		return nil, false
	}
	trailing := bytes.HasSuffix(body, []byte("\n"))
	text := strings.TrimSuffix(string(body), "\n")
	if text == "" {
		return []string{""}, trailing
	}
	return strings.Split(text, "\n"), trailing
}

func linesToBytes(lines []string, trailing bool) []byte {
	body := strings.Join(lines, "\n")
	if trailing {
		body += "\n"
	}
	return []byte(body)
}

func hunkSequences(hunk Hunk) (oldSeq []string, newSeq []string) {
	for _, line := range hunk.Lines {
		switch line.Kind {
		case ' ':
			oldSeq = append(oldSeq, line.Text)
			newSeq = append(newSeq, line.Text)
		case '-':
			oldSeq = append(oldSeq, line.Text)
		case '+':
			newSeq = append(newSeq, line.Text)
		}
	}
	return oldSeq, newSeq
}

func addedLinesFromHunks(hunks []Hunk, fallback []string) []string {
	var lines []string
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			if line.Kind == '+' || line.Kind == ' ' {
				lines = append(lines, line.Text)
			}
		}
	}
	if len(lines) == 0 {
		return fallback
	}
	return lines
}

func findSequence(lines, seq []string) int {
	if len(seq) == 0 {
		return len(lines)
	}
	for i := 0; i+len(seq) <= len(lines); i++ {
		match := true
		for j := range seq {
			if lines[i+j] != seq[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
