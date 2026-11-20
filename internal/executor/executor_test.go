package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/oracle"
	"github.com/otonielsilva/api-specter/internal/plan"
)

// getAnimals returns a plan and a contract with the single operation
// GET /animals, enabled.
func getAnimals() (*plan.Plan, *contract.Contract) {
	const id = "GET /animals"

	p := &plan.Plan{
		Order:      []string{id},
		Operations: map[string]plan.Operator{id: {Enabled: true}},
	}
	c := &contract.Contract{
		Operations: []contract.Operation{{Method: contract.GET, Path: "/animals"}},
	}

	return p, c
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) = %v", raw, err)
	}

	return u
}

// A request that fails in the transport is an exchange for the oracles to
// judge, not an error of the run: Run returns the finding and no error.
func TestRunTransportFailureIsAFinding(t *testing.T) {
	t.Run("server is down", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		base := mustParse(t, srv.URL)
		srv.Close() // nothing listens on base any more

		p, c := getAnimals()

		got, err := New(Config{BaseURL: base, Oracles: []oracle.Oracle{oracle.SimpleOracle{}}}).Run(context.Background(), p, c)
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if len(got) != 1 {
			t.Fatalf("Run() returned %d findings, want 1: %+v", len(got), got)
		}
	})

	t.Run("server is slower than the request timeout", func(t *testing.T) {
		// The handler waits until the client gives up or the test ends, so
		// srv.Close never blocks on it.
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		// Cleanups run last in, first out: release the handler, then close.
		t.Cleanup(srv.Close)
		t.Cleanup(func() { close(release) })

		p, c := getAnimals()
		ex := New(Config{BaseURL: mustParse(t, srv.URL), RequestTimeout: 50 * time.Millisecond, Oracles: []oracle.Oracle{oracle.SimpleOracle{}}})

		start := time.Now()
		got, err := ex.Run(context.Background(), p, c)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if len(got) != 1 {
			t.Fatalf("Run() returned %d findings, want 1: %+v", len(got), got)
		}
		// The timeout is 50ms. Anything near the 10s default means the
		// per-request deadline was not applied.
		if elapsed > 2*time.Second {
			t.Errorf("Run() took %v, want about the 50ms timeout", elapsed)
		}
	})
}

// Cancelling the run stops it before any request is sent, and the error
// says why.
func TestRunStopsWhenCancelled(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p, c := getAnimals()

	_, err := New(Config{BaseURL: mustParse(t, srv.URL)}).Run(ctx, p, c)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("server got %d requests, want 0", got)
	}
}

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

// Without oracles Run is not an error: it still sends the requests, finds
// nothing and logs one warning, so the missing oracles do not go unnoticed.
func TestRunWithoutOraclesWarns(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError) // would be a finding with SimpleOracle
	}))
	t.Cleanup(srv.Close)

	// slog.Default is global: capture it for this test and put it back.
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	p, c := getAnimals()

	got, err := New(Config{BaseURL: mustParse(t, srv.URL)}).Run(context.Background(), p, c)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if len(got) != 0 {
		t.Errorf("Run() returned %d findings, want 0: %+v", len(got), got)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("server got %d requests, want 1", n)
	}
	if out := logs.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, "no oracles configured") {
		t.Errorf("log = %q, want a warning about no oracles", out)
	}
}

// Run sends every method, in the order of the plan and not in the order of
// the contract. Disabled operations and operations with a path parameter
// that has no value yet are not sent.
func TestRunSendsInPlanOrder(t *testing.T) {
	var (
		mu       sync.Mutex
		received []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	// The contract lists GET first. The plan puts POST first, which is how a
	// producer runs before its consumers, and ends with DELETE.
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: contract.GET, Path: "/animals"},
		{Method: contract.POST, Path: "/animals"},
		{Method: contract.GET, Path: "/animals/{id}"},
		{Method: contract.PUT, Path: "/animals"},
		{Method: contract.DELETE, Path: "/animals"},
	}}
	p := &plan.Plan{
		Order: []string{
			"POST /animals",
			"GET /animals",
			"GET /animals/{id}", // path parameter without a value: skipped
			"PUT /animals",      // disabled: skipped
			"DELETE /animals",
		},
		Operations: map[string]plan.Operator{
			"POST /animals":     {Enabled: true},
			"GET /animals":      {Enabled: true},
			"GET /animals/{id}": {Enabled: true},
			"PUT /animals":      {Enabled: false},
			"DELETE /animals":   {Enabled: true},
		},
	}

	_, err := New(Config{BaseURL: mustParse(t, srv.URL)}).Run(context.Background(), p, c)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	want := []string{"POST /animals", "GET /animals", "DELETE /animals"}

	mu.Lock()
	defer mu.Unlock()

	if !slices.Equal(received, want) {
		t.Errorf("server received %v, want %v", received, want)
	}
}

