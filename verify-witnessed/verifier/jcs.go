package verifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// canonicalJSON implements the RFC 8785 rules needed by the pinned Boundary
// decision-record wire type without importing Boundary or a JCS package.
func canonicalJSON(value any) ([]byte, error) {
	marshaled, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical JSON input: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(marshaled))
	decoder.UseNumber()
	var generic any
	if err := decoder.Decode(&generic); err != nil {
		return nil, fmt.Errorf("decode canonical JSON input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("canonical JSON input contains trailing data")
	}

	var output bytes.Buffer
	if err := appendCanonicalJSON(&output, generic); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendCanonicalJSON(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		if typed {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case string:
		appendCanonicalString(output, typed)
	case json.Number:
		number, err := canonicalNumber(typed)
		if err != nil {
			return err
		}
		output.WriteString(number)
	case []any:
		output.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				output.WriteByte(',')
			}
			if err := appendCanonicalJSON(output, item); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
		output.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				output.WriteByte(',')
			}
			appendCanonicalString(output, key)
			output.WriteByte(':')
			if err := appendCanonicalJSON(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON value %T", value)
	}
	return nil
}

func appendCanonicalString(output *bytes.Buffer, value string) {
	const hexDigits = "0123456789abcdef"
	output.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			if r >= 0 && r <= 0x1f {
				output.WriteString(`\u00`)
				output.WriteByte(hexDigits[byte(r)>>4])
				output.WriteByte(hexDigits[byte(r)&0x0f])
			} else {
				output.WriteRune(r)
			}
		}
	}
	output.WriteByte('"')
}

func canonicalNumber(number json.Number) (string, error) {
	value, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return "", fmt.Errorf("invalid RFC 8785 number %q", number)
	}
	if value == 0 {
		return "0", nil
	}

	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	scientific := strconv.FormatFloat(value, 'e', -1, 64)
	mantissa, exponentText, ok := strings.Cut(scientific, "e")
	if !ok {
		return "", fmt.Errorf("format RFC 8785 number %q", number)
	}
	exponent, err := strconv.Atoi(exponentText)
	if err != nil {
		return "", fmt.Errorf("format RFC 8785 exponent %q: %w", exponentText, err)
	}
	digits := strings.ReplaceAll(mantissa, ".", "")

	if exponent >= -6 && exponent < 21 {
		point := exponent + 1
		switch {
		case point <= 0:
			return sign + "0." + strings.Repeat("0", -point) + digits, nil
		case point >= len(digits):
			return sign + digits + strings.Repeat("0", point-len(digits)), nil
		default:
			return sign + digits[:point] + "." + digits[point:], nil
		}
	}

	result := sign + digits[:1]
	if len(digits) > 1 {
		result += "." + digits[1:]
	}
	if exponent >= 0 {
		result += "e+" + strconv.Itoa(exponent)
	} else {
		result += "e" + strconv.Itoa(exponent)
	}
	return result, nil
}

func utf16Less(left, right string) bool {
	leftUnits := utf16.Encode([]rune(left))
	rightUnits := utf16.Encode([]rune(right))
	limit := len(leftUnits)
	if len(rightUnits) < limit {
		limit = len(rightUnits)
	}
	for i := 0; i < limit; i++ {
		if leftUnits[i] != rightUnits[i] {
			return leftUnits[i] < rightUnits[i]
		}
	}
	return len(leftUnits) < len(rightUnits)
}
