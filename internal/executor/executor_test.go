package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/plan"
)

func TestRun(t *testing.T) {
	const id = "GET /animals"

	known := &contract.Contract{
		Operations: []contract.Operation{{Method: contract.GET, Path: "/animals"}},
	}

	tests := []struct {
		name    string
		plan    *plan.Plan
		c       *contract.Contract
		wantErr error
	}{
		{
			name: "operation in the contract",
			plan: &plan.Plan{
				Order:      []string{id},
				Operations: map[string]plan.Operator{id: {Enabled: true}},
			},
			c: known,
		},
		{
			name: "operation missing from the contract",
			plan: &plan.Plan{
				Order:      []string{id},
				Operations: map[string]plan.Operator{id: {Enabled: true}},
			},
			c:       &contract.Contract{},
			wantErr: ErrUnknownOperation,
		},
		{
			name: "disabled operation is skipped even if missing",
			plan: &plan.Plan{
				Order:      []string{id},
				Operations: map[string]plan.Operator{id: {Enabled: false}},
			},
			c: &contract.Contract{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := New(Config{})

			_, err := ex.Run(context.Background(), tt.plan, tt.c)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Run() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
