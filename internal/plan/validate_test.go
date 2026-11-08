package plan

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	// B and C require A, so A must come first.
	ops := graph(map[string][]string{"A": nil, "B": {"A"}, "C": {"A"}})

	tests := []struct {
		name    string
		plan    *Plan
		wantMsg string // "" when the plan is valid
	}{
		{
			name: "valid order",
			plan: &Plan{Order: []string{"A", "B", "C"}, Operations: ops},
		},
		{
			name:    "operation missing from order",
			plan:    &Plan{Order: []string{"A", "B"}, Operations: ops},
			wantMsg: `operation "C" is missing from order`,
		},
		{
			name:    "order names an unknown operation",
			plan:    &Plan{Order: []string{"A", "B", "C", "X"}, Operations: ops},
			wantMsg: `order lists "X", which is not in operations`,
		},
		{
			name:    "producer after its consumer",
			plan:    &Plan{Order: []string{"B", "A", "C"}, Operations: ops},
			wantMsg: `operation "B" at 0 runs before its producer "A" at 1`,
		},
		{
			name:    "duplicated id in order",
			plan:    &Plan{Order: []string{"A", "B", "B", "C"}, Operations: ops},
			wantMsg: `order lists "B" twice, at 1 and 2`,
		},
		{
			name:    "producer not in operations",
			plan:    &Plan{Order: []string{"B"}, Operations: graph(map[string][]string{"B": {"X"}})},
			wantMsg: `operation "B" requires "id" from "X", which is not in operations`,
		},
		{
			name: "self-loop is ignored",
			plan: &Plan{Order: []string{"A"}, Operations: graph(map[string][]string{"A": {"A"}})},
		},
		{
			// Sort dropped A -> B to break the cycle, so B runs before A.
			name: "edge dropped by a cycle is ignored",
			plan: &Plan{
				Order:      []string{"B", "A"},
				Operations: graph(map[string][]string{"A": {"B"}, "B": {"A"}}),
				Cycles:     []Cycle{{Ops: []string{"A", "B"}, Dropped: drop("A", "B")}},
			},
		},
		{
			// Only A -> B is excused, B -> A still has to hold.
			name: "edge not dropped is still checked",
			plan: &Plan{
				Order:      []string{"A", "B"},
				Operations: graph(map[string][]string{"A": {"B"}, "B": {"A"}}),
				Cycles:     []Cycle{{Ops: []string{"A", "B"}, Dropped: drop("A", "B")}},
			},
			wantMsg: `operation "A" at 0 runs before its producer "B" at 1`,
		},
		{
			name: "no operations",
			plan: &Plan{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.plan)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("Validate: unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate error = %v, want %v", err, ErrInvalid)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Validate error = %q, want it to contain %q", err, tt.wantMsg)
			}
		})
	}
}

func TestValidateZooPlan(t *testing.T) {
	if err := Validate(loadZoo(t)); err != nil {
		t.Errorf("Validate(zoo plan): %v", err)
	}
}

// Whatever Sort returns, cycles included, passes Validate.
func TestValidateAcceptsSortOutput(t *testing.T) {
	ops := graph(map[string][]string{
		"A": {"B"}, "B": {"A", "C"}, "C": {"B"},
		"POST /x": {"PUT /y"}, "PUT /y": {"POST /x"},
		"D": {"A", "POST /x"},
	})
	order, cycles, err := Sort(ops)
	if err != nil {
		t.Fatalf("Sort: unexpected error: %v", err)
	}
	if err := Validate(&Plan{Order: order, Operations: ops, Cycles: cycles}); err != nil {
		t.Errorf("Validate(Sort output): %v", err)
	}
}
