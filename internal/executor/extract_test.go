package executor

import (
	"maps"
	"os"
	"reflect"
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

func TestLookup(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		from      string
		wantFound bool
		wantValue any
	}{
		// encoding/json decodes every JSON number into float64.
		{name: "Value fount at root", from: "response.body.id", body: "{\"id\": 1}", wantValue: float64(1), wantFound: true},
		{name: "Value found at null", from: "response.body.id", body: "{\"id\": null}", wantFound: true},
		{name: "Value not found", from: "response.body.idx", body: "{\"id\": 1}"},
		{name: "Body nil", from: "response.body.idx"},
		{name: "From nil", body: "{\"id\": 1}"},

		// Values of each type.
		{name: "String", from: "response.body.id", body: `{"id": "abc"}`, wantValue: "abc", wantFound: true},
		{name: "Bool", from: "response.body.id", body: `{"id": true}`, wantValue: true, wantFound: true},
		{name: "Object", from: "response.body.id", body: `{"id": {"x": 1}}`, wantValue: map[string]any{"x": float64(1)}, wantFound: true},
		{name: "Array", from: "response.body.id", body: `{"id": [1, 2]}`, wantValue: []any{float64(1), float64(2)}, wantFound: true},
		{name: "Zero is not absent", from: "response.body.id", body: `{"id": 0}`, wantValue: float64(0), wantFound: true},
		{name: "Empty string is not absent", from: "response.body.id", body: `{"id": ""}`, wantValue: "", wantFound: true},
		{name: "False is not absent", from: "response.body.id", body: `{"id": false}`, wantValue: false, wantFound: true},

		// Absent field.
		{name: "Empty object", from: "response.body.id", body: `{}`},
		{name: "Other field only", from: "response.body.id", body: `{"name": "rex"}`},
		{name: "Field name is case sensitive", from: "response.body.id", body: `{"ID": 1}`},

		// Body that is not an object.
		{name: "Empty body", from: "response.body.id", body: ``},
		{name: "Not JSON", from: "response.body.id", body: `not json`},
		{name: "HTML", from: "response.body.id", body: `<html><body>502 Bad Gateway</body></html>`},
		{name: "Truncated JSON", from: "response.body.id", body: `{"id": 1`},
		{name: "Array at root", from: "response.body.id", body: `[{"id": 1}]`},
		{name: "Number at root", from: "response.body.id", body: `42`},
		{name: "String at root", from: "response.body.id", body: `"id"`},
		{name: "Null at root", from: "response.body.id", body: `null`},

		// Invalid or unsupported from.
		{name: "Header is not supported", from: "response.header.Location", body: `{"Location": "/animals/1"}`},
		{name: "Missing response prefix", from: "body.id", body: `{"id": 1}`},
		{name: "Prefix only", from: "response.body", body: `{"id": 1}`},

		// Nested path.
		{name: "Nested path", from: "response.body.data.id", body: `{"data": {"id": 1}}`, wantValue: float64(1), wantFound: true},
		{name: "Nested path, three levels", from: "response.body.a.b.c", body: `{"a": {"b": {"c": "x"}}}`, wantValue: "x", wantFound: true},
		{name: "Nested path ending in an object", from: "response.body.data", body: `{"data": {"id": 1}}`, wantValue: map[string]any{"id": float64(1)}, wantFound: true},
		{name: "Nested null value", from: "response.body.data.id", body: `{"data": {"id": null}}`, wantFound: true},
		{name: "Nested key missing", from: "response.body.data.id", body: `{"data": {"name": "rex"}}`},
		{name: "Middle step is not an object", from: "response.body.data.id", body: `{"data": 5}`},
		{name: "Middle step is an array", from: "response.body.data.id", body: `{"data": [{"id": 1}]}`},
		{name: "Middle step is null", from: "response.body.data.id", body: `{"data": null}`},
		{name: "Flat key with a dot is not reachable", from: "response.body.a.b", body: `{"a.b": 1}`},
		{name: "Empty key at the end", from: "response.body.data.", body: `{"data": {"": 1}}`},
		{name: "Empty key in the middle", from: "response.body.data..id", body: `{"data": {"id": 1}}`},

		// Shape of the JSON.
		{name: "Duplicate key keeps the last", from: "response.body.id", body: `{"id": 1, "id": 2}`, wantValue: float64(2), wantFound: true},
		{name: "Indented JSON", from: "response.body.id", body: "{\n  \"id\": 7\n}\n", wantValue: float64(7), wantFound: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := []byte(tt.body)
			got, found := lookup(b, tt.from)
			if tt.wantFound != found {
				t.Errorf("lookup() found = %v, want %v", found, tt.wantFound)
			}
			if !reflect.DeepEqual(got, tt.wantValue) {
				t.Errorf("lookup() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}
