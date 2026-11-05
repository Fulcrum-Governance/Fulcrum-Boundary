package verifier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Decision-record rejection classes. The strings are the shared vocabulary of
// Boundary's cross-language verifier conformance corpus (the `reject:<class>`
// values in its manifest). This module cannot import Boundary, so the class
// names are restated here and pinned by the corpus test.
const (
	recordRejectDuplicateKey = "duplicate-key"
	recordRejectTrailingData = "trailing-data"
	recordRejectNotObject    = "not-object"
	recordRejectUnknownField = "unknown-field"
	recordRejectParse        = "parse-error"
)

var (
	errDuplicateMember = errors.New("duplicate object member")
	errTrailingData    = errors.New("trailing data after JSON value")
	errNotObject       = errors.New("decision record must be a JSON object")
	errUnknownMember   = errors.New("unknown object member")
)

// recordRejectReason classifies a decodeDecisionRecord failure into one of the
// shared rejection classes. Errors that carry no sentinel classify as
// parse-error, as in Boundary's own decoder.
func recordRejectReason(err error) string {
	switch {
	case errors.Is(err, errDuplicateMember):
		return recordRejectDuplicateKey
	case errors.Is(err, errTrailingData):
		return recordRejectTrailingData
	case errors.Is(err, errNotObject):
		return recordRejectNotObject
	case errors.Is(err, errUnknownMember):
		return recordRejectUnknownField
	default:
		return recordRejectParse
	}
}

// Closed member sets, derived from the JSON tags of the verifier's own record
// types so the scan cannot drift from the struct it guards.
var (
	recordMemberSet         = jsonMemberSet(reflect.TypeOf(decisionRecordV1{}))
	checkMemberSet          = jsonMemberSet(reflect.TypeOf(checkFailure{}))
	executionClaimMemberSet = jsonMemberSet(reflect.TypeOf(executionClaim{}))
)

func jsonMemberSet(t reflect.Type) map[string]struct{} {
	set := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name, _, ignored := jsonField(t.Field(i))
		if ignored {
			continue
		}
		set[name] = struct{}{}
	}
	return set
}

type memberScope int

const (
	scopeAny memberScope = iota
	scopeRecord
	scopeCheck
	scopeExecutionClaim
)

func memberSetFor(scope memberScope) map[string]struct{} {
	switch scope {
	case scopeRecord:
		return recordMemberSet
	case scopeCheck:
		return checkMemberSet
	case scopeExecutionClaim:
		return executionClaimMemberSet
	default:
		return nil
	}
}

func decodeStrictRecordRequired(data []byte, dst *decisionRecordV1, required ...string) error {
	return decodeStrictRequiredWithScan(data, dst, scanDecisionRecord, required...)
}

// scanDecisionRecord is the decision-record pre-flight scan. It mirrors
// Boundary's strict ingest: the input is one JSON object; member-name
// uniqueness is case-folded at every depth; the top-level object, check and
// execution_claim accept only the exact-case members of their closed sets;
// and no bytes follow the value. A duplicate member or trailing data outranks
// an unknown member seen earlier in the input.
func scanDecisionRecord(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var firstUnknown string
	if err := consumeRecordValue(decoder, scopeRecord, &firstUnknown); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errTrailingData
		}
		return fmt.Errorf("%w: %v", errTrailingData, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errNotObject
	}
	if firstUnknown != "" {
		return fmt.Errorf("%w %q", errUnknownMember, firstUnknown)
	}
	return nil
}

func consumeRecordValue(decoder *json.Decoder, scope memberScope, firstUnknown *string) error {
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
				return fmt.Errorf("%w %q", errDuplicateMember, key)
			}
			seen[folded] = struct{}{}
			if members != nil {
				if _, known := members[key]; !known && *firstUnknown == "" {
					*firstUnknown = key
				}
			}
			childScope := scopeAny
			if scope == scopeRecord {
				switch key {
				case "check":
					childScope = scopeCheck
				case "execution_claim":
					childScope = scopeExecutionClaim
				}
			}
			if err := consumeRecordValue(decoder, childScope, firstUnknown); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("object is not terminated")
		}
		return nil
	case '[':
		for decoder.More() {
			if err := consumeRecordValue(decoder, scopeAny, firstUnknown); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("array is not terminated")
		}
		return nil
	default:
		return fmt.Errorf("unexpected delimiter %q", delim)
	}
}
