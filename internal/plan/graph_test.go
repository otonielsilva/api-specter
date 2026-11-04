package plan

import (
	"slices"
	"testing"
)

// These tests cover Sort (see docs/design/plan-and-run.md, sections 4 and 8).
// Sort has no code yet, so the bodies are comments: uncomment the call and
// fill in as you implement each step.
//
// Suggested signature:
//
//	func Sort(ops map[string]Operator) ([]string, []Cycle, error)
//
// Edges come from Operator.Requires[i].Producers[j].Op -> the operation
// (every producer is an edge, so OR is treated as AND in v0).
//
// Tip: build graphs with a small helper instead of repeating struct literals:
//
//	func graph(deps map[string][]string) map[string]Operator
//
// where deps["B"] = []string{"A"} means B requires a value produced by A.

// Step 1: ordering a graph with no cycle.

// zooOrder is the order of the zoo plan, the table in docs/design/zoo-graph.md:
// Kahn's algorithm with the (is DELETE, id) tie-break.
// var zooOrder = []string{
//	"GET /animals",
//	"GET /feeds",
//	"GET /healthz",
//	"POST /animals",
//	"GET /animals/{id}",
//	"GET /animals/{id}/feedings",
//	"PATCH /animals/{id}",
//	"POST /feeds",
//	"GET /feeds/{id}",
//	"POST /animals/{id}/feedings",
//	"POST /feeds/{id}/restock",
//	"PUT /animals/{id}",
//	"PUT /feeds/{id}",
//	"DELETE /animals/{id}",
//	"DELETE /feeds/{id}",
// }

func TestSortZooMatchesGoldenOrder(t *testing.T) {
	t.Skip("TODO: enable once Sort is implemented")
	// p := loadZoo(t)
	//
	// // Guard: the golden file and the table must agree, so a change to one
	// // without the other fails here and not as a confusing Sort failure.
	// if !slices.Equal(p.Order, zooOrder) {
	//	t.Fatalf("golden order differs from zooOrder:\n got: %q\nwant: %q", p.Order, zooOrder)
	// }
	//
	// order, cycles, err := Sort(p.Operations)
	// if err != nil {
	//	t.Fatalf("Sort: unexpected error: %v", err)
	// }
	// if len(cycles) != 0 {
	//	t.Errorf("Sort: got cycles %+v, want none", cycles)
	// }
	// if !slices.Equal(order, zooOrder) {
	//	t.Errorf("Sort order:\n got: %q\nwant: %q", order, zooOrder)
	// }
}

func TestSortDeleteComesLast(t *testing.T) {
	t.Skip("TODO: write me")
	// Property test: for every DELETE in the result, no non-DELETE comes after
	// it unless that operation transitively needs the DELETE (it never does in
	// the zoo). Kahn alone does not give this, the (is DELETE, id) key does.
	// Smaller case: DELETE /x/{id} and GET /x/{id} both depend on POST /x;
	// GET must come first even though "DELETE" < "GET" alphabetically.
}

func TestSortIsDeterministic(t *testing.T) {
	t.Skip("TODO: write me")
	// Same graph, many runs (map iteration order is random in Go, so loop
	// like TestParseOpenAPIDeterministicOrder does): the order never changes.
}

func TestSortIsTopological(t *testing.T) {
	t.Skip("TODO: write me")
	// For every edge producer -> consumer, producer appears before consumer
	// in the result. Check it with a map id -> position, on the zoo graph.
}

func TestSortNoOperations(t *testing.T) {
	t.Skip("TODO: write me")
	// nil and empty map: empty (non-nil) order, no cycles, no error.
}

// Step 2: cycle detection.

func TestSortSelfLoopIsIgnored(t *testing.T) {
	t.Skip("TODO: write me")
	// A requires a value produced by A: no error, A is in the order once,
	// and Cycles stays empty (section 4: self-loops are ignored).
}

