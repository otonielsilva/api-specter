package generator

import (
	"errors"
	"fmt"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// Minimal returns the smallest value that s accepts, ready for json.Marshal.
// An object only gets its required properties. It returns an error for a
// schema it cannot satisfy, so the caller never sends a made-up body.
func Minimal(s *contract.Schema) (any, error) {
	if s == nil {
		return nil, errors.New("generator: nil schema")
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
		return "", nil
	case contract.SchemaTypeNumber:
		return float64(0), nil
	case contract.SchemaTypeBoolean:
		return false, nil
	case contract.SchemaTypeInteger:
		return int64(0), nil
	default:
		return nil, fmt.Errorf("generator: unsupported schema type %q", s.Type)
	}
}
