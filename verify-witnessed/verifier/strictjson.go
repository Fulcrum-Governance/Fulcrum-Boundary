package verifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"
)

func decodeStrict(data []byte, dst any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON is not valid UTF-8")
	}
	if err := rejectLoneUnicodeSurrogates(data); err != nil {
		return err
	}
	if err := rejectDuplicateMembersAndTrailingValues(data); err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode JSON: trailing value")
		}
		return fmt.Errorf("decode JSON trailing data: %w", err)
	}
	return nil
}

func rejectLoneUnicodeSurrogates(data []byte) error {
	inString := false
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || i+1 >= len(data) {
				continue
			}
			if data[i+1] != 'u' {
				i++
				continue
			}
			unit, ok := decodeHexCodeUnit(data, i+2)
			if !ok {
				continue
			}
			i += 5
			switch {
			case unit >= 0xd800 && unit <= 0xdbff:
				if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
					return fmt.Errorf("JSON string contains an unpaired high surrogate")
				}
				low, lowOK := decodeHexCodeUnit(data, i+3)
				if !lowOK || low < 0xdc00 || low > 0xdfff {
					return fmt.Errorf("JSON string contains an unpaired high surrogate")
				}
				i += 6
			case unit >= 0xdc00 && unit <= 0xdfff:
				return fmt.Errorf("JSON string contains an unpaired low surrogate")
			}
		}
	}
	return nil
}

func decodeHexCodeUnit(data []byte, start int) (uint16, bool) {
	if start+4 > len(data) {
		return 0, false
	}
	var value uint16
	for _, character := range data[start : start+4] {
		value <<= 4
		switch {
		case character >= '0' && character <= '9':
			value |= uint16(character - '0')
		case character >= 'a' && character <= 'f':
			value |= uint16(character-'a') + 10
		case character >= 'A' && character <= 'F':
			value |= uint16(character-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func decodeStrictRequired(data []byte, dst any, required ...string) error {
	if err := decodeStrict(data, dst); err != nil {
		return err
	}
	if err := validateRequiredJSONShape(data, dst); err != nil {
		return err
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return fmt.Errorf("decode required JSON members: %w", err)
	}
	if members == nil {
		return fmt.Errorf("JSON value must be an object")
	}
	for _, name := range required {
		if _, ok := members[name]; !ok {
			return fmt.Errorf("required member %q is missing", name)
		}
	}
	return nil
}

func validateRequiredJSONShape(data []byte, dst any) error {
	typeOf := reflect.TypeOf(dst)
	if typeOf == nil || typeOf.Kind() != reflect.Pointer || typeOf.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("strict JSON destination must point to a struct")
	}
	return validateRequiredValue(json.RawMessage(data), typeOf.Elem(), "$", true)
}

func validateRequiredValue(raw json.RawMessage, valueType reflect.Type, path string, required bool) error {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if required {
			return fmt.Errorf("required JSON value %s must not be null", path)
		}
		return nil
	}

	switch valueType.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return fmt.Errorf("required JSON value %s must be an object", path)
		}
		for i := 0; i < valueType.NumField(); i++ {
			field := valueType.Field(i)
			if !field.IsExported() {
				continue
			}
			name, optional, ignored := jsonField(field)
			if ignored {
				continue
			}
			member, present := object[name]
			if !present {
				if !optional {
					return fmt.Errorf("required member %q is missing at %s", name, path)
				}
				continue
			}
			if err := validateRequiredValue(member, field.Type, path+"."+name, true); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil || items == nil {
			return fmt.Errorf("required JSON value %s must be an array", path)
		}
		for i, item := range items {
			if err := validateRequiredValue(item, valueType.Elem(), fmt.Sprintf("%s[%d]", path, i), true); err != nil {
				return err
			}
		}
	case reflect.Map:
		var members map[string]json.RawMessage
		if err := json.Unmarshal(raw, &members); err != nil || members == nil {
			return fmt.Errorf("required JSON value %s must be an object", path)
		}
		for name, member := range members {
			if err := validateRequiredValue(member, valueType.Elem(), path+"."+name, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func jsonField(field reflect.StructField) (name string, optional, ignored bool) {
	tag := field.Tag.Get("json")
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return "", false, true
	}
	name = parts[0]
	if name == "" {
		name = field.Name
	}
	for _, option := range parts[1:] {
		if option == "omitempty" || option == "omitzero" {
			optional = true
		}
	}
	return name, optional, false
}

func rejectDuplicateMembersAndTrailingValues(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid JSON: trailing value")
		}
		return fmt.Errorf("invalid JSON trailing data: %w", err)
	}
	return nil
}

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
				return fmt.Errorf("duplicate object member %q", key)
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
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
			if err := consumeJSONValue(decoder); err != nil {
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

func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return os.ReadFile(path)
}

func decodeCompactJSONLine(data []byte, dst any, required ...string) error {
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return fmt.Errorf("compact JSON sidecar must end with exactly one LF")
	}
	if bytes.Count(data, []byte{'\n'}) != 1 {
		return fmt.Errorf("compact JSON sidecar must contain exactly one LF")
	}
	body := data[:len(data)-1]
	if len(body) < 2 || body[0] != '{' || body[len(body)-1] != '}' {
		return fmt.Errorf("compact JSON sidecar must contain one object")
	}
	if hasJSONWhitespaceOutsideStrings(body) {
		return fmt.Errorf("compact JSON sidecar contains insignificant whitespace")
	}
	return decodeStrictRequired(body, dst, required...)
}

func hasJSONWhitespaceOutsideStrings(data []byte) bool {
	inString := false
	escaped := false
	for _, b := range data {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			continue
		}
		switch b {
		case ' ', '\t', '\r', '\n':
			return true
		}
	}
	return false
}

type exactJSONLine struct {
	complete []byte
	body     []byte
}

func splitExactJSONLines(data []byte) ([]exactJSONLine, error) {
	if len(data) == 0 {
		return []exactJSONLine{}, nil
	}
	if data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("JSONL file does not end with LF")
	}
	parts := bytes.SplitAfter(data, []byte{'\n'})
	parts = parts[:len(parts)-1]
	lines := make([]exactJSONLine, 0, len(parts))
	for i, complete := range parts {
		if len(complete) == 1 {
			return nil, fmt.Errorf("JSONL line %d is empty", i+1)
		}
		if complete[len(complete)-2] == '\r' {
			return nil, fmt.Errorf("JSONL line %d uses CRLF", i+1)
		}
		lines = append(lines, exactJSONLine{
			complete: complete,
			body:     complete[:len(complete)-1],
		})
	}
	return lines, nil
}
