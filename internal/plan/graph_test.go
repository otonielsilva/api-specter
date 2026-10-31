package plan

import "testing"

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

func TestSortZooMatchesGoldenOrder(t *testing.T) {
	t.Skip("TODO: write me")
	// Build the zoo graph by hand (11 edges, table in docs/design/zoo-graph.md):
	//   POST /animals -> GET, PUT, PATCH, DELETE /animals/{id},
	//                    GET and POST /animals/{id}/feedings   (id, path)
	//   POST /feeds   -> GET, PUT, DELETE /feeds/{id},
	//                    POST /feeds/{id}/restock              (id, path)
	//   POST /feeds   -> POST /animals/{id}/feedings           (feedId, body)
	// Plus the 3 operations with no dependency: GET /healthz, GET /animals,
	// GET /feeds. 15 operations in total.
	// Expect exactly the 15-line order of the table, no cycles, nil error.
	// Alternative: load testdata/zoo.plan.json and compare Sort(p.Operations)
	// with p.Order, so the fixture is not written twice.
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
