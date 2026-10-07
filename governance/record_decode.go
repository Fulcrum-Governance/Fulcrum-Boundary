package governance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
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
	// RecordRejectUnknownField marks a member name that the decision-record
	// schema does not define, in any object: the record's member set is
	// closed (DecisionRecordV1 at the top level, ExecutionClaim inside
	// execution_claim). A verifier that dropped unknown members before
	// hashing would accept attacker-added content under a valid stored hash,
	// so the record is rejected at ingest instead.
	RecordRejectUnknownField = "unknown-field"
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
	// ErrUnknownObjectMember marks a member name outside the record schema's
	// closed member set, at any object position the schema defines.
	ErrUnknownObjectMember = errors.New("unknown object member")
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
	case errors.Is(err, ErrUnknownObjectMember):
		return RecordRejectUnknownField
	default:
		return RecordRejectParse
	}
}

// DecodeDecisionRecord parses decision-record JSON bytes into a
// DecisionRecordV1 with strict ingest semantics: the input must be a single
// JSON object, every object at any depth must have unique member names, no
// bytes may follow the top-level value, and every member name must belong to
// the schema's closed member set (DecisionRecordV1 at the top level,
// ExecutionClaim inside execution_claim). Go's encoding/json silently keeps
// the last duplicate member and silently drops members that have no struct
// field — either would let one byte stream carry content the verifier's hash
// never covered; this decode rejects both before the record is verified.
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
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		// encoding/json reports a member outside the struct's field set as
		// `json: unknown field "name"`; classify it as the shared
		// unknown-field rejection instead of the generic parse-error class.
		if strings.HasPrefix(err.Error(), "json: unknown field") {
			return record, fmt.Errorf("%w: %v", ErrUnknownObjectMember, err)
		}
		return record, err
	}
	return record, nil
}

// recordMemberSet and executionClaimMemberSet are the closed member-name sets
// the ingest scan enforces, built from the JSON tags of DecisionRecordV1 and
// ExecutionClaim so the check cannot drift from the schema it mirrors.
var (
	recordMemberSet         = jsonMemberSet(reflect.TypeOf(DecisionRecordV1{}))
	executionClaimMemberSet = jsonMemberSet(reflect.TypeOf(ExecutionClaim{}))
)

// jsonMemberSet returns the member names a struct's JSON tags declare: the
// tag name, or the Go field name when the tag carries no name (encoding/json
// falls back to the field name the same way). `json:"-"` fields are skipped.
func jsonMemberSet(t reflect.Type) map[string]struct{} {
	set := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		switch name {
		case "-":
			continue
		case "":
			name = field.Name
		}
		set[name] = struct{}{}
	}
	return set
}

// memberScope names which closed member set, if any, constrains the object
// the scan is inside.
type memberScope int

const (
	// scopeAny marks an object position the record schema does not constrain
	// (a value nested under a known member that is not itself a schema
	// object). Only member-name uniqueness is enforced there.
	scopeAny memberScope = iota
	// scopeRecord marks the top-level object: DecisionRecordV1's member set.
	scopeRecord
	// scopeExecutionClaim marks the object value of the top-level
	// execution_claim member: ExecutionClaim's member set.
	scopeExecutionClaim
)

// memberSetFor returns the closed member set for a scope, or nil when the
// position is unconstrained.
func memberSetFor(scope memberScope) map[string]struct{} {
	switch scope {
	case scopeRecord:
		return recordMemberSet
	case scopeExecutionClaim:
		return executionClaimMemberSet
	default:
		return nil
	}
}

// rejectDuplicateMembersAndTrailing consumes exactly one JSON value from body
// and fails when any object repeats a member name, when a schema-constrained
// object carries a member name outside its closed set, or when non-whitespace
// data follows the value. It performs no allocation of the decoded value: it
// is a pre-flight scan ahead of the real unmarshal.
func rejectDuplicateMembersAndTrailing(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	// firstUnknown defers the unknown-field report until the whole value has
	// scanned clean: a duplicate member or trailing bytes anywhere in the
	// input outrank a member-name problem seen earlier. That ordering mirrors
	// the standalone verifiers, which check uniqueness during the parse and
	// the member set only after it completes.
	var firstUnknown string
	if err := consumeJSONValue(decoder, scopeRecord, &firstUnknown); err != nil {
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
	if firstUnknown != "" {
		return fmt.Errorf("%w %q", ErrUnknownObjectMember, firstUnknown)
	}
	return nil
}

// consumeJSONValue walks one complete JSON value from the decoder's token
// stream, enforcing member-name uniqueness inside every object it enters.
// Uniqueness is case-folded: "action" and "Action" in one object are a
// duplicate, matching the Python, TypeScript, and Rust verifiers' folded
// uniqueness checks.
//
// At schema-constrained object positions (scope other than scopeAny) each
// member name must also appear byte-for-byte in the scope's closed member
// set. The check is exact-case because encoding/json would otherwise bind
// "ACTION" to the Action field case-insensitively, letting one byte stream
// carry a member the standalone verifiers reject as unknown-field. An
// out-of-set name is recorded in firstUnknown rather than returned at once,
// so a duplicate member or trailing data later in the input still classifies
// with its own reason (the other verifiers reach the member-set check only
// after their parse completes).
func consumeJSONValue(decoder *json.Decoder, scope memberScope, firstUnknown *string) error {
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
		members := memberSetFor(scope)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object member name is not a string")
			}
			folded := strings.ToLower(key)
			if _, exists := seen[folded]; exists {
				return fmt.Errorf("%w %q", ErrDuplicateObjectMember, key)
			}
			seen[folded] = struct{}{}
			if members != nil {
				if _, ok := members[key]; !ok && *firstUnknown == "" {
					*firstUnknown = key
				}
			}
			childScope := scopeAny
			if scope == scopeRecord && key == "execution_claim" {
				childScope = scopeExecutionClaim
			}
			if err := consumeJSONValue(decoder, childScope, firstUnknown); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return err
		}
		return nil
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder, scopeAny, firstUnknown); err != nil {
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