func TestSortDetectsCycle(t *testing.T) {
	t.Skip("TODO: write me")
	// A <-> B (each requires the other) plus an independent C.
	// Before step 3 exists: expect an error whose message names A and B and
	// not C. Once step 3 is done this test moves to the cycle-breaking table.
}

// Step 3: deterministic cycle breaking.

func TestSortBreaksCycles(t *testing.T) {
	t.Skip("TODO: write me")
	// Table test. For each case: the graph, the expected Order, the expected
	// Cycles (Ops and Dropped). Cases:
	//   1. simple cycle A <-> B: one edge dropped, recorded in Cycles
	//   2. two separate cycles: two entries in Cycles
	//   3. cycle where only one edge has a non-POST producer: that edge is
	//      the one dropped (rule suggested in section 4)
	//   4. tie between two candidate edges: the one with the smaller id wins
	// Note: the rule for which edge to drop is still open in the design doc.
	// Decide it before writing the expected values and write it in a comment.
}

func TestSortCycleBreakingIsDeterministic(t *testing.T) {
	t.Skip("TODO: write me")
	// Same cyclic graph, many runs: same Order and same Cycles every time.
}

// Step 4: Validate (check of an order against the edges).

func TestValidateOrder(t *testing.T) {
	t.Skip("TODO: write me")
	// Suggested: func Validate(p *Plan) error
	// Table of bad plans, each error must name the operation:
	//   1. order is missing an operation that is in Operations
	//   2. order contains an id that is not in Operations
	//   3. a producer comes after its consumer
	//   4. a duplicated id in order
	// And a good one: the zoo plan (testdata/zoo.plan.json) returns nil.
}

// Step 5: wiring into Build.

func TestBuildOrderPutsDeleteLast(t *testing.T) {
	t.Skip("TODO: write me")
	// Build with a contract with GET, POST and DELETE on /pets: Requires is
	// still empty (no heuristic yet), so Order is just (is DELETE, id):
	// GET /pets, POST /pets, DELETE /pets. Replaces the "only the set is
	// checked" comment in TestBuild.
}

// BuildGraph: edges and pending counts.

// wantNode is the expected state of one node. Dependents are compared sorted
// because BuildGraph appends them in map iteration order, which is random.
type wantNode struct {
	dependents []string
	pending    int
}

// graph builds an operations map from deps, where deps["B"] = {"A"} means B
// requires a value produced by A. Each producer goes in its own Requirement.
func graph(deps map[string][]string) map[string]Operator {
	ops := make(map[string]Operator, len(deps))
	for id, producers := range deps {
		var reqs []Requirement
		for _, p := range producers {
			reqs = append(reqs, Requirement{
				Param:     "id",
				In:        Path,
				Producers: []Producer{{Op: p}},
			})
		}
		ops[id] = Operator{Enabled: true, Requires: reqs}
	}
	return ops
}

// checkGraph compares got with want node by node, ignoring dependents order.
func checkGraph(t *testing.T, got map[string]*Node, want map[string]wantNode) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("got %d nodes, want %d", len(got), len(want))
	}
	for id, w := range want {
		n, ok := got[id]
		if !ok {
			t.Errorf("node %q missing", id)
			continue
		}
		deps := slices.Sorted(slices.Values(n.dependents))
		wantDeps := slices.Sorted(slices.Values(w.dependents))
		if !slices.Equal(deps, wantDeps) {
			t.Errorf("node %q dependents:\n got: %q\nwant: %q", id, deps, wantDeps)
		}
		if n.pending != w.pending {
			t.Errorf("node %q pending = %d, want %d", id, n.pending, w.pending)
		}
	}
}

