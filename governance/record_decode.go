package governance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Machine-readable decision-record rejection classes. These strings are the
// shared vocabulary the cross-language verifier parity harness
// (scripts/ci/run-verifier-parity.sh, docs/VERIFIER_PARITY.md) compares across
// the Go, Python, TypeScript, and Rust verifiers: on a rejected record each
// verifier emits reason=<code> naming one of these classes.
const (
	// RecordRejectDuplicateKey marks an object whose member name repeats at
	// any nesting depth. Stock JSON parsers silently keep the last value,
	// which would let a forged record carry two verdicts in one byte stream;
	// the record must be rejected instead.
	RecordRejectDuplicateKey = "duplicate-key"
	// RecordRejectTrailingData marks non-whitespace bytes after the top-level
	// JSON value.
	RecordRejectTrailingData = "trailing-data"
	// RecordRejectNotJSONObject marks input whose top-level value is not a
	// JSON object.
	RecordRejectNotJSONObject = "not-object"
	// RecordRejectParse marks any other malformed JSON input.
	RecordRejectParse = "parse-error"
	// RecordRejectRead marks a record file that could not be read at all.
	RecordRejectRead = "read-error"
	// RecordRejectMissingHash marks a record whose decision_hash is absent or
	// empty: there is nothing to recompute against.
	RecordRejectMissingHash = "missing-hash"
	// RecordRejectHashMismatch marks a record whose recomputed decision_hash
	// (or another compared digest) differs from the stored value.
	RecordRejectHashMismatch = "hash-mismatch"
	// RecordRejectSchema marks an unsupported schema_version value.
	RecordRejectSchema = "schema-version"
	// RecordRejectSignature marks a signature check failure under
	// --verify-signature.
	RecordRejectSignature = "signature"
	// RecordRejectVerifyFail is the fallback verification-failure class.
	RecordRejectVerifyFail = "verify-fail"
)

// Sentinel errors wrapped by DecodeDecisionRecord failures so callers can
// classify a rejection with RecordRejectReason instead of matching text.
var (
	// ErrDuplicateObjectMember marks a repeated member name in a JSON object.
	ErrDuplicateObjectMember = errors.New("duplicate object member")
	// ErrTrailingJSONData marks bytes following the top-level JSON value.
	ErrTrailingJSONData = errors.New("trailing data after JSON value")
	// ErrRecordNotJSONObject marks a top-level value that is not a JSON object.
	ErrRecordNotJSONObject = errors.New("decision record must be a JSON object")
)

// RecordRejectReason maps a DecodeDecisionRecord error to its machine-readable
// rejection class (the RecordReject* constants). Errors that are not one of the
// sentinel-typed rejections classify as parse-error.
func RecordRejectReason(err error) string {
	switch {
	case errors.Is(err, ErrDuplicateObjectMember):
		return RecordRejectDuplicateKey
	case errors.Is(err, ErrTrailingJSONData):
		return RecordRejectTrailingData
	case errors.Is(err, ErrRecordNotJSONObject):
		return RecordRejectNotJSONObject
	default:
		return RecordRejectParse
	}
}

// DecodeDecisionRecord parses decision-record JSON bytes into a
// DecisionRecordV1 with strict ingest semantics: the input must be a single
// JSON object, every object at any depth must have unique member names, and no
// bytes may follow the top-level value. Go's encoding/json silently keeps the
// last duplicate member, which would let one byte stream carry two different
// verdicts; this decode rejects that ambiguity before the record is verified.
// Classification of a failure is available via RecordRejectReason.
func DecodeDecisionRecord(body []byte) (DecisionRecordV1, error) {
	var record DecisionRecordV1
	if err := rejectDuplicateMembersAndTrailing(body); err != nil {
		return record, err
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return record, ErrRecordNotJSONObject
	}
	if err := json.Unmarshal(body, &record); err != nil {
		return record, err
	}
	return record, nil
}

// rejectDuplicateMembersAndTrailing consumes exactly one JSON value from body
// and fails when any object repeats a member name or when non-whitespace data
// follows the value. It performs no allocation of the decoded value: it is a
// pre-flight scan ahead of the real unmarshal.
func rejectDuplicateMembersAndTrailing(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder); err != nil {
		return fmt.Errorf("%w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrTrailingJSONData
		}
		// The first value consumed cleanly, so any non-EOF failure here means
		// non-whitespace bytes follow it — a second value or garbage. Both are
		// the same shared class: trailing data after the JSON value.
		return fmt.Errorf("%w: %v", ErrTrailingJSONData, err)
	}
	return nil
}

// consumeJSONValue walks one complete JSON value from the decoder's token
// stream, enforcing member-name uniqueness inside every object it enters.
func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object member name is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w %q", ErrDuplicateObjectMember, key)
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return err
		}
		return nil
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unexpected delimiter %q", delim)
	}
}
