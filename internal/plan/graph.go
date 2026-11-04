package plan

import "log/slog"

// Node is one operation in the dependency graph built by BuildGraph.
// dependents are the operations that consume a value this one produces;
// pending is how many distinct producers this operation still waits for.
type Node struct {
	dependents []string
	pending    int
}

// Sort returns the order to run the operations in, using Kahn's algorithm
// (see docs/design/plan-and-run.md, sections 4 and 8).
//
// Edges come from Operator.Requires[i].Producers[j].Op -> the operation.
// Ties are broken by the key (is DELETE, id), so the result is deterministic
// and DELETEs come last. Self-loops are ignored. Cycles lists the dependency
// cycles found and the edge dropped to break each one.
func Sort(ops map[string]Operator) ([]string, []Cycle, error) {
	BuildGraph(ops)
	// TODO: implement
	return nil, nil, nil
}

// BuildGraph returns a Node for every operation in ops, with an edge from
// each producer to the operation that requires it. A producer listed more
// than once for the same operation counts as one edge, self-loops are
// ignored, and a producer that is not in ops is logged and skipped. Cycles
// are not detected here. The order of dependents is not deterministic.
func BuildGraph(ops map[string]Operator) map[string]*Node {
	// Dependent map
	nodes := make(map[string]*Node, len(ops))
	for id := range ops {
		nodes[id] = &Node{}
	}

	// build list of dependents
	for id, op := range ops {
		seen := make(map[string]struct{})
		for _, req := range op.Requires {
			for _, prod := range req.Producers {
				if prod.Op == id {
					continue
				}
				if _, dup := seen[prod.Op]; dup {
					continue
				}
				seen[prod.Op] = struct{}{}
				p, ok := nodes[prod.Op]
				if !ok {
					slog.Warn("producer not in plan, edge skipped",
						"op", id,
						"producer", prod.Op)
					continue
				}
				p.dependents = append(p.dependents, id) // aresta prod -> id
				nodes[id].pending++
			}
		}
	}

	return nodes
}
