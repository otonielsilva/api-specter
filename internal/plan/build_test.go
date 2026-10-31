package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"slices"
	"testing"

	"github.com/otonielsilva/api-specter/internal/contract"
)

func TestBuild(t *testing.T) {
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: "GET", Path: "/pets"},
		{Method: "POST", Path: "/pets"},
	}}
	spec := []byte("openapi: 3.0.3")

	p := Build(c, "api.yaml", spec)

	sum := sha256.Sum256(spec)
	if want := "sha256:" + hex.EncodeToString(sum[:]); p.Spec.Hash != want {
		t.Errorf("Spec.Hash = %q, want %q", p.Spec.Hash, want)
	}
	if p.Spec.Path != "api.yaml" {
		t.Errorf("Spec.Path = %q, want api.yaml", p.Spec.Path)
	}
	if p.Version != 1 || p.Generator != HeuristicGenerated {
		t.Errorf("Version/Generator = %d/%q", p.Version, p.Generator)
	}
	// Only the set is checked: the order is decided by the grafo step later.
	if got, want := slices.Sorted(slices.Values(p.Order)), []string{"GET /pets", "POST /pets"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Order = %v, want operations %v", p.Order, want)
	}
}

func TestBuildListsEveryOperationWithEmptySlices(t *testing.T) {
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: "GET", Path: "/pets"},
		{Method: "DELETE", Path: "/pets/{id}"},
	}}

	p := Build(c, "api.yaml", nil)

	if len(p.Operations) != 2 {
		t.Fatalf("len(Operations) = %d, want 2", len(p.Operations))
	}
	for _, id := range []string{"GET /pets", "DELETE /pets/{id}"} {
		op, ok := p.Operations[id]
		if !ok {
			t.Errorf("missing operation %q", id)
			continue
		}
		if !op.Enabled {
			t.Errorf("%q: Enabled = false, want true", id)
		}
		if op.Tags == nil || op.Requires == nil {
			t.Errorf("%q: Tags/Requires = %#v/%#v, want empty non-nil", id, op.Tags, op.Requires)
		}
	}
	if p.SeedsNeeded == nil || p.Cycles == nil {
		t.Errorf("SeedsNeeded/Cycles = %#v/%#v, want empty non-nil", p.SeedsNeeded, p.Cycles)
	}
}

func TestBuildNoOperationsHasEmptyOrder(t *testing.T) {
	p := Build(&contract.Contract{}, "api.yaml", nil)
	if p.Order == nil || len(p.Order) != 0 {
		t.Errorf("Order = %#v, want empty non-nil slice", p.Order)
	}
}

func TestBuildPathParamWithPostOnCollectionRequiresProducer(t *testing.T) {
	// The GET comes first on purpose: the rule must not depend on the POST
	// having been seen already.
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: contract.GET, Path: "/animal/{id}"},
		{Method: contract.POST, Path: "/animal"},
		{Method: contract.POST, Path: "/orphan"},
	}}

	p := Build(c, "api.yaml", nil)

	op, ok := p.Operations["GET /animal/{id}"]
	if !ok {
		t.Fatal(`missing operation "GET /animal/{id}"`)
	}
	want := []Requirement{{
		Param:     "id",
		In:        Path,
		Producers: []Producer{{Op: "POST /animal", From: "response.body.id", Match: Unresolved}},
	}}
	if !reflect.DeepEqual(op.Requires, want) {
		t.Errorf("Requires = %#v, want %#v", op.Requires, want)
	}
}

func TestBuildOperationWithoutPathParamHasNoRequires(t *testing.T) {
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: contract.GET, Path: "/animal"},
		{Method: contract.POST, Path: "/animal"},
	}}

	p := Build(c, "api.yaml", nil)

	for _, id := range []string{"GET /animal", "POST /animal"} {
		op, ok := p.Operations[id]
		if !ok {
			t.Errorf("missing operation %q", id)
			continue
		}
		if op.Requires == nil || len(op.Requires) != 0 {
			t.Errorf("%q: Requires = %#v, want empty non-nil", id, op.Requires)
		}
	}
	if len(p.SeedsNeeded) != 0 {
		t.Errorf("SeedsNeeded = %#v, want none", p.SeedsNeeded)
	}
}

func TestBuildPathParamWithoutPostOnCollectionIsSeedNeeded(t *testing.T) {
	// /orphan and /stray have no POST. GET /orphan is there to show that only
	// a POST counts as a producer.
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: contract.GET, Path: "/orphan"},
		{Method: contract.GET, Path: "/orphan/{id}"},
		{Method: contract.GET, Path: "/stray/{id}"},
	}}

	p := Build(c, "api.yaml", nil)

	for _, id := range []string{"GET /orphan/{id}", "GET /stray/{id}"} {
		op, ok := p.Operations[id]
		if !ok {
			t.Errorf("missing operation %q", id)
			continue
		}
		if op.Requires == nil || len(op.Requires) != 0 {
			t.Errorf("%q: Requires = %#v, want empty non-nil", id, op.Requires)
		}
	}
	// Contract order, so the plan file does not change between runs.
	want := []SeedNeed{
		{Op: "GET /orphan/{id}", Param: "id", In: Path},
		{Op: "GET /stray/{id}", Param: "id", In: Path},
	}
	if !reflect.DeepEqual(p.SeedsNeeded, want) {
		t.Errorf("SeedsNeeded = %#v, want %#v", p.SeedsNeeded, want)
	}
}

