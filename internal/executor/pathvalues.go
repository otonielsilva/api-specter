package executor

import (
	"fmt"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/plan"
)

// pathValues returns, for each path parameter of op, the text to put in the
// URL, keyed by the name of the parameter: the map that resolvePath takes. It
// is empty, never nil, when op has no path parameters.
//
// The plan says who can produce a parameter (operator.Requires), the Pool says
// what was captured, and the schema of the parameter decides whether a
// captured value is acceptable (paramString). Producers are tried in the
// order of the plan and the first one with a usable value wins: a producer
// that has not run, or whose value does not fit the schema, is skipped.
//
// When a path parameter ends with no usable value, pathValues returns nil and
// an error that wraps ErrMissingParam and names the parameter, so the caller
// skips the operation and can say why. The value itself is not in the error:
// it comes from a response of the target.
func pathValues(op contract.Operation, operator plan.Operator, pool *Pool) (map[string]string, error) {
	values := make(map[string]string)

	for _, param := range op.Params {
		if param.In != contract.ParamInPath {
			continue
		}

		text, ok := pathValue(param, operator.Requires, pool)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrMissingParam, param.Name)
		}
		values[param.Name] = text
	}

	return values, nil
}

// pathValue returns the text of the first usable value that the plan's
// producers of param have in pool. Only a requirement for this parameter in
// the path counts: the same name may also be asked for in the body.
func pathValue(param contract.Param, requires []plan.Requirement, pool *Pool) (string, bool) {
	for _, req := range requires {
		if req.Param != param.Name || req.In != plan.Path {
			continue
		}

		for _, producer := range req.Producers {
			value, found := pool.Get(Key{Op: producer.Op, From: producer.From})
			if !found {
				continue
			}

			// A value that does not fit the schema (a null, 1.5 for an
			// integer) is not an error of the run: another producer may have
			// a good one.
			if text, err := paramString(value, param.Schema); err == nil {
				return text, true
			}
		}
	}

	return "", false
}
