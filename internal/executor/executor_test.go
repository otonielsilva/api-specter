package executor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
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
		name      string
		plan      *plan.Plan
		c         *contract.Contract
		noBaseURL bool
		wantHits  int32
		wantErr   error
	}{
		{
			name: "operation in the contract",
			plan: &plan.Plan{
				Order:      []string{id},
				Operations: map[string]plan.Operator{id: {Enabled: true}},
			},
			c:        known,
			wantHits: 1,
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
		{
			name: "no base URL",
			plan: &plan.Plan{
				Order:      []string{id},
				Operations: map[string]plan.Operator{id: {Enabled: true}},
			},
			c:         known,
			noBaseURL: true,
			wantErr:   ErrNoBaseURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The handler runs in another goroutine, so the counter is atomic.
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			var cfg Config
			if !tt.noBaseURL {
				base, err := url.Parse(srv.URL)
				if err != nil {
					t.Fatalf("url.Parse(%q) = %v", srv.URL, err)
				}
				cfg.BaseURL = base
			}

			_, err := New(cfg).Run(context.Background(), tt.plan, tt.c)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Run() error = %v, want %v", err, tt.wantErr)
			}
			if got := hits.Load(); got != tt.wantHits {
				t.Errorf("server got %d requests, want %d", got, tt.wantHits)
			}
		})
	}
}
