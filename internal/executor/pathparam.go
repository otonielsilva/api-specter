package executor

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// ErrUnconvertible means a value cannot be written as the text of a path
// parameter of the given schema.
var ErrUnconvertible = errors.New("value does not fit the path parameter")

// maxExact is the largest integer that a float64 holds without ambiguity:
// 2^53 is also what encoding/json produces for 2^53+1, so from there on the
// value read from a response may not be the id the API sent.
const maxExact = 1<<53 - 1

// paramString returns the text of value to put in the URL for a path
// parameter described by schema. The schema decides what is acceptable; the
// value is never guessed at.
//
// value comes from json.Unmarshal into an any: a JSON number is a float64. An
// integer schema accepts a float64 that is a whole number up to maxExact, a
// number schema accepts any finite float64, a string schema accepts a
// non-empty string or a whole float64 (many APIs declare a numeric id as a
// string) and a boolean schema accepts a bool. Anything else, including nil,
// objects, arrays and a nil or untyped schema, returns ErrUnconvertible and
// an empty string, so the caller skips the operation instead of sending a
// request that would only prove a bug of specter.
//
// The text is not escaped: that belongs to whoever builds the URL.
func paramString(value any, schema *contract.Schema) (string, error) {
	if schema == nil {
		return "", fmt.Errorf("%w: no schema", ErrUnconvertible)
	}

	switch v := value.(type) {
	case float64:
		if s, ok := numberString(v, schema.Type); ok {
			return s, nil
		}
	case string:
		// An empty value would make a path like /animals/ and call another
		// endpoint.
		if schema.Type == contract.SchemaTypeString && v != "" {
			return v, nil
		}
	case bool:
		if schema.Type == contract.SchemaTypeBoolean {
			return strconv.FormatBool(v), nil
		}
	}

	return "", fmt.Errorf("%w: %v as %s", ErrUnconvertible, value, schema.Type)
}

// numberString writes v as text for a schema of type typ. ok is false when
// the type does not take a number or v does not fit it.
func numberString(v float64, typ contract.SchemaType) (text string, ok bool) {
	switch typ {
	case contract.SchemaTypeInteger, contract.SchemaTypeString:
		if !isExactInteger(v) {
			return "", false
		}
		// int64(-0.0) is 0, so negative zero is written as "0".
		return strconv.FormatInt(int64(v), 10), true
	case contract.SchemaTypeNumber:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", false
		}
		if v == 0 {
			return "0", true
		}
		// 'f' with -1 is the shortest text that reads back as v, and never
		// uses scientific notation (fmt.Sprint writes 1e+06).
		return strconv.FormatFloat(v, 'f', -1, 64), true
	default:
		return "", false
	}
}

// isExactInteger reports whether v is a whole number that a float64 holds
// without ambiguity. NaN and the infinities are not.
func isExactInteger(v float64) bool {
	// NaN must be tested first: v != math.Trunc(v) is also true for it, but
	// for the wrong reason.
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return false
	}
	return v == math.Trunc(v) && math.Abs(v) <= maxExact
}
