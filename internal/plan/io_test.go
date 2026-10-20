package plan

import "testing"

func TestLoadZooPlan(t *testing.T) {
	t.Skip("TODO: load testdata/zoo.plan.json; expect 15 operations, 15 ids in Order, 11 requirements, Generator heuristic/0.1")
}

func TestLoadRejects(t *testing.T) {
	t.Skip("TODO: table-driven; unknown field, version 2 (ErrVersion), data after the plan (ErrInvalid), unknown location in requires and in seeds_needed (ErrInvalid), truncated JSON")
}

func TestLoadFillsEmpty(t *testing.T) {
	t.Skip("TODO: a plan with no order, tags, requires, producers, seeds_needed or cycles comes back with empty slices and map, not nil")
}
