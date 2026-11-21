package executor

import (
	"slices"

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
