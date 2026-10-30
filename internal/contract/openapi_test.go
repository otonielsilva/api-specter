package contract

import (
	"context"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestParseOpenAPIMergesPathAndOperationParams(t *testing.T) {
	spec := "testdata/params.yaml"

	c, err := ParseOpenAPI(context.Background(), &spec)
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
// ParseOpenAPI in random order, so look them up by ID with a helper:
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

func TestParseOpenAPIPetstoreMetadata(t *testing.T) {
	t.Skip("TODO: write me")
	// Parse testdata/petstore.yaml. Expect Title "Petstore fixture",
	// Version "2.1.0" and len(Operations) == 7.
	// Then check Method and Path of each operation through findOp:
	// listPets GET /pets, createPet POST /pets, getPet GET /pets/{id}, ...
}

func TestParseOpenAPIPetstoreBodies(t *testing.T) {
	t.Skip("TODO: write me")
	// Use findOp. Expectations from the fixture:
	//   getPet / deletePet : Bodies empty, BodyRequired false
	//   createPet          : "application/json" with object schema, BodyRequired true,
	//                        Properties include name/age/owner, Required == [name]
	//   replacePet         : same bodies, BodyRequired false
	//   uploadPhoto        : only "application/octet-stream", Type string, Format binary
	//   search             : target has 2 OneOf alternatives, options has 2 AnyOf
}

func TestParseOpenAPIPetstoreParams(t *testing.T) {
	t.Skip("TODO: write me")
	// listPets params: limit (query, int 1..100), status (query, enum of 3),
	// filter (query, object from content), X-Request-Id (header, format uuid).
	// getPet: the path-level "id" must be inherited (In path, Required, Min 1).
	// Tip: build a map[string]Param by Name first so order does not matter.
}

func TestParseOpenAPIPetstoreAllOf(t *testing.T) {
	t.Skip("TODO: write me")
	// Operation getPet has no body, so reach Pet through the response once
	// convertResponses exists. Until then, add an operation that uses Pet as a
	// request body in the fixture, or test convertSchema directly.
	// Expect Pet to have name, age, owner (from NewPet) plus id and createdAt,
	// and Required containing both "name" and "id".
}

func TestParseOpenAPIErrors(t *testing.T) {
	t.Skip("TODO: write me")
	// Table of bad inputs, each must return a non-nil error and a nil *Contract:
	//   1. path that does not exist
	//   2. a file with invalid YAML (write it with t.TempDir() + os.WriteFile)
	//   3. valid YAML that fails doc.Validate (e.g. a path without responses)
	// Use errors.Is(err, os.ErrNotExist) for case 1.
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
		c, err := ParseOpenAPI(context.Background(), &spec)
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
	c, err := ParseOpenAPI(context.Background(), &spec)
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
