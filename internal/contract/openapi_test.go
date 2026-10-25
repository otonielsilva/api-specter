package contract

import (
	"context"
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
//
//	func findOp(t *testing.T, c *Contract, id string) Operation {
//		t.Helper()
//		for _, op := range c.Operations { if op.ID == id { return op } }
//		t.Fatalf("operation %q not found", id)
//		return Operation{}
//	}

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
	t.Skip("TODO: write me, and make it pass by sorting Operations in the parser")
	// Parse the same file 20 times and compare the list of operation IDs with
	// reflect.DeepEqual. Today this fails randomly because Go map iteration is
	// unordered. Fix in ParseOpenAPI: sort by Path, then Method, before returning.
}
