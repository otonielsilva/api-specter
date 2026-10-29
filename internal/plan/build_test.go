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
