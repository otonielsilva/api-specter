package contract

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseOpenAPIMergesPathAndOperationParams(t *testing.T) {
	spec := "testdata/params.yaml"

	c, err := ParseOpenAPI(context.Background(), spec)
	if err != nil {
		t.Fatalf("ParseOpenAPI() error = %v", err)
	}
	if got := len(c.Operations); got != 1 {
		t.Fatalf("len(Operations) = %d, want 1", got)
	}

	params := c.Operations[0].Params
	if got := len(params); got != 2 {
		t.Fatalf("len(Params) = %d, want 2 (id and q, q not duplicated)", got)
	}

	id := params[0]
	if id.Name != "id" || id.In != ParamInPath || !id.Required {
		t.Errorf("params[0] = %+v, want required path param id", id)
	}
	if id.Schema == nil || id.Schema.Type != SchemaTypeInteger {
		t.Fatalf("id schema = %+v, want integer", id.Schema)
	}
	if id.Schema.Min == nil || *id.Schema.Min != 1 {
		t.Errorf("id Min = %v, want 1", id.Schema.Min)
	}

	// The operation's own q must replace the one shared by the path.
	q := params[1]
	if q.Name != "q" || q.In != ParamInQuery {
		t.Fatalf("params[1] = %+v, want query param q", q)
	}
	if !q.Required {
		t.Errorf("q.Required = false, want true (operation overrides path)")
	}
	if q.Schema == nil || q.Schema.MaxLength == nil || *q.Schema.MaxLength != 5 {
		t.Errorf("q schema = %+v, want MaxLength 5", q.Schema)
	}
}

// The tests below use testdata/petstore.yaml. Operations come out of
// ParseOpenAPI sorted by path and method, but tests should not depend on the
// position, so look them up by ID with a helper:
func findOp(t *testing.T, c *Contract, id string) Operation {
	t.Helper()
	for _, op := range c.Operations {
		if op.ID == id {
			return op
		}
	}
	t.Fatalf("operation %q not found", id)
	return Operation{}
}

func TestParseOpenAPIPetstoreAllOf(t *testing.T) {
	c, err := ParseOpenAPI(context.Background(), "testdata/petstore.yaml")
	if err != nil {
		t.Fatalf("ParseOpenAPI() error = %v", err)
	}

	resp := findOp(t, c, "getPet").Responses["200"]
	if resp == nil {
		t.Fatal("getPet has no 200 response")
	}
	pet := resp.Bodies["application/json"]
	if pet == nil {
		t.Fatal("getPet 200 has no application/json schema")
	}

	// name, age and owner come from NewPet; id and createdAt from the inline half.
	for _, name := range []string{"name", "age", "owner", "id", "createdAt"} {
		if pet.Properties[name] == nil {
			t.Errorf("Pet is missing property %q", name)
		}
	}
	for _, name := range []string{"name", "id"} {
		if !slices.Contains(pet.Required, name) {
			t.Errorf("Pet.Required = %v, want it to contain %q", pet.Required, name)
		}
	}
}

