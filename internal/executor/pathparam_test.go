package executor

import (
	"errors"
	"math"
	"testing"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// The value comes from json.Unmarshal into an any, so a JSON number is always
// a float64. The schema of the path parameter, not the value, decides what the
// text in the URL looks like and whether the value is acceptable at all.
func TestParamString(t *testing.T) {
	schema := func(typ contract.SchemaType) *contract.Schema {
		return &contract.Schema{Type: typ}
	}

	integer := schema(contract.SchemaTypeInteger)
	number := schema(contract.SchemaTypeNumber)
	str := schema(contract.SchemaTypeString)
	boolean := schema(contract.SchemaTypeBoolean)

	// 2^53 is the first integer that float64 cannot tell from its neighbour:
	// 2^53+1 is rounded to 2^53 by Unmarshal. Up to 2^53-1 the value is exact.

	tests := []struct {
		name    string
		value   any
		schema  *contract.Schema
		want    string
		wantErr bool // when true the error must be ErrUnconvertible
	}{
		// integer: the value must be a whole number that float64 holds exactly.
		{name: "integer", value: float64(1), schema: integer, want: "1"},
		{name: "integer zero", value: float64(0), schema: integer, want: "0"},
		{name: "integer negative", value: float64(-3), schema: integer, want: "-3"},
		// fmt.Sprint would write 1e+06.
		{name: "integer that Sprint writes in scientific notation", value: float64(1e6), schema: integer, want: "1000000"},
		{name: "integer negative zero is written as zero", value: math.Copysign(0, -1), schema: integer, want: "0"},
		{name: "integer largest exact", value: float64(maxExact), schema: integer, want: "9007199254740991"},
		{name: "integer smallest exact", value: float64(-maxExact), schema: integer, want: "-9007199254740991"},
		{name: "integer 2^53 may be a rounded neighbour", value: float64(1 << 53), schema: integer, wantErr: true},
		{name: "integer above 2^53", value: float64(1<<53 + 2), schema: integer, wantErr: true},
		{name: "integer below -2^53", value: -float64(1 << 53), schema: integer, wantErr: true},
		{name: "integer with a fraction", value: 1.5, schema: integer, wantErr: true},
		{name: "integer NaN", value: math.NaN(), schema: integer, wantErr: true},
		{name: "integer +Inf", value: math.Inf(1), schema: integer, wantErr: true},
		{name: "integer given a numeric string", value: "42", schema: integer, wantErr: true},
		{name: "integer given a bool", value: true, schema: integer, wantErr: true},

		// number: any finite float64.
		{name: "number whole", value: float64(1), schema: number, want: "1"},
		{name: "number with a fraction", value: 1.5, schema: number, want: "1.5"},
		{name: "number shortest form", value: 0.1, schema: number, want: "0.1"},
		{name: "number never in scientific notation", value: float64(1e6), schema: number, want: "1000000"},
		{name: "number NaN", value: math.NaN(), schema: number, wantErr: true},
		{name: "number -Inf", value: math.Inf(-1), schema: number, wantErr: true},
		{name: "number given a string", value: "1.5", schema: number, wantErr: true},

		// string: the text goes through as it is; escaping belongs to
		// resolvePath, which knows where in the URL the text lands.
		{name: "string", value: "abc", schema: str, want: "abc"},
		{name: "string uuid", value: "123e4567-e89b-12d3-a456-426614174000", schema: str, want: "123e4567-e89b-12d3-a456-426614174000"},
		{name: "string is not escaped here", value: "a b/c?d", schema: str, want: "a b/c?d"},
		{name: "string empty would call another endpoint", value: "", schema: str, wantErr: true},
		// Many APIs declare a numeric id as a string in the path.
		{name: "string given a whole number", value: float64(1), schema: str, want: "1"},
		{name: "string given a number above 2^53", value: float64(1<<53 + 2), schema: str, wantErr: true},
		{name: "string given a bool", value: true, schema: str, wantErr: true},

		// boolean.
		{name: "boolean true", value: true, schema: boolean, want: "true"},
		{name: "boolean false", value: false, schema: boolean, want: "false"},
		{name: "boolean given a string", value: "true", schema: boolean, wantErr: true},
		{name: "boolean given a number", value: float64(1), schema: boolean, wantErr: true},

		// Values that never fit a path segment, whatever the schema says.
		{name: "nil with integer", value: nil, schema: integer, wantErr: true},
		{name: "nil with string", value: nil, schema: str, wantErr: true},
		{name: "object", value: map[string]any{"id": float64(1)}, schema: integer, wantErr: true},
		{name: "array", value: []any{float64(1)}, schema: integer, wantErr: true},

		// A schema that cannot describe a path parameter.
		{name: "nil schema", value: float64(1), schema: nil, wantErr: true},
		{name: "schema without type", value: float64(1), schema: &contract.Schema{}, wantErr: true},
		{name: "object schema", value: map[string]any{}, schema: schema(contract.SchemaTypeObject), wantErr: true},
		{name: "array schema", value: []any{}, schema: schema(contract.SchemaTypeArray), wantErr: true},
		{name: "null schema", value: nil, schema: schema(contract.SchemaTypeNull), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := paramString(tt.value, tt.schema)

			if tt.wantErr {
				if !errors.Is(err, ErrUnconvertible) {
					t.Fatalf("paramString(%v) error = %v, want ErrUnconvertible", tt.value, err)
				}
				if got != "" {
					t.Errorf("paramString(%v) = %q on error, want empty", tt.value, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("paramString(%v) unexpected error: %v", tt.value, err)
			}
			if got != tt.want {
				t.Errorf("paramString(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
