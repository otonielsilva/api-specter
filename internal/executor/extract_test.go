package executor

import (
	"maps"
	"os"
	"slices"
	"testing"

	"github.com/otonielsilva/api-specter/internal/plan"
)

// loadZooPlan reads the golden plan of the zoo. Each test gets its own copy,
// so a test may change it.
func loadZooPlan(t *testing.T) *plan.Plan {
	t.Helper()

	f, err := os.Open("../plan/testdata/zoo.plan.json")
	if err != nil {
		t.Fatalf("open golden plan: %v", err)
	}
	defer f.Close()

	p, err := plan.Load(f)
	if err != nil {
		t.Fatalf("load golden plan: %v", err)
	}
	return p
}

// forEachProducer calls fn for every producer of every requirement in p. fn
// gets a pointer, so it can change the producer in place.
func forEachProducer(p *plan.Plan, fn func(consumer string, prod *plan.Producer)) {
	for id, op := range p.Operations {
		for i := range op.Requires {
			for j := range op.Requires[i].Producers {
				fn(id, &op.Requires[i].Producers[j])
			}
		}
	}
}

// sameExtractions fails the test when got and want differ.
func sameExtractions(t *testing.T, got, want map[string][]string) {
	t.Helper()

	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("extract() = %v, want %v", got, want)
	}
}

func TestExtractEmptyPlan(t *testing.T) {
	got := extract(&plan.Plan{})

	if got == nil {
		t.Fatal("extract() = nil, want an empty map")
	}
	if len(got) != 0 {
		t.Errorf("extract() = %v, want it empty", got)
	}
}

// In the golden plan only POST /animals and POST /feeds produce a value, and
// both are read from response.body.id. Many operations ask for it, so this
// also checks that a place is listed once, no matter how many ask for it.
func TestExtractZoo(t *testing.T) {
	got := extract(loadZooPlan(t))

	want := map[string][]string{
		"POST /animals": {"response.body.id"},
		"POST /feeds":   {"response.body.id"},
	}
	sameExtractions(t, got, want)
}

// The index is by producer. An operation that only consumes, such as
// GET /animals/{id}, or that needs nothing, such as GET /healthz, has no entry.
func TestExtractIsIndexedByProducer(t *testing.T) {
	got := extract(loadZooPlan(t))

	for _, id := range []string{"GET /animals/{id}", "DELETE /feeds/{id}", "GET /healthz", "GET /animals"} {
		if _, found := got[id]; found {
			t.Errorf("extract() has an entry for %q, which produces nothing", id)
		}
	}
}

// A disabled operation never runs, so it asks for nothing. With every
// consumer of POST /feeds off, nothing needs its response.
func TestExtractIgnoresDisabledConsumers(t *testing.T) {
	p := loadZooPlan(t)

	forEachProducer(p, func(consumer string, prod *plan.Producer) {
		if prod.Op != "POST /feeds" {
			return
		}
		op := p.Operations[consumer]
		op.Enabled = false
		p.Operations[consumer] = op
	})

	got := extract(p)

	want := map[string][]string{
		"POST /animals": {"response.body.id"},
	}
	sameExtractions(t, got, want)
}

// An unresolved producer is a guess that needs review, so it is not extracted.
func TestExtractIgnoresUnresolvedProducers(t *testing.T) {
	p := loadZooPlan(t)

	forEachProducer(p, func(_ string, prod *plan.Producer) {
		if prod.Op == "POST /feeds" {
			prod.Match = plan.Unresolved
		}
	})

	got := extract(p)

	want := map[string][]string{
		"POST /animals": {"response.body.id"},
	}
	sameExtractions(t, got, want)
}

// The consumers of one producer may ask for different places. The result lists
// each once and in order, whatever order the map of operations is walked in.
func TestExtractSortsPlaces(t *testing.T) {
	p := loadZooPlan(t)

	// Make the consumers of POST /animals ask for four different places.
	places := []string{"response.body.name", "response.body.id", "response.body.age", "response.body.id"}
	i := 0
	forEachProducer(p, func(_ string, prod *plan.Producer) {
		if prod.Op != "POST /animals" {
			return
		}
		prod.From = places[i%len(places)]
		i++
	})

	want := map[string][]string{
		"POST /animals": {"response.body.age", "response.body.id", "response.body.name"},
		"POST /feeds":   {"response.body.id"},
	}

	// Walking a map is random on purpose, so one run proves little.
	for range 200 {
		sameExtractions(t, extract(p), want)
		if t.Failed() {
			return
		}
	}
}

// A producer that is not in the plan is indexed anyway: the executor never
// reaches it, so the entry is harmless, and extract needs no special case.
func TestExtractIndexesProducerOutsidePlan(t *testing.T) {
	p := loadZooPlan(t)

	op := p.Operations["GET /healthz"]
	op.Requires = []plan.Requirement{{
		Param: "token",
		In:    plan.Header,
		Producers: []plan.Producer{
			{Op: "POST /login", From: "response.body.token", Match: plan.MatchParam},
		},
	}}
	p.Operations["GET /healthz"] = op

	got := extract(p)

	want := map[string][]string{
		"POST /animals": {"response.body.id"},
		"POST /feeds":   {"response.body.id"},
		"POST /login":   {"response.body.token"},
	}
	sameExtractions(t, got, want)
}
