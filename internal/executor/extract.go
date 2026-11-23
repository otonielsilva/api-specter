package executor

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/otonielsilva/api-specter/internal/plan"
)

// extract returns, for each operation that produces a value, the places in its
// response that the plan needs: the key is the id of the producer and the
// value lists each From once, sorted. The executor reads it after a response,
// to know what to put in the Pool.
//
// Only requirements that can be met are counted. An operation that is not
// Enabled never runs, so it asks for nothing, and a Producer whose Match is
// Unresolved is a guess that needs review, so it is not extracted. A producer
// that is not in p.Operations is indexed all the same: the executor never
// reaches it, and extract needs no special case. The result is never nil.
func extract(p *plan.Plan) map[string][]string {
	extractions := make(map[string][]string)
	for _, op := range p.Operations {
		if !op.Enabled {
			continue
		}

		for _, req := range op.Requires {
			for _, p := range req.Producers {
				if p.Match != plan.Unresolved &&
					!slices.Contains(extractions[p.Op], p.From) {
					extractions[p.Op] = append(extractions[p.Op], p.From)
				}
			}
		}
	}

	for _, values := range extractions {
		slices.Sort(values)
	}

	return extractions
}

// lookup returns the value that from points to in the JSON body of a response.
// found is false when there is none, which is different from a field that is
// null: that one returns nil and true, the same as Pool.Get.
//
// v0: only one level is supported, "response.body.<field>", and the body must
// be a JSON object. Anything else (other prefix, nested path, a body that is
// empty, invalid or not an object) is not found. Numbers come back as float64,
// because that is how encoding/json decodes them into an any.
func lookup(body []byte, from string) (any, bool) {
	field, ok := strings.CutPrefix(from, "response.body.")
	if !ok {
		return nil, false
	}

	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, false
	}

	obj, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}

	value, ok := obj[field]
	return value, ok
}