func TestBuildTwoPathParamsRequireOneProducerEach(t *testing.T) {
	// The GET comes first on purpose, as in the single-param test.
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: contract.GET, Path: "/owner/{ownerId}/pet/{petId}"},
		{Method: contract.POST, Path: "/owner"},
		{Method: contract.POST, Path: "/owner/{ownerId}/pet"},
	}}

	p := Build(c, "api.yaml", nil)

	op, ok := p.Operations["GET /owner/{ownerId}/pet/{petId}"]
	if !ok {
		t.Fatal(`missing operation "GET /owner/{ownerId}/pet/{petId}"`)
	}
	// Left to right, each param produced by the POST of its own collection.
	want := []Requirement{
		{
			Param:     "ownerId",
			In:        Path,
			Producers: []Producer{{Op: "POST /owner", From: "response.body.id", Match: Unresolved}},
		},
		{
			Param:     "petId",
			In:        Path,
			Producers: []Producer{{Op: "POST /owner/{ownerId}/pet", From: "response.body.id", Match: Unresolved}},
		},
	}
	if !reflect.DeepEqual(op.Requires, want) {
		t.Errorf("Requires = %#v, want %#v", op.Requires, want)
	}
	if len(p.SeedsNeeded) != 0 {
		t.Errorf("SeedsNeeded = %#v, want none", p.SeedsNeeded)
	}
}

func TestBuildMixedPathParamsSplitBetweenRequiresAndSeeds(t *testing.T) {
	// POST /owner exists but POST /owner/{ownerId}/pet does not, so ownerId
	// has a producer and petId has none.
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: contract.GET, Path: "/owner/{ownerId}/pet/{petId}"},
		{Method: contract.POST, Path: "/owner"},
	}}

	p := Build(c, "api.yaml", nil)

	op, ok := p.Operations["GET /owner/{ownerId}/pet/{petId}"]
	if !ok {
		t.Fatal(`missing operation "GET /owner/{ownerId}/pet/{petId}"`)
	}
	wantRequires := []Requirement{{
		Param:     "ownerId",
		In:        Path,
		Producers: []Producer{{Op: "POST /owner", From: "response.body.id", Match: Unresolved}},
	}}
	if !reflect.DeepEqual(op.Requires, wantRequires) {
		t.Errorf("Requires = %#v, want %#v", op.Requires, wantRequires)
	}
	wantSeeds := []SeedNeed{
		{Op: "GET /owner/{ownerId}/pet/{petId}", Param: "petId", In: Path},
	}
	if !reflect.DeepEqual(p.SeedsNeeded, wantSeeds) {
		t.Errorf("SeedsNeeded = %#v, want %#v", p.SeedsNeeded, wantSeeds)
	}
}

func TestInferProducerFrom(t *testing.T) {
	post := func(status string, props ...string) contract.Operation {
		properties := make(map[string]*contract.Schema, len(props))
		for _, name := range props {
			properties[name] = &contract.Schema{Type: contract.SchemaTypeString}
		}
		return contract.Operation{
			Method: contract.POST,
			Path:   "/animals",
			Responses: map[string]*contract.Response{status: {Bodies: map[string]*contract.Schema{
				"application/json": {Type: contract.SchemaTypeObject, Properties: properties},
			}}},
		}
	}

	tests := []struct {
		name      string
		producer  contract.Operation
		param     string
		wantFrom  string
		wantMatch Match
	}{
		{"param name wins over id", post("201", "id", "petId"), "petId", "response.body.petId", MatchParam},
		{"param name ignores case and underscore", post("201", "pet_id"), "petId", "response.body.pet_id", MatchParam},
		{"collection id wins over id", post("201", "id", "animalId"), "id", "response.body.animalId", MatchCollection},
		{"collection id with underscore", post("201", "animal_id"), "id", "response.body.animal_id", MatchCollection},
		{"plain id", post("201", "id", "name"), "id", "response.body.id", MatchID},
		{"no match falls back to default", post("201", "name"), "id", defaultFrom, Unresolved},
		{"no responses falls back to default", contract.Operation{Path: "/animals"}, "id", defaultFrom, Unresolved},
		{"non 2xx response is ignored", post("400", "id"), "id", defaultFrom, Unresolved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, match := inferProducerFrom(tt.producer, tt.param)
			if from != tt.wantFrom || match != tt.wantMatch {
				t.Errorf("got (%q, %q), want (%q, %q)", from, match, tt.wantFrom, tt.wantMatch)
			}
		})
	}
}