func TestParseOpenAPIErrors(t *testing.T) {
	dir := t.TempDir()

	badYAML := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badYAML, []byte("openapi: [unclosed\n  : :"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Parses fine but fails doc.Validate: the operation has no responses.
	invalid := filepath.Join(dir, "invalid.yaml")
	const noResponses = `openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  /x:
    get:
      operationId: x
`
	if err := os.WriteFile(invalid, []byte(noResponses), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr error // optional: the error chain must contain it
	}{
		{"missing file", filepath.Join(dir, "nope.yaml"), os.ErrNotExist},
		{"invalid yaml", badYAML, nil},
		{"fails validation", invalid, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseOpenAPI(context.Background(), tt.path)
			if err == nil {
				t.Fatal("error = nil, want an error")
			}
			if c != nil {
				t.Errorf("contract = %+v, want nil", c)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want it to wrap %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseOpenAPIDeterministicOrder(t *testing.T) {
	spec := "testdata/petstore.yaml"

	want := []string{
		"POST /pets",
		"GET /pets",
		"GET /pets/{id}",
		"PUT /pets/{id}",
		"DELETE /pets/{id}",
		"POST /pets/{id}/photo",
		"POST /search",
	}

	for i := range 500 {
		c, err := ParseOpenAPI(context.Background(), spec)
		if err != nil {
			t.Fatalf("ParseOpenAPI() error = %v", err)
		}
		if len(c.Operations) != len(want) {
			t.Fatalf("len(Operations) = %d, want %d", len(c.Operations), len(want))
		}
		for j := range c.Operations {
			key := string(c.Operations[j].Method) + " " + c.Operations[j].Path
			if key != want[j] {
				t.Errorf("Execution[%d] len(Operations) = %d, want %d", i, len(c.Operations), len(want))
			}
		}
	}
}

func TestSortOperations(t *testing.T) {
	op := func(m Method, path string) Operation {
		return Operation{Method: m, Path: path}
	}

	tests := []struct {
		name string
		in   []Operation
		want []string
	}{
		{
			name: "orders by path first",
			in:   []Operation{op(GET, "/b"), op(DELETE, "/a"), op(POST, "/c")},
			want: []string{"DELETE /a", "GET /b", "POST /c"},
		},
		{
			name: "known methods follow methodRank",
			in: []Operation{
				op(DELETE, "/x"), op(HEAD, "/x"), op(OPTIONS, "/x"), op(PATCH, "/x"),
				op(PUT, "/x"), op(GET, "/x"), op(POST, "/x"),
			},
			want: []string{
				"POST /x", "GET /x", "PUT /x", "PATCH /x",
				"OPTIONS /x", "HEAD /x", "DELETE /x",
			},
		},
		{
			name: "TRACE and CONNECT rank after the common methods",
			in:   []Operation{op(CONNECT, "/x"), op(TRACE, "/x"), op(DELETE, "/x")},
			want: []string{"DELETE /x", "TRACE /x", "CONNECT /x"},
		},
		{
			name: "unknown method goes after every known one",
			in:   []Operation{op("DUMMY", "/x"), op(CONNECT, "/x"), op(POST, "/x")},
			want: []string{"POST /x", "CONNECT /x", "DUMMY /x"},
		},
		{
			name: "unknown methods are ordered by name, not left tied",
			in:   []Operation{op("ZED", "/x"), op("ALPHA", "/x"), op("MID", "/x")},
			want: []string{"ALPHA /x", "MID /x", "ZED /x"},
		},
		{
			name: "path wins over method rank",
			in:   []Operation{op("DUMMY", "/a"), op(POST, "/b")},
			want: []string{"DUMMY /a", "POST /b"},
		},
		{
			name: "prefix path comes before the longer one",
			in:   []Operation{op(GET, "/pets/{id}/photo"), op(GET, "/pets/{id}"), op(GET, "/pets")},
			want: []string{"GET /pets", "GET /pets/{id}", "GET /pets/{id}/photo"},
		},
		{
			name: "empty",
			in:   nil,
			want: []string{},
		},
	}

	// A fixed seed keeps the shuffles reproducible.
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a seeded PRNG keeps the shuffles reproducible

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range 50 {
				ops := slices.Clone(tt.in)
				rng.Shuffle(len(ops), func(a, b int) { ops[a], ops[b] = ops[b], ops[a] })

				slices.SortFunc(ops, compareOperations)

				got := make([]string, 0, len(ops))
				for _, o := range ops {
					got = append(got, string(o.Method)+" "+o.Path)
				}
				if !slices.Equal(got, tt.want) {
					t.Fatalf("shuffle %d: got %v, want %v", i, got, tt.want)
				}
			}
		})
	}
}

func TestParseOpenAPIRecursiveSchemaSharesPointer(t *testing.T) {
	spec := "testdata/cycle.yaml"
	c, err := ParseOpenAPI(context.Background(), spec)
	if err != nil {
		t.Fatalf("ParseOpenAPI: %v", err)
	}

	op := findOp(t, c, "createIngredient")

	ing := op.Bodies["application/json"]
	if ing == nil {
		t.Fatal("body is nil")
	}

	// Direct cycle: Ingredient -> sub_ingredients -> Items -> Ingredient.
	sub := ing.Properties["sub_ingredients"]
	if sub == nil {
		t.Fatal("sub_ingredients is nil")
	}
	if sub.Items != ing {
		t.Errorf("sub_ingredients.Items = %p, want %p (Ingredient)", sub.Items, ing)
	}

	// Mutual cycle: Ingredient -> origin -> main_product -> Ingredient.
	supplier := ing.Properties["origin"]
	if supplier == nil {
		t.Fatal("origin is nil")
	}
	if got := supplier.Properties["main_product"]; got != ing {
		t.Errorf("origin.main_product = %p, want %p (Ingredient)", got, ing)
	}
}

// One memo covers the whole document, so the Ingredient of the request body
// and the one of the 201 response are the same *Schema.
func TestParseOpenAPISharesSchemaAcrossRequestAndResponse(t *testing.T) {
	spec := "testdata/cycle.yaml"
	c, err := ParseOpenAPI(context.Background(), spec)
	if err != nil {
		t.Fatalf("ParseOpenAPI: %v", err)
	}

	op := findOp(t, c, "createIngredient")
	req := op.Bodies["application/json"]
	resp := op.Responses["201"].Bodies["application/json"]
	if req == nil || resp == nil {
		t.Fatalf("request = %p, response = %p, want both set", req, resp)
	}
	if req != resp {
		t.Errorf("request = %p, response = %p, want the same pointer", req, resp)
	}
}
