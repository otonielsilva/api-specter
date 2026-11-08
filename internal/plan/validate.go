package plan

import (
	"fmt"
	"maps"
	"slices"
)

// Validate checks that the order of p can be run: it lists every operation
// exactly once, names no operation that is not in Operations, and puts every
// producer before the operations that require it. A plan can be edited by
// hand, so "run" calls this instead of trusting the order.
//
// A producer that is not in Operations is an error too. Self-loops are
// ignored, like in Sort, and so are the edges dropped to break a cycle
// (Cycles[i].Dropped), since in those the producer runs after the consumer
// on purpose. Errors wrap ErrInvalid and name the operation. Operations are
// visited in sorted order, so the same bad plan always gives the same error.
func Validate(p *Plan) error {
	pos := make(map[string]int, len(p.Order))
	for i, id := range p.Order {
		if _, ok := p.Operations[id]; !ok {
			return fmt.Errorf("plan: %w: order lists %q, which is not in operations", ErrInvalid, id)
		}
		if j, dup := pos[id]; dup {
			return fmt.Errorf("plan: %w: order lists %q twice, at %d and %d", ErrInvalid, id, j, i)
		}
		pos[id] = i
	}

	ids := slices.Sorted(maps.Keys(p.Operations))
	for _, id := range ids {
		if _, ok := pos[id]; !ok {
			return fmt.Errorf("plan: %w: operation %q is missing from order", ErrInvalid, id)
		}
	}

	dropped := make(map[[2]string]bool, len(p.Cycles))
	for _, c := range p.Cycles {
		dropped[[2]string{c.Dropped.From, c.Dropped.To}] = true
	}

	for _, id := range ids {
		for _, req := range p.Operations[id].Requires {
			for _, prod := range req.Producers {
				if prod.Op == id || dropped[[2]string{prod.Op, id}] {
					continue
				}
				at, ok := pos[prod.Op]
				if !ok {
					return fmt.Errorf("plan: %w: operation %q requires %q from %q, which is not in operations",
						ErrInvalid, id, req.Param, prod.Op)
				}
				if at > pos[id] {
					return fmt.Errorf("plan: %w: operation %q at %d runs before its producer %q at %d",
						ErrInvalid, id, pos[id], prod.Op, at)
				}
			}
		}
	}
	return nil
}
