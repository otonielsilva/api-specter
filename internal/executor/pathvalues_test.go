package executor

import (
	"errors"
	"maps"
	"testing"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/plan"
)

// pathValues builds the map that resolvePath needs: for each path parameter of
// the operation, the text of the first value that the plan's producers have in
// the Pool. The plan says who can produce a parameter and from where; the Pool
// says what was actually captured; the schema of the parameter says whether
// the captured value is acceptable (paramString).
func TestPathValues(t *testing.T) {
	var (
		integer = &contract.Schema{Type: contract.SchemaTypeInteger}
		str     = &contract.Schema{Type: contract.SchemaTypeString}

		animalID  = Key{Op: "POST /animals", From: "response.body.id"}
		animalAlt = Key{Op: "POST /zoo/animals", From: "response.body.id"}
	)

	// pathParam is a required path parameter of the operation.
	pathParam := func(name string, schema *contract.Schema) contract.Param {
		return contract.Param{In: contract.ParamInPath, Name: name, Required: true, Schema: schema}
	}

	// requirement is what the plan says about a path parameter. Each key is a
	// producer, tried in this order.
	requirement := func(param string, producers ...Key) plan.Requirement {
		r := plan.Requirement{Param: param, In: plan.Path}
		for _, k := range producers {
			r.Producers = append(r.Producers, plan.Producer{Op: k.Op, From: k.From, Match: plan.MatchID})
		}
		return r
	}

	tests := []struct {
		name     string
		params   []contract.Param
		requires []plan.Requirement
		pool     map[Key]any // what the Pool holds; nil means empty
		want     map[string]string
		wantErr  error // nil means success; otherwise errors.Is must match
	}{
		// Nothing to fill.
		{
			name: "operation without parameters",
			want: map[string]string{},
		},
		{
			name:   "only query and header parameters are not path parameters",
			params: []contract.Param{{In: contract.ParamInQuery, Name: "start", Required: true, Schema: integer}, {In: contract.ParamInHeader, Name: "X-Id", Required: true, Schema: str}},
			want:   map[string]string{},
		},

		// One parameter.
		{
			name:     "value of the only producer",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID)},
			pool:     map[Key]any{animalID: float64(7)},
			want:     map[string]string{"id": "7"},
		},
		{
			name:     "the name of the parameter is not the name of the field",
			params:   []contract.Param{pathParam("animalId", integer)},
			requires: []plan.Requirement{requirement("animalId", animalID)},
			pool:     map[Key]any{animalID: float64(7)},
			want:     map[string]string{"animalId": "7"},
		},
		{
			name:     "string schema takes a string value",
			params:   []contract.Param{pathParam("id", str)},
			requires: []plan.Requirement{requirement("id", animalID)},
			pool:     map[Key]any{animalID: "abc"},
			want:     map[string]string{"id": "abc"},
		},

		// Several producers: the first one that has a usable value wins.
		{
			name:     "first producer wins when both have a value",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID, animalAlt)},
			pool:     map[Key]any{animalID: float64(1), animalAlt: float64(2)},
			want:     map[string]string{"id": "1"},
		},
		{
			name:     "second producer when the first has no value",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID, animalAlt)},
			pool:     map[Key]any{animalAlt: float64(2)},
			want:     map[string]string{"id": "2"},
		},
		{
			name:     "second producer when the value of the first does not fit the schema",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID, animalAlt)},
			pool:     map[Key]any{animalID: 1.5, animalAlt: float64(2)},
			want:     map[string]string{"id": "2"},
		},
		{
			name:     "a null value is not usable, so the next producer is tried",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID, animalAlt)},
			pool:     map[Key]any{animalID: nil, animalAlt: float64(2)},
			want:     map[string]string{"id": "2"},
		},

		// Several parameters.
		{
			name:   "two parameters, each with its own producer",
			params: []contract.Param{pathParam("zoo", integer), pathParam("id", integer)},
			requires: []plan.Requirement{
				requirement("zoo", Key{Op: "POST /zoos", From: "response.body.id"}),
				requirement("id", animalID),
			},
			pool: map[Key]any{{Op: "POST /zoos", From: "response.body.id"}: float64(3), animalID: float64(7)},
			want: map[string]string{"zoo": "3", "id": "7"},
		},
		{
			name:   "one of two parameters has no value",
			params: []contract.Param{pathParam("zoo", integer), pathParam("id", integer)},
			requires: []plan.Requirement{
				requirement("zoo", Key{Op: "POST /zoos", From: "response.body.id"}),
				requirement("id", animalID),
			},
			pool:    map[Key]any{animalID: float64(7)},
			wantErr: ErrMissingParam,
		},

		// The plan asks for the value somewhere else, not for a path parameter.
		{
			name:     "a requirement for the body does not fill a path parameter",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{{Param: "id", In: plan.Body, Producers: []plan.Producer{{Op: animalID.Op, From: animalID.From}}}},
			pool:     map[Key]any{animalID: float64(7)},
			wantErr:  ErrMissingParam,
		},
		{
			name:     "requirement of another parameter is not used",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("other", animalID)},
			pool:     map[Key]any{animalID: float64(7)},
			wantErr:  ErrMissingParam,
		},

		// No usable value.
		{
			name:     "producer has not run, the pool is empty",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID)},
			wantErr:  ErrMissingParam,
		},
		{
			name:    "no requirement: a seed nobody produces",
			params:  []contract.Param{pathParam("id", integer)},
			wantErr: ErrMissingParam,
		},
		{
			name:     "requirement without producers",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id")},
			wantErr:  ErrMissingParam,
		},
		{
			name:     "the pool holds a value of another field of the same producer",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID)},
			pool:     map[Key]any{{Op: animalID.Op, From: "response.body.name"}: "rex"},
			wantErr:  ErrMissingParam,
		},
		{
			name:     "every producer has a value that does not fit",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID, animalAlt)},
			pool:     map[Key]any{animalID: 1.5, animalAlt: "7"},
			wantErr:  ErrMissingParam,
		},
		{
			name:     "a value above 2^53 is refused, not rounded",
			params:   []contract.Param{pathParam("id", integer)},
			requires: []plan.Requirement{requirement("id", animalID)},
			pool:     map[Key]any{animalID: float64(1<<53 + 2)},
			wantErr:  ErrMissingParam,
		},
		{
			name:     "parameter without a schema",
			params:   []contract.Param{pathParam("id", nil)},
			requires: []plan.Requirement{requirement("id", animalID)},
			pool:     map[Key]any{animalID: float64(7)},
			wantErr:  ErrMissingParam,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := contract.Operation{Method: contract.GET, Path: "/animals/{id}", Params: tt.params}
			operator := plan.Operator{Enabled: true, Requires: tt.requires}

			pool := NewPool()
			for k, v := range tt.pool {
				pool.Put(k, v)
			}

			got, err := pathValues(op, operator, pool)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("pathValues() error = %v, want %v", err, tt.wantErr)
				}
				if got != nil {
					t.Errorf("pathValues() = %v on error, want nil", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("pathValues() unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("pathValues() = nil, want a map (empty when there is nothing to fill)")
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("pathValues() = %v, want %v", got, tt.want)
			}
		})
	}
}