// A POST whose contract has a JSON body schema sends the minimal value of
// that schema, with the Content-Type header.
func TestRunSendsMinimalBody(t *testing.T) {
	var (
		mu          sync.Mutex
		gotBody     []byte
		gotType     string
		gotContains bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)

		mu.Lock()
		gotBody, gotType, gotContains = b, r.Header.Get("Content-Type"), true
		mu.Unlock()

		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	const id = "POST /animals"
	c := &contract.Contract{Operations: []contract.Operation{{
		Method: contract.POST,
		Path:   "/animals",
		Bodies: map[string]*contract.Schema{
			"application/json": {
				Type:     contract.SchemaTypeObject,
				Required: []string{"name", "age"},
				Properties: map[string]*contract.Schema{
					"name":    {Type: contract.SchemaTypeString, MinLength: ptr(uint64(1))},
					"age":     {Type: contract.SchemaTypeInteger, Min: ptr(1.0)},
					"species": {Type: contract.SchemaTypeString}, // optional
				},
			},
		},
	}}}
	p := &plan.Plan{
		Order:      []string{id},
		Operations: map[string]plan.Operator{id: {Enabled: true}},
	}

	_, err := New(Config{BaseURL: mustParse(t, srv.URL)}).Run(context.Background(), p, c)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !gotContains {
		t.Fatal("the server received no request")
	}
	if gotType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotType)
	}

	var got map[string]any
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", gotBody, err)
	}
	want := map[string]any{"name": "a", "age": float64(1)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

// When Minimal cannot build a body, the operation is skipped: nothing is
// sent, and it is not a finding, because the fault is specter's.
func TestRunSkipsWhenBodyCannotBeBuilt(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	const id = "POST /animals"
	c := &contract.Contract{Operations: []contract.Operation{{
		Method: contract.POST,
		Path:   "/animals",
		Bodies: map[string]*contract.Schema{
			"application/json": {Type: "foo"}, // Minimal returns an error
		},
	}}}
	p := &plan.Plan{
		Order:      []string{id},
		Operations: map[string]plan.Operator{id: {Enabled: true}},
	}

	got, err := New(Config{BaseURL: mustParse(t, srv.URL), Oracles: []oracle.Oracle{oracle.SimpleOracle{}}}).
		Run(context.Background(), p, c)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("server received %d requests, want 0", n)
	}
	if len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

// An operation that requires a body but only declares a media type other than
// JSON is skipped: sending it without a body would only make the API answer
// 4xx, and that is not a finding, because the fault is specter's.
func TestRunSkipsWhenRequiredBodyIsNotJSON(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	const id = "POST /animals"
	c := &contract.Contract{Operations: []contract.Operation{{
		Method:       contract.POST,
		Path:         "/animals",
		BodyRequired: true,
		Bodies: map[string]*contract.Schema{
			"application/x-www-form-urlencoded": {Type: contract.SchemaTypeObject},
		},
	}}}
	p := &plan.Plan{
		Order:      []string{id},
		Operations: map[string]plan.Operator{id: {Enabled: true}},
	}

	got, err := New(Config{BaseURL: mustParse(t, srv.URL), Oracles: []oracle.Oracle{oracle.SimpleOracle{}}}).
		Run(context.Background(), p, c)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("server received %d requests, want 0", n)
	}
	if len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

func ptr[T any](v T) *T { return &v }
