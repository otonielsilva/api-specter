package contract

import (
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestConvertSchema(t *testing.T) {
	tests := []struct {
		name  string
		build func() *openapi3.Schema
		check func(t *testing.T, got *Schema)
	}{
		{
			name:  "nil input",
			build: func() *openapi3.Schema { return nil },
			check: func(t *testing.T, got *Schema) {
				if got != nil {
					t.Errorf("got %+v, want nil", got)
				}
			},
		},
		{
			name: "string limits, pattern and format",
			build: func() *openapi3.Schema {
				return openapi3.NewStringSchema().
					WithMinLength(1).WithMaxLength(5).
					WithPattern("^[a-z]+$").WithFormat("uuid")
			},
			check: func(t *testing.T, got *Schema) {
				if got.Type != SchemaTypeString {
					t.Errorf("Type = %q, want string", got.Type)
				}
				if got.MinLength == nil || *got.MinLength != 1 {
					t.Errorf("MinLength = %v, want 1", got.MinLength)
				}
				if got.MaxLength == nil || *got.MaxLength != 5 {
					t.Errorf("MaxLength = %v, want 5", got.MaxLength)
				}
				if got.Pattern != "^[a-z]+$" || got.Format != "uuid" {
					t.Errorf("Pattern/Format = %q/%q", got.Pattern, got.Format)
				}
			},
		},
		{
			name:  "minLength zero is unset",
			build: openapi3.NewStringSchema,
			check: func(t *testing.T, got *Schema) {
				if got.MinLength != nil {
					t.Errorf("MinLength = %d, want nil", *got.MinLength)
				}
				if got.MaxLength != nil {
					t.Errorf("MaxLength = %d, want nil", *got.MaxLength)
				}
			},
		},
		{
			name: "integer bounds",
			build: func() *openapi3.Schema {
				return openapi3.NewIntegerSchema().WithMin(0).WithMax(30)
			},
			check: func(t *testing.T, got *Schema) {
				if got.Type != SchemaTypeInteger {
					t.Errorf("Type = %q, want integer", got.Type)
				}
				if got.Min == nil || *got.Min != 0 {
					t.Errorf("Min = %v, want 0", got.Min)
				}
				if got.Max == nil || *got.Max != 30 {
					t.Errorf("Max = %v, want 30", got.Max)
				}
			},
		},
		{
			name: "enum",
			build: func() *openapi3.Schema {
				return openapi3.NewStringSchema().WithEnum("dog", "cat")
			},
			check: func(t *testing.T, got *Schema) {
				if want := []any{"dog", "cat"}; !reflect.DeepEqual(got.Enum, want) {
					t.Errorf("Enum = %v, want %v", got.Enum, want)
				}
			},
		},
		{
			name: "nullable flag (OpenAPI 3.0)",
			build: func() *openapi3.Schema {
				return openapi3.NewStringSchema().WithNullable()
			},
			check: func(t *testing.T, got *Schema) {
				if !got.Nullable || got.Type != SchemaTypeString {
					t.Errorf("got %+v, want nullable string", got)
				}
			},
		},
		{
			name: "type list with null (OpenAPI 3.1)",
			build: func() *openapi3.Schema {
				s := &openapi3.Schema{}
				s.Type = &openapi3.Types{"null", "string"}
				return s
			},
			check: func(t *testing.T, got *Schema) {
				if !got.Nullable || got.Type != SchemaTypeString {
					t.Errorf("got Type=%q Nullable=%v, want string nullable", got.Type, got.Nullable)
				}
			},
		},
		{
			name: "nested object with required",
			build: func() *openapi3.Schema {
				owner := openapi3.NewObjectSchema().
					WithProperty("email", openapi3.NewStringSchema())
				owner.Required = []string{"email"}
				return openapi3.NewObjectSchema().WithProperty("owner", owner)
			},
			check: func(t *testing.T, got *Schema) {
				owner := got.Properties["owner"]
				if owner == nil || owner.Type != SchemaTypeObject {
					t.Fatalf("owner = %+v, want object", owner)
				}
				if !reflect.DeepEqual(owner.Required, []string{"email"}) {
					t.Errorf("owner.Required = %v, want [email]", owner.Required)
				}
				if email := owner.Properties["email"]; email == nil || email.Type != SchemaTypeString {
					t.Errorf("email = %+v, want string", email)
				}
			},
		},
		{
			name: "array items",
			build: func() *openapi3.Schema {
				return openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema())
			},
			check: func(t *testing.T, got *Schema) {
				if got.Type != SchemaTypeArray || got.Items == nil || got.Items.Type != SchemaTypeString {
					t.Errorf("got %+v, want array of string", got)
				}
			},
		},
		{
			name: "allOf merges properties and required without duplicates",
			build: func() *openapi3.Schema {
				base := openapi3.NewObjectSchema().
					WithProperty("name", openapi3.NewStringSchema())
				base.Required = []string{"name"}
				ext := openapi3.NewObjectSchema().
					WithProperty("id", openapi3.NewIntegerSchema())
				ext.Required = []string{"id", "name"}
				return &openapi3.Schema{AllOf: openapi3.SchemaRefs{base.NewRef(), ext.NewRef()}}
			},
			check: func(t *testing.T, got *Schema) {
				if got.Type != SchemaTypeObject {
					t.Errorf("Type = %q, want object inherited from allOf", got.Type)
				}
				if _, ok := got.Properties["name"]; !ok {
					t.Error("missing property name")
				}
				if _, ok := got.Properties["id"]; !ok {
					t.Error("missing property id")
				}
				if want := []string{"name", "id"}; !reflect.DeepEqual(got.Required, want) {
					t.Errorf("Required = %v, want %v", got.Required, want)
				}
			},
		},
		{
			name: "oneOf and anyOf keep every alternative",
			build: func() *openapi3.Schema {
				return &openapi3.Schema{
					OneOf: openapi3.SchemaRefs{
						openapi3.NewStringSchema().NewRef(),
						openapi3.NewIntegerSchema().NewRef(),
					},
					AnyOf: openapi3.SchemaRefs{
						openapi3.NewBoolSchema().NewRef(),
					},
				}
			},
			check: func(t *testing.T, got *Schema) {
				if len(got.OneOf) != 2 || got.OneOf[0].Type != SchemaTypeString || got.OneOf[1].Type != SchemaTypeInteger {
					t.Errorf("OneOf = %+v, want [string integer]", got.OneOf)
				}
				if len(got.AnyOf) != 1 || got.AnyOf[0].Type != SchemaTypeBoolean {
					t.Errorf("AnyOf = %+v, want [boolean]", got.AnyOf)
				}
			},
		},
		{
			name: "self reference is cut off instead of recursing forever",
			build: func() *openapi3.Schema {
				node := openapi3.NewObjectSchema()
				node.Properties = openapi3.Schemas{"next": node.NewRef()}
				return node
			},
			check: func(t *testing.T, got *Schema) {
				if got.Type != SchemaTypeObject {
					t.Fatalf("Type = %q, want object", got.Type)
				}
				if next, ok := got.Properties["next"]; !ok || next != nil {
					t.Errorf("next = %+v (present=%v), want present and nil", next, ok)
				}
			},
		},
		{
			name: "shared schema in sibling properties is not a cycle",
			build: func() *openapi3.Schema {
				shared := openapi3.NewStringSchema().NewRef()
				return openapi3.NewObjectSchema().
					WithProperty("a", shared.Value).
					WithProperty("b", shared.Value)
			},
			check: func(t *testing.T, got *Schema) {
				for _, name := range []string{"a", "b"} {
					if p := got.Properties[name]; p == nil || p.Type != SchemaTypeString {
						t.Errorf("property %s = %+v, want string", name, p)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, convertSchema(tt.build(), nil))
		})
	}
}

func TestConvertParams(t *testing.T) {
	t.Run("keeps location, name and required", func(t *testing.T) {
		got := convertParams(nil, openapi3.Parameters{
			{Value: openapi3.NewQueryParameter("limit").WithRequired(true).WithSchema(openapi3.NewIntegerSchema())},
			{Value: openapi3.NewHeaderParameter("X-Request-Id").WithSchema(openapi3.NewStringSchema().WithFormat("uuid"))},
		})
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].In != ParamInQuery || got[0].Name != "limit" || !got[0].Required {
			t.Errorf("got[0] = %+v, want required query limit", got[0])
		}
		if got[1].In != ParamInHeader || got[1].Schema == nil || got[1].Schema.Format != "uuid" {
			t.Errorf("got[1] = %+v, want header with uuid format", got[1])
		}
	})

	t.Run("operation param replaces path param with same location and name", func(t *testing.T) {
		pathLevel := openapi3.Parameters{{Value: openapi3.NewQueryParameter("q").WithSchema(openapi3.NewStringSchema())}}
		opLevel := openapi3.Parameters{{Value: openapi3.NewQueryParameter("q").WithRequired(true).WithSchema(openapi3.NewStringSchema().WithMaxLength(5))}}

		got := convertParams(pathLevel, opLevel)
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if !got[0].Required || got[0].Schema.MaxLength == nil {
			t.Errorf("got %+v, want the operation's version", got[0])
		}
	})

	t.Run("same name in a different location is a different param", func(t *testing.T) {
		got := convertParams(nil, openapi3.Parameters{
			{Value: openapi3.NewQueryParameter("id")},
			{Value: openapi3.NewHeaderParameter("id")},
		})
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("does not modify the caller's slices", func(t *testing.T) {
		pathLevel := openapi3.Parameters{{Value: openapi3.NewPathParameter("id")}}
		opLevel := openapi3.Parameters{{Value: openapi3.NewQueryParameter("q")}}
		convertParams(pathLevel, opLevel)
		if len(pathLevel) != 1 || len(opLevel) != 1 {
			t.Errorf("inputs changed: %d, %d", len(pathLevel), len(opLevel))
		}
	})

	t.Run("schema taken from content when there is no schema", func(t *testing.T) {
		p := openapi3.NewQueryParameter("filter")
		p.Schema = nil
		p.Content = openapi3.Content{
			"application/json": &openapi3.MediaType{
				Schema: openapi3.NewObjectSchema().
					WithProperty("tag", openapi3.NewStringSchema()).NewRef(),
			},
		}
		got := convertParams(nil, openapi3.Parameters{{Value: p}})
		if len(got) != 1 || got[0].Schema == nil || got[0].Schema.Type != SchemaTypeObject {
			t.Fatalf("got %+v, want object schema from content", got)
		}
		if _, ok := got[0].Schema.Properties["tag"]; !ok {
			t.Error("missing property tag")
		}
	})

	t.Run("nil refs are skipped", func(t *testing.T) {
		got := convertParams(nil, openapi3.Parameters{nil, {Value: nil}, {Value: openapi3.NewQueryParameter("q")}})
		if len(got) != 1 || got[0].Name != "q" {
			t.Errorf("got %+v, want only q", got)
		}
	})
}

// TestConvertSchemaAllOfItems covers a gap in TestConvertSchema/allOf.
func TestConvertSchemaAllOfItems(t *testing.T) {
	t.Skip("TODO: write me")
	// Build an array schema whose items come only from an allOf member:
	//   &openapi3.Schema{AllOf: SchemaRefs{ NewArraySchema().WithItems(NewStringSchema()).NewRef() }}
	// Expect: got.Type == array and got.Items.Type == string (inherited).
	// Second case: the outer schema already has Items; the allOf member must
	// NOT overwrite them.
}

// TestConvertSchemaAlternativesSkipped covers convertAlternatives.
func TestConvertSchemaAlternativesSkipped(t *testing.T) {
	t.Skip("TODO: write me")
	// OneOf containing a nil ref and a ref with Value == nil, plus one valid
	// string schema. Expect: OneOf has exactly 1 element.
	// Another case: every alternative is nil. Expect: OneOf == nil (not an
	// empty non-nil slice), so assert with `got.OneOf != nil`.
	// A oneOf whose member refers back to the parent is cut off by the cycle
	// guard; expect it to be skipped, not to appear as a nil element.
}

func TestConvertBody(t *testing.T) {
	t.Skip("TODO: write me")
	// Build the input in Go, no YAML. Example:
	//   &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().
	//       WithRequired(true).WithJSONSchema(openapi3.NewStringSchema())}
	// Table cases (use t.Run and a struct with name/input/wantBodies/wantRequired):
	//   1. nil ref                      -> nil, false
	//   2. ref with Value == nil        -> nil, false
	//   3. required JSON body           -> {"application/json": string}, true
	//   4. optional body                -> same bodies, false
	//   5. two media types              -> both keys present (check len == 2)
	//   6. media type without a schema  -> key present with a nil value. Use
	//      `v, ok := got["text/plain"]` to tell "nil value" from "missing key".
	//   7. non-JSON media type          -> "application/octet-stream" with
	//      Type string and Format "binary".
}

func TestConvertResponses(t *testing.T) {
	t.Skip("TODO: write me once convertResponses is implemented")
	// Build with openapi3.NewResponses(...) or set values via the Responses
	// map API of kin v0.151.0 (check `go doc github.com/getkin/kin-openapi/openapi3.Responses`).
	// Table cases:
	//   1. nil *Responses              -> nil map, no panic
	//   2. "200" with JSON content     -> Responses["200"].Bodies["application/json"]
	//   3. "204" without content       -> key "204" PRESENT, &Response{} with empty Bodies
	//   4. "default" and "2XX" keys    -> kept exactly as written
	//   5. media type without a schema -> key present with a nil value
	//   6. nil ResponseRef / Value nil -> skipped or empty, but no panic.
	//      Decide which and write the decision into the test name.
}

func TestConvertSchemaDoesNotAliasDocument(t *testing.T) {
	req := make([]string, 1, 4)
	req[0] = "a"
	enum := []any{"a", "b"}
	doc := &openapi3.Schema{
		Required: req,
		Enum:     enum,
		AllOf:    openapi3.SchemaRefs{{Value: &openapi3.Schema{Required: []string{"x"}}}},
	}

	got := convertSchema(doc, nil)

	if got == nil {
		t.Fatal("got nil, want non-nil")
	}
	// The allOf merge appends to Required; it must not write into the
	// spare capacity of the document's own array.
	if extra := req[:cap(req)][1]; extra != "" { //nolint:gosec // cap(req) is 4, so index 1 is in range
		t.Errorf("document backing array was written: %q", extra)
	}

	got.Required[0] = "CHANGED"
	if req[0] != "a" {
		t.Errorf("document Required changed to %q", req[0])
	}

	got.Enum[0] = "CHANGED"
	if enum[0] != "a" {
		t.Errorf("document Enum changed to %v", enum[0])
	}
}
