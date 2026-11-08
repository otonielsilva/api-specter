package plan

import (
	"cmp"
	"fmt"
	"log/slog"
	"slices"
	"strings"
)

// Node is one operation in the dependency graph built by BuildGraph.
// dependents are the operations that consume a value this one produces;
// producers are the edges coming in, one per distinct producer, with the
// parameter that edge supplies; pending is how many of those producers this
// operation still waits for.
type Node struct {
	dependents []string
	producers  []inEdge
	pending    int
}

// inEdge is an edge seen from the consumer: the producer and the parameter
// it supplies (the first one, when a producer supplies several).
type inEdge struct {
	from  string
	param string
}

// Sort returns the order to run the operations in, using Kahn's algorithm
// (see docs/design/plan-and-run.md, sections 4 and 8).
//
// Edges come from Operator.Requires[i].Producers[j].Op -> the operation.
// Ties are broken by the key (is DELETE, id), so the result is deterministic
// and DELETEs come last. Self-loops are ignored.
//
// A cycle does not stop Sort. When no operation is ready and some are still
// waiting, Sort finds the cycles among them (strongly connected components
// of more than one operation), drops one edge in each and goes on. The edge
// dropped is the smallest by the key (producer is a POST, from, to): an edge
// whose producer is not a POST goes first, because a POST is the usual way to
// create the value, and the ids break the tie. Each drop is returned in
// Cycles, with the operations of the cycle sorted, in the order the drops
// were made.
func Sort(ops map[string]Operator) ([]string, []Cycle, error) {
	graph := BuildGraph(ops)

	ready := []string{}
	for id, node := range graph {
		if node.pending == 0 {
			ready = append(ready, id)
		}
	}

	order := make([]string, 0, len(graph))
	var cycles []Cycle
	for {
		for len(ready) > 0 {
			// Pick the smallest ready operation by (is DELETE, id). Sorting
			// the whole slice each time is O(n log n) per step, fine for API
			// sizes.
			slices.SortFunc(ready, compareRunOrder)
			id := ready[0]
			ready = ready[1:]
			order = append(order, id)

			for _, dep := range graph[id].dependents {
				graph[dep].pending--
				if graph[dep].pending == 0 {
					ready = append(ready, dep)
				}
			}
		}
		if len(order) == len(graph) {
			return order, cycles, nil
		}

		comps := stuckCycles(graph)
		if len(comps) == 0 {
			// Operations wait but none is in a cycle: a bug in Sort or
			// BuildGraph, since every wait leads back to a cycle.
			return nil, nil, fmt.Errorf("%w: operations wait but form no cycle", ErrCycle)
		}
		for _, comp := range comps {
			e := edgeToDrop(graph, comp)
			dropEdge(graph, e)
			cycles = append(cycles, Cycle{Ops: comp, Dropped: e})
			if graph[e.To].pending == 0 {
				ready = append(ready, e.To)
			}
		}
	}
}

// compareRunOrder orders operation ids by the key (is DELETE, id): every
// non-DELETE comes before every DELETE, and ties are broken by the id.
func compareRunOrder(a, b string) int {
	aDel := strings.HasPrefix(a, "DELETE ")
	bDel := strings.HasPrefix(b, "DELETE ")
	if aDel != bDel {
		if aDel {
			return 1
		}
		return -1
	}
	return strings.Compare(a, b)
}