func TestBuildGraphZooMatchesGolden(t *testing.T) {
	p := loadZoo(t)

	// The 11 edges of docs/design/zoo-graph.md. Only the two POSTs on a
	// collection produce values; POST /animals/{id}/feedings waits for both.
	want := map[string]wantNode{
		"POST /animals": {dependents: []string{
			"GET /animals/{id}",
			"PUT /animals/{id}",
			"PATCH /animals/{id}",
			"DELETE /animals/{id}",
			"GET /animals/{id}/feedings",
			"POST /animals/{id}/feedings",
		}},
		"POST /feeds": {dependents: []string{
			"GET /feeds/{id}",
			"PUT /feeds/{id}",
			"DELETE /feeds/{id}",
			"POST /feeds/{id}/restock",
			"POST /animals/{id}/feedings",
		}},
		"GET /healthz":                {},
		"GET /animals":                {},
		"GET /feeds":                  {},
		"GET /animals/{id}":           {pending: 1},
		"PUT /animals/{id}":           {pending: 1},
		"PATCH /animals/{id}":         {pending: 1},
		"DELETE /animals/{id}":        {pending: 1},
		"GET /animals/{id}/feedings":  {pending: 1},
		"POST /animals/{id}/feedings": {pending: 2},
		"GET /feeds/{id}":             {pending: 1},
		"PUT /feeds/{id}":             {pending: 1},
		"DELETE /feeds/{id}":          {pending: 1},
		"POST /feeds/{id}/restock":    {pending: 1},
	}

	checkGraph(t, BuildGraph(p.Operations), want)
}

func TestBuildGraph(t *testing.T) {
	tests := []struct {
		name string
		ops  map[string]Operator
		want map[string]wantNode
	}{
		{
			name: "nil operations",
			ops:  nil,
			want: map[string]wantNode{},
		},
		{
			name: "no dependencies",
			ops:  graph(map[string][]string{"A": nil, "B": nil}),
			want: map[string]wantNode{"A": {}, "B": {}},
		},
		{
			name: "one edge",
			ops:  graph(map[string][]string{"A": nil, "B": {"A"}}),
			want: map[string]wantNode{
				"A": {dependents: []string{"B"}},
				"B": {pending: 1},
			},
		},
		{
			name: "two producers",
			ops:  graph(map[string][]string{"A": nil, "B": nil, "C": {"A", "B"}}),
			want: map[string]wantNode{
				"A": {dependents: []string{"C"}},
				"B": {dependents: []string{"C"}},
				"C": {pending: 2},
			},
		},
		{
			name: "self-loop is ignored",
			ops:  graph(map[string][]string{"A": {"A"}}),
			want: map[string]wantNode{"A": {}},
		},
		{
			// Same producer in two Requirements counts as one edge.
			name: "duplicate producer across requirements",
			ops:  graph(map[string][]string{"A": nil, "B": {"A", "A"}}),
			want: map[string]wantNode{
				"A": {dependents: []string{"B"}},
				"B": {pending: 1},
			},
		},
		{
			// Same producer twice in one Requirement's Producers.
			name: "duplicate producer in one requirement",
			ops: map[string]Operator{
				"A": {},
				"B": {Requires: []Requirement{{
					Param:     "id",
					In:        Path,
					Producers: []Producer{{Op: "A"}, {Op: "A"}},
				}}},
			},
			want: map[string]wantNode{
				"A": {dependents: []string{"B"}},
				"B": {pending: 1},
			},
		},
		{
			// Logged with slog.Warn and skipped: no node is created for X.
			name: "producer not in plan is skipped",
			ops:  graph(map[string][]string{"B": {"X"}}),
			want: map[string]wantNode{"B": {}},
		},
		{
			// BuildGraph does not detect cycles; both nodes wait for each other.
			name: "cycle",
			ops:  graph(map[string][]string{"A": {"B"}, "B": {"A"}}),
			want: map[string]wantNode{
				"A": {dependents: []string{"B"}, pending: 1},
				"B": {dependents: []string{"A"}, pending: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkGraph(t, BuildGraph(tt.ops), tt.want)
		})
	}
}
