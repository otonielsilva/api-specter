package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/plan"
)

// pathServer starts a server that answers every POST with the given status and
// body, and every other request with 200. It records "METHOD /escaped/path" of
// each request, in order, and the escaped form is the one on the wire: a value
// with a space shows up as %20.
func pathServer(t *testing.T, status int, body string) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		mu       sync.Mutex
		received []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()

		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(received)
	}
}

// A GET /animals/{id} runs with the value that POST /animals returned, in the
// place the plan says (From). Everything that is not a usable value skips the
// GET, and nothing is sent for it: the request would only prove a bug of
// specter.
func TestRunFillsPathParameter(t *testing.T) {
	integer := &contract.Schema{Type: contract.SchemaTypeInteger}
	str := &contract.Schema{Type: contract.SchemaTypeString}

	const (
		post = "POST /animals"
		get  = "GET /animals/{id}"
	)

	tests := []struct {
		name   string
		status int    // of the POST
		body   string // of the POST
		from   string // where the plan says the id is
		schema *contract.Schema
		want   []string // what the server received, in order
	}{
		// The value reaches the URL.
		{name: "id of a 201 response", status: 201, body: `{"id": 7}`, from: "response.body.id", schema: integer, want: []string{"POST /animals", "GET /animals/7"}},
		{name: "id of a 200 response", status: 200, body: `{"id": 7}`, from: "response.body.id", schema: integer, want: []string{"POST /animals", "GET /animals/7"}},
		{name: "id inside an envelope", status: 201, body: `{"data": {"id": 3}}`, from: "response.body.data.id", schema: integer, want: []string{"POST /animals", "GET /animals/3"}},
		{name: "field with another name than the parameter", status: 201, body: `{"animalId": 5}`, from: "response.body.animalId", schema: integer, want: []string{"POST /animals", "GET /animals/5"}},
		{name: "string id", status: 201, body: `{"id": "abc"}`, from: "response.body.id", schema: str, want: []string{"POST /animals", "GET /animals/abc"}},
		{name: "number given to a string parameter", status: 201, body: `{"id": 12}`, from: "response.body.id", schema: str, want: []string{"POST /animals", "GET /animals/12"}},
		// The value is one segment of the path, escaped on the wire.
		{name: "space is escaped", status: 201, body: `{"id": "a b"}`, from: "response.body.id", schema: str, want: []string{"POST /animals", "GET /animals/a%20b"}},
		{name: "slash does not add a segment", status: 201, body: `{"id": "a/b"}`, from: "response.body.id", schema: str, want: []string{"POST /animals", "GET /animals/a%2Fb"}},
		{name: "question mark does not start a query", status: 201, body: `{"id": "a?b=c"}`, from: "response.body.id", schema: str, want: []string{"POST /animals", "GET /animals/a%3Fb=c"}},

		// A failed producer feeds nothing, even when the body has the field.
		{name: "400 with an id in the body", status: 400, body: `{"id": 1}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "404 with an id in the body", status: 404, body: `{"id": 1}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "500 with an id in the body", status: 500, body: `{"id": 1}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "3xx with an id in the body", status: 302, body: `{"id": 1}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},

		// A successful response without a usable value.
		{name: "field is missing", status: 201, body: `{"name": "rex"}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "field is null", status: 201, body: `{"id": null}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "body is empty", status: 201, body: ``, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "body is not JSON", status: 201, body: `created`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "envelope has no id", status: 201, body: `{"data": {}}`, from: "response.body.data.id", schema: integer, want: []string{"POST /animals"}},
		{name: "unsupported place in the response", status: 201, body: `{"id": 7}`, from: "response.header.Location", schema: integer, want: []string{"POST /animals"}},

		// A value that the contract does not accept for the parameter.
		{name: "fraction for an integer", status: 201, body: `{"id": 1.5}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "string for an integer", status: 201, body: `{"id": "7"}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "integer above 2^53", status: 201, body: `{"id": 9007199254740993}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},
		{name: "object", status: 201, body: `{"id": {"x": 1}}`, from: "response.body.id", schema: integer, want: []string{"POST /animals"}},

		// A value that would leave the resource or call another endpoint.
		{name: "double dot", status: 201, body: `{"id": ".."}`, from: "response.body.id", schema: str, want: []string{"POST /animals"}},
		{name: "dot", status: 201, body: `{"id": "."}`, from: "response.body.id", schema: str, want: []string{"POST /animals"}},
		{name: "empty string", status: 201, body: `{"id": ""}`, from: "response.body.id", schema: str, want: []string{"POST /animals"}},
		{name: "value that looks like a placeholder is plain text", status: 201, body: `{"id": "{id}"}`, from: "response.body.id", schema: str, want: []string{"POST /animals", "GET /animals/%7Bid%7D"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, received := pathServer(t, tt.status, tt.body)

			c := &contract.Contract{Operations: []contract.Operation{
				{Method: contract.POST, Path: "/animals"},
				{
					Method: contract.GET,
					Path:   "/animals/{id}",
					Params: []contract.Param{{In: contract.ParamInPath, Name: "id", Required: true, Schema: tt.schema}},
				},
			}}
			p := &plan.Plan{
				Order: []string{post, get},
				Operations: map[string]plan.Operator{
					post: {Enabled: true},
					get: {Enabled: true, Requires: []plan.Requirement{{
						Param:     "id",
						In:        plan.Path,
						Producers: []plan.Producer{{Op: post, From: tt.from, Match: plan.MatchID}},
					}}},
				},
			}

			if _, err := New(Config{BaseURL: mustParse(t, srv.URL)}).Run(context.Background(), p, c); err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}

			if got := received(); !slices.Equal(got, tt.want) {
				t.Errorf("server received %q, want %q", got, tt.want)
			}
		})
	}
}

// Each path parameter has its own producer, and a path with two of them is
// filled from the two values.
func TestRunFillsTwoPathParameters(t *testing.T) {
	var (
		mu       sync.Mutex
		received []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()

		if r.Method == http.MethodPost {
			// Each collection answers with its own id.
			id := map[string]string{"/zoos": "3", "/animals": "7"}[r.URL.Path]
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": ` + id + `}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	integer := &contract.Schema{Type: contract.SchemaTypeInteger}
	param := func(name string) contract.Param {
		return contract.Param{In: contract.ParamInPath, Name: name, Required: true, Schema: integer}
	}
	requires := func(name, producer string) plan.Requirement {
		return plan.Requirement{
			Param:     name,
			In:        plan.Path,
			Producers: []plan.Producer{{Op: producer, From: "response.body.id", Match: plan.MatchID}},
		}
	}

	const get = "GET /zoos/{zoo}/animals/{id}"
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: contract.POST, Path: "/zoos"},
		{Method: contract.POST, Path: "/animals"},
		{Method: contract.GET, Path: "/zoos/{zoo}/animals/{id}", Params: []contract.Param{param("zoo"), param("id")}},
	}}
	p := &plan.Plan{
		Order: []string{"POST /zoos", "POST /animals", get},
		Operations: map[string]plan.Operator{
			"POST /zoos":    {Enabled: true},
			"POST /animals": {Enabled: true},
			get: {Enabled: true, Requires: []plan.Requirement{
				requires("zoo", "POST /zoos"),
				requires("id", "POST /animals"),
			}},
		},
	}

	if _, err := New(Config{BaseURL: mustParse(t, srv.URL)}).Run(context.Background(), p, c); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	want := []string{"POST /zoos", "POST /animals", "GET /zoos/3/animals/7"}

	mu.Lock()
	defer mu.Unlock()

	if !slices.Equal(received, want) {
		t.Errorf("server received %q, want %q", received, want)
	}
}

// The Pool belongs to one Run. A second Run of the same plan starts without
// the values of the first, so a consumer never reads an id of an earlier run
// whose producer did not answer this time.
func TestRunPoolIsNotSharedBetweenRuns(t *testing.T) {
	status := http.StatusCreated

	var (
		mu       sync.Mutex
		received []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = append(received, r.Method+" "+r.URL.EscapedPath())
		st := status
		mu.Unlock()

		if r.Method == http.MethodPost {
			w.WriteHeader(st)
			_, _ = w.Write([]byte(`{"id": 7}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	const (
		post = "POST /animals"
		get  = "GET /animals/{id}"
	)
	c := &contract.Contract{Operations: []contract.Operation{
		{Method: contract.POST, Path: "/animals"},
		{
			Method: contract.GET,
			Path:   "/animals/{id}",
			Params: []contract.Param{{In: contract.ParamInPath, Name: "id", Required: true, Schema: &contract.Schema{Type: contract.SchemaTypeInteger}}},
		},
	}}
	p := &plan.Plan{
		Order: []string{post, get},
		Operations: map[string]plan.Operator{
			post: {Enabled: true},
			get: {Enabled: true, Requires: []plan.Requirement{{
				Param:     "id",
				In:        plan.Path,
				Producers: []plan.Producer{{Op: post, From: "response.body.id", Match: plan.MatchID}},
			}}},
		},
	}
	e := New(Config{BaseURL: mustParse(t, srv.URL)})

	if _, err := e.Run(context.Background(), p, c); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}

	mu.Lock()
	status = http.StatusInternalServerError
	received = nil
	mu.Unlock()

	if _, err := e.Run(context.Background(), p, c); err != nil {
		t.Fatalf("second Run() error = %v, want nil", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if want := []string{"POST /animals"}; !slices.Equal(received, want) {
		t.Errorf("second run: server received %q, want %q", received, want)
	}
}
