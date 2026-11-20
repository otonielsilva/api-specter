package generator

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// Minimal returns the smallest value that s accepts, ready for json.Marshal.
// An object only gets its required properties. It returns an error for a
// schema it cannot satisfy, so the caller never sends a made-up body.
func Minimal(s *contract.Schema) (any, error) {
	if s == nil {
		return nil, errors.New("generator: nil schema")
	}

	// An enum lists the only values allowed, whatever the type.
	if len(s.Enum) > 0 {
		return s.Enum[0], nil
	}

	switch s.Type {
	case contract.SchemaTypeObject:
		obj := make(map[string]any, len(s.Required))
		for _, name := range s.Required {
			prop, ok := s.Properties[name]
			if !ok {
				return nil, fmt.Errorf("generator: required property %q is not in properties", name)
			}
			value, err := Minimal(prop)
			if err != nil {
				return nil, fmt.Errorf("property %q: %w", name, err)
			}
			obj[name] = value
		}
		return obj, nil
	case contract.SchemaTypeArray:
		return []any{}, nil
	case contract.SchemaTypeString:
		return minimalString(s)
	case contract.SchemaTypeNumber:
		return minimalNumber(s)
	case contract.SchemaTypeBoolean:
		return false, nil
	case contract.SchemaTypeInteger:
		return minimalInteger(s)
	default:
		return nil, fmt.Errorf("generator: unsupported schema type %q", s.Type)
	}
}

// minimalInteger returns the integer closest to zero that is not below the
// minimum, or the largest one allowed when the maximum is below zero.
func minimalInteger(s *contract.Schema) (int64, error) {
	if s.Min != nil && s.Max != nil && *s.Min > *s.Max {
		return 0, fmt.Errorf("generator: minimum %v is above maximum %v", *s.Min, *s.Max)
	}

	switch {
	case s.Min != nil:
		value := math.Ceil(*s.Min)
		if s.Max != nil && value > math.Floor(*s.Max) {
			return 0, fmt.Errorf("generator: no integer between %v and %v", *s.Min, *s.Max)
		}
		return int64(value), nil
	case s.Max != nil && *s.Max < 0:
		return int64(math.Floor(*s.Max)), nil
	default:
		return 0, nil
	}
}

// minimalNumber is minimalInteger without the rounding.
func minimalNumber(s *contract.Schema) (float64, error) {
	if s.Min != nil && s.Max != nil && *s.Min > *s.Max {
		return 0, fmt.Errorf("generator: minimum %v is above maximum %v", *s.Min, *s.Max)
	}

	switch {
	case s.Min != nil:
		return *s.Min, nil
	case s.Max != nil && *s.Max < 0:
		return *s.Max, nil
	default:
		return 0, nil
	}
}

// maxGeneratedLength caps the length of a generated string, so a contract with
// a huge minLength cannot make the run allocate gigabytes.
const maxGeneratedLength = 1 << 20

// minimalString returns the shortest string that has at least MinLength
// characters. It uses "a" and not a space, because an API may trim its input.
func minimalString(s *contract.Schema) (string, error) {
	if s.MinLength == nil {
		return "", nil
	}
	if s.MaxLength != nil && *s.MinLength > *s.MaxLength {
		return "", fmt.Errorf("generator: minLength %d is above maxLength %d", *s.MinLength, *s.MaxLength)
	}
	if *s.MinLength > maxGeneratedLength {
		return "", fmt.Errorf("generator: minLength %d is above the limit of %d", *s.MinLength, maxGeneratedLength)
	}
	return strings.Repeat("a", int(*s.MinLength)), nil
}
