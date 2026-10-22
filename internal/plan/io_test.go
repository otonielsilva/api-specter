package plan

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLoadZooPlan(t *testing.T) {
	f, err := os.Open("testdata/zoo.plan.json")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()

	p, err := Load(f)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if p.Generator != "heuristic/0.1" {
		t.Errorf("Generator = %q, want %q", p.Generator, "heuristic/0.1")
	}
	if got := len(p.Operations); got != 15 {
		t.Errorf("len(Operations) = %d, want 15", got)
	}
	if got := len(p.Order); got != 15 {
		t.Errorf("len(Order) = %d, want 15", got)
	}

	requirements := 0
	for _, op := range p.Operations {
		requirements += len(op.Requires)
	}
	if requirements != 11 {
		t.Errorf("requirements = %d, want 11", requirements)
	}

	// Every id in Order must name an operation, or run would skip it silently.
	for _, id := range p.Order {
		if _, ok := p.Operations[id]; !ok {
			t.Errorf("Order has %q, which is not in Operations", id)
		}
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name           string
		json           string
		wantErr        error // sentinel, nil when encoding/json produced the error
		wantErrMessage string
	}{
		{"unknown field", `{"test": "2"}`, nil, "plan: decode"},
		{"unsupported version number", `{"version": 2}`, ErrVersion, "got 2, want 1"},
		{"data after the plan", `{"version": 1} {}`, ErrInvalid, "data after the plan"},
		{
			"unknown location in requires",
			`{"version": 1, "operations": {"GET /a": {"requires": [{"param": "id", "in": "cookie"}]}}}`,
			ErrInvalid, `unknown location "cookie"`,
		},
		{
			"unknown location in seeds_needed",
			`{"version": 1, "seeds_needed": [{"op": "GET /a", "param": "id", "in": "cookie"}]}`,
			ErrInvalid, `unknown location "cookie"`,
		},
		{"truncated JSON", `{"version": 1,`, nil, "plan: decode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(strings.NewReader(tt.json))

			if got != nil {
				t.Errorf("Load() plan = %v, want nil", got)
			}

			if err == nil {
				t.Fatal("Load() error = nil, want an error")
			}

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Load() error = %v, want %v", err, tt.wantErr)
			}

			if !strings.Contains(err.Error(), tt.wantErrMessage) {
				t.Errorf("Load() error = %v, want contains %q", err, tt.wantErrMessage)
			}
		})
	}
}

func TestLoadFillsEmpty(t *testing.T) {
	// One operation with no tags and a requirement with no producers, one
	// operation with no requires. order, seeds_needed and cycles are absent.
	const in = `{
		"version": 1,
		"operations": {
			"GET /a": {"enabled": true},
			"GET /b/{id}": {"enabled": true, "requires": [{"param": "id", "in": "path"}]}
		}
	}`

	p, err := Load(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if p.Order == nil {
		t.Error("Order is nil, want empty slice")
	}
	if p.SeedsNeeded == nil {
		t.Error("SeedsNeeded is nil, want empty slice")
	}
	if p.Cycles == nil {
		t.Error("Cycles is nil, want empty slice")
	}

	for id, op := range p.Operations {
		if op.Tags == nil {
			t.Errorf("%q: Tags is nil, want empty slice", id)
		}
		if op.Requires == nil {
			t.Errorf("%q: Requires is nil, want empty slice", id)
		}
		for _, req := range op.Requires {
			if req.Producers == nil {
				t.Errorf("%q: Producers of %q is nil, want empty slice", id, req.Param)
			}
		}
	}

	// A plan with no operations at all gets an empty map, not a nil one.
	empty, err := Load(strings.NewReader(`{"version": 1}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if empty.Operations == nil {
		t.Error("Operations is nil, want empty map")
	}
}
