package plan

import (
	"errors"
	"slices"
	"strings"
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
var zooOrder = []string{
	"GET /animals",
	"GET /feeds",
	"GET /healthz",
	"POST /animals",
	"GET /animals/{id}",
	"GET /animals/{id}/feedings",
	"PATCH /animals/{id}",
	"POST /feeds",
	"GET /feeds/{id}",
	"POST /animals/{id}/feedings",
	"POST /feeds/{id}/restock",
	"PUT /animals/{id}",
	"PUT /feeds/{id}",
	"DELETE /animals/{id}",
	"DELETE /feeds/{id}",
}

func TestSortZooMatchesGoldenOrder(t *testing.T) {
	p := loadZoo(t)

	// Guard: the golden file and the table must agree, so a change to one
	// without the other fails here and not as a confusing Sort failure.
	if !slices.Equal(p.Order, zooOrder) {
		t.Fatalf("golden order differs from zooOrder:\n got: %q\nwant: %q", p.Order, zooOrder)
	}

	order, cycles, err := Sort(p.Operations)
	if err != nil {
		t.Fatalf("Sort: unexpected error: %v", err)
	}
	if len(cycles) != 0 {
		t.Errorf("Sort: got cycles %+v, want none", cycles)
	}
	if !slices.Equal(order, zooOrder) {
		t.Errorf("Sort order:\n got: %q\nwant: %q", order, zooOrder)
	}
}

// Kahn alone does not put DELETEs last; the (is DELETE, id) key does.
func TestSortDeleteComesLast(t *testing.T) {
	// GET must come first even though "DELETE" < "GET" alphabetically.
	t.Run("small", func(t *testing.T) {
		// GET and DELETE both require a value produced by POST.
		ops := graph(map[string][]string{
			"POST /x":        nil,
			"DELETE /x/{id}": {"POST /x"},
			"GET /x/{id}":    {"POST /x"},
		})

		got := mustSort(t, ops)

		getIndex := slices.Index(got, "GET /x/{id}")
		deleteIndex := slices.Index(got, "DELETE /x/{id}")
		if getIndex == -1 || deleteIndex == -1 {
			t.Fatalf("Sort order %q is missing GET /x/{id} or DELETE /x/{id}", got)
		}
		if getIndex >= deleteIndex {
			t.Errorf("GET /x/{id} at %d, DELETE /x/{id} at %d, want GET first; order %q",
				getIndex, deleteIndex, got)
		}
	})

	// From the first DELETE on, everything is a DELETE. This holds because
	// nothing in the zoo requires a value produced by a DELETE.
	t.Run("zoo", func(t *testing.T) {
		got := mustSort(t, loadZoo(t).Operations)

		first := slices.IndexFunc(got, isDelete)
		if first == -1 {
			t.Fatalf("Sort order %q has no DELETE, want at least one", got)
		}
		for i, id := range got[first:] {
			if !isDelete(id) {
				t.Errorf("%q at %d comes after DELETE %q at %d; order %q",
					id, first+i, got[first], first, got)
			}
		}
	})
}

// Map iteration order is random in Go and Sort ranges over maps (the graph
// and each node's dependents), so one run proves little: compare many runs.
func TestSortIsDeterministic(t *testing.T) {
	ops := loadZoo(t).Operations
	want := mustSort(t, ops)

	for i := range 200 {
		got := mustSort(t, ops)
		if !slices.Equal(got, want) {
			t.Fatalf("run %d: Sort order changed:\n got: %q\nwant: %q", i, got, want)
		}
	}
}

// For every edge producer -> consumer, the producer comes before the consumer.
func TestSortIsTopological(t *testing.T) {
	ops := loadZoo(t).Operations
	got := mustSort(t, ops)

	if len(got) != len(ops) {
		t.Fatalf("Sort returned %d operations, want %d; order %q", len(got), len(ops), got)
	}

	pos := make(map[string]int, len(got))
	for i, id := range got {
		pos[id] = i
	}

	edges := 0
	for consumer, op := range ops {
		for _, req := range op.Requires {
			for _, prod := range req.Producers {
				if prod.Op == consumer {
					continue // self-loops are ignored by Sort
				}
				edges++
				if pos[prod.Op] >= pos[consumer] {
					t.Errorf("producer %q at %d, consumer %q at %d, want producer first",
						prod.Op, pos[prod.Op], consumer, pos[consumer])
				}
			}
		}
	}
	// Guard: without edges the check above passes for any order.
	if edges == 0 {
		t.Fatal("zoo plan has no edges, the test checks nothing")
	}
}

func TestSortNoOperations(t *testing.T) {
	tests := []struct {
		name string
		ops  map[string]Operator
	}{
		{name: "nil", ops: nil},
		{name: "empty", ops: map[string]Operator{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustSort(t, tt.ops)
			// Non-nil so the plan's "order" is [] in the JSON, not null.
			if got == nil || len(got) != 0 {
				t.Errorf("Sort order = %#v, want empty non-nil slice", got)
			}
		})
	}
}

// Step 2: cycle detection.

func TestSortSelfLoopIsIgnored(t *testing.T) {
	got := mustSort(t, graph(map[string][]string{
		"POST /x": {"POST /x"},
	}))
	if want := []string{"POST /x"}; !slices.Equal(got, want) {
		t.Errorf("Sort order = %q, want %q", got, want)
	}
}

// Before step 3 exists Sort fails on a cycle. Once step 3 is done this test
// moves to the cycle-breaking table.
func TestSortDetectsCycle(t *testing.T) {
	// A <-> B (each requires the other) plus an independent C.
	ops := graph(map[string][]string{
		"A": {"B"},
		"B": {"A"},
		"C": nil,
	})

	got, cycles, err := Sort(ops)
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("Sort error = %v, want %v", err, ErrCycle)
	}
	if got != nil {
		t.Errorf("Sort order = %q, want nil on error", got)
	}
	if cycles != nil {
		t.Errorf("Sort cycles = %+v, want nil on error", cycles)
	}

	// Ids are quoted in the message, so `"C"` cannot match a letter of
	// another word.
	msg := err.Error()
	for _, id := range []string{`"A"`, `"B"`} {
		if !strings.Contains(msg, id) {
			t.Errorf("error %q does not name %s", msg, id)
		}
	}
	if strings.Contains(msg, `"C"`) {
		t.Errorf("error %q names \"C\", which is not in the cycle", msg)
	}
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

// Step 5: wiring into Build. See TestBuildOrderPutsDeleteLast in build_test.go.

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

// isDelete reports whether the operation id is a DELETE.
func isDelete(id string) bool {
	return strings.HasPrefix(id, "DELETE ")
}

// mustSort calls Sort and stops the test on an error or on any cycle.
func mustSort(t *testing.T, ops map[string]Operator) []string {
	t.Helper()
	got, cycles, err := Sort(ops)
	if err != nil {
		t.Fatalf("Sort: unexpected error: %v", err)
	}
	if len(cycles) != 0 {
		t.Fatalf("Sort: got cycles %+v, want none", cycles)
	}
	return got
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