// stuckCycles returns the cycles among the operations still waiting
// (pending > 0): the strongly connected components with more than one
// operation, found with Tarjan's algorithm. An operation that only waits for
// a cycle is a component of its own and is left out. Each component is
// sorted and the components are sorted by their first id, so the result does
// not depend on map order.
func stuckCycles(graph map[string]*Node) [][]string {
	var stuck []string
	for id, node := range graph {
		if node.pending > 0 {
			stuck = append(stuck, id)
		}
	}
	slices.Sort(stuck)

	isStuck := make(map[string]bool, len(stuck))
	for _, id := range stuck {
		isStuck[id] = true
	}

	// Tarjan state: index is the visit number, low the smallest index
	// reachable, and stack holds the operations of the components not yet
	// closed.
	index := make(map[string]int, len(stuck))
	low := make(map[string]int, len(stuck))
	onStack := make(map[string]bool, len(stuck))
	var stack []string
	var comps [][]string

	var visit func(id string)
	visit = func(id string) {
		index[id] = len(index)
		low[id] = index[id]
		stack = append(stack, id)
		onStack[id] = true

		for _, in := range graph[id].producers {
			next := in.from
			if !isStuck[next] {
				continue
			}
			if _, seen := index[next]; !seen {
				visit(next)
				low[id] = min(low[id], low[next])
			} else if onStack[next] {
				low[id] = min(low[id], index[next])
			}
		}

		if low[id] != index[id] {
			return
		}
		// id is the root of a component: pop it off the stack.
		var comp []string
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			comp = append(comp, top)
			if top == id {
				break
			}
		}
		if len(comp) > 1 {
			slices.Sort(comp)
			comps = append(comps, comp)
		}
	}

	for _, id := range stuck {
		if _, seen := index[id]; !seen {
			visit(id)
		}
	}

	slices.SortFunc(comps, func(a, b []string) int { return strings.Compare(a[0], b[0]) })
	return comps
}

// edgeToDrop returns the edge inside comp to drop, the smallest by the key
// (producer is a POST, from, to). comp has more than one operation, so it
// has at least one edge.
func edgeToDrop(graph map[string]*Node, comp []string) Edge {
	in := make(map[string]bool, len(comp))
	for _, id := range comp {
		in[id] = true
	}

	var best Edge
	found := false
	for _, to := range comp {
		for _, e := range graph[to].producers {
			if !in[e.from] {
				continue
			}
			cand := Edge{From: e.from, To: to, Param: e.param}
			if !found || compareDrop(cand, best) < 0 {
				best, found = cand, true
			}
		}
	}
	return best
}

// compareDrop orders edges by (producer is a POST, from, to), so the edge
// whose producer is not a POST comes first.
func compareDrop(a, b Edge) int {
	aPost := strings.HasPrefix(a.From, "POST ")
	bPost := strings.HasPrefix(b.From, "POST ")
	if aPost != bPost {
		if aPost {
			return 1
		}
		return -1
	}
	return cmp.Or(strings.Compare(a.From, b.From), strings.Compare(a.To, b.To))
}

// dropEdge removes e from both ends: e.From no longer counts e.To as a
// dependent, so running e.From later does not decrement e.To a second time,
// and e.To no longer lists e.From, so the next search for cycles does not
// find the same edge again.
func dropEdge(graph map[string]*Node, e Edge) {
	from, to := graph[e.From], graph[e.To]
	if i := slices.Index(from.dependents, e.To); i >= 0 {
		from.dependents = slices.Delete(from.dependents, i, i+1)
	}
	to.producers = slices.DeleteFunc(to.producers, func(in inEdge) bool { return in.from == e.From })
	to.pending--
}

// BuildGraph returns a Node for every operation in ops, with an edge from
// each producer to the operation that requires it. A producer listed more
// than once for the same operation counts as one edge, self-loops are
// ignored, and a producer that is not in ops is logged and skipped. Cycles
// are not detected here. The order of dependents is not deterministic.
func BuildGraph(ops map[string]Operator) map[string]*Node {
	nodes := make(map[string]*Node, len(ops))
	for id := range ops {
		nodes[id] = &Node{}
	}

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
				p.dependents = append(p.dependents, id) // edge prod -> id
				nodes[id].producers = append(nodes[id].producers, inEdge{from: prod.Op, param: req.Param})
				nodes[id].pending++
			}
		}
	}

	return nodes
}
