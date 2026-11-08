package plan

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// These tests cover Sort (see docs/design/plan-and-run.md, sections 4 and 8).
// Edges come from Operator.Requires[i].Producers[j].Op -> the operation
// (every producer is an edge, so OR is treated as AND in v0). Graphs are
// built with graph(deps), where deps["B"] = []string{"A"} means B requires a
// value produced by A.

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

// Step 2: self-loops.

func TestSortSelfLoopIsIgnored(t *testing.T) {
	got := mustSort(t, graph(map[string][]string{
		"POST /x": {"POST /x"},
	}))
	if want := []string{"POST /x"}; !slices.Equal(got, want) {
		t.Errorf("Sort order = %q, want %q", got, want)
	}
}

// Step 3: deterministic cycle breaking.

// drop is the Edge Sort records for a dropped edge built by graph, whose
// Requirements all have the param "id".
func drop(from, to string) Edge {
	return Edge{From: from, To: to, Param: "id"}
}

// The edge dropped is the smallest by (producer is a POST, from, to): an edge
// whose producer is not a POST first, then the smaller ids. In graph(...),
// deps["B"] = {"A"} is the edge A -> B.
func TestSortBreaksCycles(t *testing.T) {
	tests := []struct {
		name   string
		deps   map[string][]string
		order  []string
		cycles []Cycle
	}{
		{
			// C is ready from the start, so it runs before the cycle is
			// broken. Of A -> B and B -> A, A -> B has the smaller from.
			name:   "simple cycle and an independent operation",
			deps:   map[string][]string{"A": {"B"}, "B": {"A"}, "C": nil},
			order:  []string{"C", "B", "A"},
			cycles: []Cycle{{Ops: []string{"A", "B"}, Dropped: drop("A", "B")}},
		},
		{
			name:  "two separate cycles",
			deps:  map[string][]string{"A": {"B"}, "B": {"A"}, "C": {"D"}, "D": {"C"}},
			order: []string{"B", "A", "D", "C"},
			cycles: []Cycle{
				{Ops: []string{"A", "B"}, Dropped: drop("A", "B")},
				{Ops: []string{"C", "D"}, Dropped: drop("C", "D")},
			},
		},
		{
			// By id alone POST /x -> PUT /y would go ("POST" < "PUT"), but
			// its producer is a POST, so PUT /y -> POST /x is dropped.
			name:   "the edge whose producer is not a POST is dropped",
			deps:   map[string][]string{"POST /x": {"PUT /y"}, "PUT /y": {"POST /x"}},
			order:  []string{"POST /x", "PUT /y"},
			cycles: []Cycle{{Ops: []string{"POST /x", "PUT /y"}, Dropped: drop("PUT /y", "POST /x")}},
		},
		{
			name:   "both producers are POSTs: the smaller id wins",
			deps:   map[string][]string{"POST /a": {"POST /b"}, "POST /b": {"POST /a"}},
			order:  []string{"POST /b", "POST /a"},
			cycles: []Cycle{{Ops: []string{"POST /a", "POST /b"}, Dropped: drop("POST /a", "POST /b")}},
		},
		{
			// A -> B -> C -> A: three candidates, A -> B has the smallest from.
			name:   "three operations in one cycle",
			deps:   map[string][]string{"A": {"C"}, "B": {"A"}, "C": {"B"}},
			order:  []string{"B", "C", "A"},
			cycles: []Cycle{{Ops: []string{"A", "B", "C"}, Dropped: drop("A", "B")}},
		},
		{
			// D only waits for the cycle, so it is not in Ops.
			name:   "an operation waiting for a cycle is not part of it",
			deps:   map[string][]string{"A": {"B"}, "B": {"A"}, "D": {"A"}},
			order:  []string{"B", "A", "D"},
			cycles: []Cycle{{Ops: []string{"A", "B"}, Dropped: drop("A", "B")}},
		},
		{
			// A <-> B and B <-> C share B, so one component of three. After
			// A -> B goes, B still waits for C: a second drop in B <-> C.
			name:  "overlapping cycles need two drops",
			deps:  map[string][]string{"A": {"B"}, "B": {"A", "C"}, "C": {"B"}},
			order: []string{"C", "B", "A"},
			cycles: []Cycle{
				{Ops: []string{"A", "B", "C"}, Dropped: drop("A", "B")},
				{Ops: []string{"B", "C"}, Dropped: drop("B", "C")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, cycles, err := Sort(graph(tt.deps))
			if err != nil {
				t.Fatalf("Sort: unexpected error: %v", err)
			}
			if !slices.Equal(order, tt.order) {
				t.Errorf("Sort order = %q, want %q", order, tt.order)
			}
			if !reflect.DeepEqual(cycles, tt.cycles) {
				t.Errorf("Sort cycles:\n got: %+v\nwant: %+v", cycles, tt.cycles)
			}
		})
	}
}

// Picking the edge to drop walks maps, so like TestSortIsDeterministic this
// compares many runs, on a graph with two cycles and an operation waiting.
func TestSortCycleBreakingIsDeterministic(t *testing.T) {
	ops := graph(map[string][]string{
		"A": {"B"}, "B": {"A", "C"}, "C": {"B"},
		"POST /x": {"PUT /y"}, "PUT /y": {"POST /x"},
		"D": {"A", "POST /x"},
	})

	wantOrder, wantCycles, err := Sort(ops)
	if err != nil {
		t.Fatalf("Sort: unexpected error: %v", err)
	}
	if len(wantCycles) == 0 {
		t.Fatal("Sort found no cycle, the test checks nothing")
	}

	for i := range 200 {
		order, cycles, err := Sort(ops)
		if err != nil {
			t.Fatalf("run %d: Sort: unexpected error: %v", i, err)
		}
		if !slices.Equal(order, wantOrder) {
			t.Fatalf("run %d: Sort order changed:\n got: %q\nwant: %q", i, order, wantOrder)
		}
		if !reflect.DeepEqual(cycles, wantCycles) {
			t.Fatalf("run %d: Sort cycles changed:\n got: %+v\nwant: %+v", i, cycles, wantCycles)
		}
	}
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
