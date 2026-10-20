package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
)

// Version is the plan format version this package reads.
const Version = 1

var (
	// ErrVersion reports a plan whose version this package does not support.
	ErrVersion = errors.New("unsupported plan version")

	// ErrInvalid reports a plan that is valid JSON but breaks the format.
	ErrInvalid = errors.New("invalid plan")
)

// Load reads a plan from r.
//
// It rejects unknown fields, data after the plan, an unsupported version and
// parameter locations other than path, query, header and body. Lists that are
// missing in the input come back empty rather than nil, so writing the plan
// back produces [] and not null.
func Load(r io.Reader) (*Plan, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var p Plan
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("plan: decode: %w", err)
	}

	// Decode stops after the first value; anything but EOF here is extra data.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("plan: %w: data after the plan", ErrInvalid)
	}

	if p.Version != Version {
		return nil, fmt.Errorf("plan: %w: got %d, want %d", ErrVersion, p.Version, Version)
	}

	if err := p.validateLocations(); err != nil {
		return nil, err
	}

	p.fillEmpty()

	return &p, nil
}

// validateLocations checks every "in" field. Operations are visited in sorted
// order so the same bad plan always reports the same error.
func (p *Plan) validateLocations() error {
	for _, id := range slices.Sorted(maps.Keys(p.Operations)) {
		for _, req := range p.Operations[id].Requires {
			if !req.In.valid() {
				return fmt.Errorf("plan: %w: operation %q: unknown location %q for %q",
					ErrInvalid, id, req.In, req.Param)
			}
		}
	}

	for _, seed := range p.SeedsNeeded {
		if !seed.In.valid() {
			return fmt.Errorf("plan: %w: seed for %q: unknown location %q for %q",
				ErrInvalid, seed.Op, seed.In, seed.Param)
		}
	}

	return nil
}

func (l Location) valid() bool {
	switch l {
	case Path, QueryParam, Header, Body:
		return true
	}

	return false
}

// fillEmpty replaces nil slices and maps with empty ones.
func (p *Plan) fillEmpty() {
	if p.Order == nil {
		p.Order = []string{}
	}

	if p.SeedsNeeded == nil {
		p.SeedsNeeded = []SeedNeed{}
	}

	if p.Cycles == nil {
		p.Cycles = []Cycle{}
	}

	if p.Operations == nil {
		p.Operations = map[string]Operator{}
	}

	for id, op := range p.Operations {
		if op.Tags == nil {
			op.Tags = []string{}
		}

		if op.Requires == nil {
			op.Requires = []Requirement{}
		}

		for i := range op.Requires {
			if op.Requires[i].Producers == nil {
				op.Requires[i].Producers = []Producer{}
			}
		}

		// The map holds copies of the structs, so write the change back.
		p.Operations[id] = op
	}
}
