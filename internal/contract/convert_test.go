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
	t.Run("items inherited from allOf member", func(t *testing.T) {
		in := &openapi3.Schema{AllOf: openapi3.SchemaRefs{
			openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema()).NewRef(),
		}}

		got := convertSchema(in, nil)

		if got.Type != SchemaTypeArray {
			t.Errorf("Type = %q, want array", got.Type)
		}
		if got.Items == nil || got.Items.Type != SchemaTypeString {
			t.Errorf("Items = %+v, want string", got.Items)
		}
	})

	t.Run("outer items are not overwritten by allOf member", func(t *testing.T) {
		in := openapi3.NewArraySchema().WithItems(openapi3.NewIntegerSchema())
		in.AllOf = openapi3.SchemaRefs{
			openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema()).NewRef(),
		}

		got := convertSchema(in, nil)

		if got.Items == nil || got.Items.Type != SchemaTypeInteger {
			t.Errorf("Items = %+v, want the outer integer items", got.Items)
		}
	})
}

// TestConvertSchemaAlternativesSkipped covers convertAlternatives.
func TestConvertSchemaAlternativesSkipped(t *testing.T) {
	t.Run("nil ref and nil value are skipped", func(t *testing.T) {
		in := &openapi3.Schema{OneOf: openapi3.SchemaRefs{
			nil,
			{Value: nil},
			openapi3.NewStringSchema().NewRef(),
		}}

		got := convertSchema(in, nil)

		if len(got.OneOf) != 1 || got.OneOf[0].Type != SchemaTypeString {
			t.Errorf("OneOf = %+v, want exactly [string]", got.OneOf)
		}
	})

	t.Run("all alternatives nil gives a nil slice", func(t *testing.T) {
		in := &openapi3.Schema{
			OneOf: openapi3.SchemaRefs{nil, {Value: nil}},
			AnyOf: openapi3.SchemaRefs{nil},
		}

		got := convertSchema(in, nil)

		if got.OneOf != nil {
			t.Errorf("OneOf = %#v, want nil", got.OneOf)
		}
		if got.AnyOf != nil {
			t.Errorf("AnyOf = %#v, want nil", got.AnyOf)
		}
	})

	t.Run("alternative pointing back to the parent is skipped", func(t *testing.T) {
		parent := &openapi3.Schema{}
		parent.OneOf = openapi3.SchemaRefs{parent.NewRef(), openapi3.NewStringSchema().NewRef()}

		got := convertSchema(parent, nil)

		if len(got.OneOf) != 1 || got.OneOf[0] == nil || got.OneOf[0].Type != SchemaTypeString {
			t.Errorf("OneOf = %+v, want only the string alternative", got.OneOf)
		}
	})
}

func TestConvertBody(t *testing.T) {
	tests := []struct {
		name       string
		input      *openapi3.RequestBodyRef
		wantBodies map[string]*Schema
		required   bool
	}{
		{
			name:       "nil ref",
			input:      nil,
			wantBodies: nil,
			required:   false,
		},
		{
			name: "String Schema",
			input: &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().
				WithRequired(true).WithJSONSchema(openapi3.NewStringSchema())},
			wantBodies: map[string]*Schema{"application/json": {Type: SchemaTypeString}},
			required:   true,
		},
		{
			// The ref exists but points at nothing, same outcome as a nil ref.
			name:       "ref with nil value",
			input:      &openapi3.RequestBodyRef{Value: nil},
			wantBodies: nil,
			required:   false,
		},
		{
			name: "optional body keeps bodies and is not required",
			input: &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().
				WithRequired(false).WithJSONSchema(openapi3.NewStringSchema())},
			wantBodies: map[string]*Schema{"application/json": {Type: SchemaTypeString}},
			required:   false,
		},
		{
			// Different schemas per key prove each media type got its own
			// schema, not just that both keys exist.
			name: "two media types",
			input: &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().
				WithContent(openapi3.Content{
					"application/json": &openapi3.MediaType{Schema: openapi3.NewStringSchema().NewRef()},
					"text/plain":       &openapi3.MediaType{Schema: openapi3.NewIntegerSchema().NewRef()},
				})},
			wantBodies: map[string]*Schema{
				"application/json": {Type: SchemaTypeString},
				"text/plain":       {Type: SchemaTypeInteger},
			},
			required: false,
		},
		{
			// DeepEqual tells a present key holding nil apart from a missing
			// key, so no `v, ok :=` lookup is needed here.
			name: "media type without a schema keeps the key with a nil value",
			input: &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().
				WithContent(openapi3.Content{"text/plain": &openapi3.MediaType{}})},
			wantBodies: map[string]*Schema{"text/plain": nil},
			required:   false,
		},
		{
			// convertBody does not invent a schema: it passes through the
			// string/binary one declared in the document.
			name: "binary octet-stream",
			input: &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().
				WithContent(openapi3.Content{
					"application/octet-stream": &openapi3.MediaType{
						Schema: openapi3.NewStringSchema().WithFormat("binary").NewRef(),
					},
				})},
			wantBodies: map[string]*Schema{
				"application/octet-stream": {Type: SchemaTypeString, Format: "binary"},
			},
			required: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, converted := convertBody(tt.input)
			if converted != tt.required {
				t.Errorf("required = %v, want %v", converted, tt.required)
			}
			// DeepEqual also catches extra keys and nil vs empty maps.
			if !reflect.DeepEqual(body, tt.wantBodies) {
				t.Errorf("bodies = %+v, want %+v", body, tt.wantBodies)
			}
		})
	}
}

func TestConvertResponses(t *testing.T) {
	jsonOf := func(s *openapi3.Schema) openapi3.Content {
		return openapi3.Content{"application/json": &openapi3.MediaType{Schema: s.NewRef()}}
	}

	tests := []struct {
		name  string
		input *openapi3.Responses
		want  map[string]*Response
	}{
		{
			name:  "nil responses",
			input: nil,
			want:  nil,
		},
		{
			name: "JSON body under 200",
			input: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{Value: &openapi3.Response{
					Content: jsonOf(openapi3.NewStringSchema()),
				}}),
			),
			want: map[string]*Response{
				"200": {Bodies: map[string]*Schema{"application/json": {Type: SchemaTypeString}}},
			},
		},
		{
			// A 204 is documented information, so the key stays; Bodies is an
			// empty non-nil map, not nil.
			name: "204 without content is present with empty bodies",
			input: openapi3.NewResponses(
				openapi3.WithStatus(204, &openapi3.ResponseRef{Value: &openapi3.Response{}}),
			),
			want: map[string]*Response{
				"204": {Bodies: map[string]*Schema{}},
			},
		},
		{
			name: "default and 2XX keys are kept as written",
			input: openapi3.NewResponses(
				openapi3.WithName("2XX", &openapi3.Response{Content: jsonOf(openapi3.NewIntegerSchema())}),
				openapi3.WithName("default", &openapi3.Response{Content: jsonOf(openapi3.NewBoolSchema())}),
			),
			want: map[string]*Response{
				"2XX":     {Bodies: map[string]*Schema{"application/json": {Type: SchemaTypeInteger}}},
				"default": {Bodies: map[string]*Schema{"application/json": {Type: SchemaTypeBoolean}}},
			},
		},
		{
			name: "media type without a schema keeps the key with a nil value",
			input: openapi3.NewResponses(
				openapi3.WithName("200", &openapi3.Response{
					Content: openapi3.Content{"text/plain": &openapi3.MediaType{}},
				}),
			),
			want: map[string]*Response{
				"200": {Bodies: map[string]*Schema{"text/plain": nil}},
			},
		},
		{
			name: "nil ref and ref with nil value are skipped",
			input: func() *openapi3.Responses {
				r := openapi3.NewResponses()
				r.Set("404", nil)
				r.Set("500", &openapi3.ResponseRef{Value: nil})
				r.Set("200", &openapi3.ResponseRef{Value: &openapi3.Response{}})
				return r
			}(),
			want: map[string]*Response{
				"200": {Bodies: map[string]*Schema{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertResponses(tt.input)
			// DeepEqual tells nil from empty maps, which matters for both the
			// outer map and each Bodies.
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("responses = %+v, want %+v", got, tt.want)
			}
		})
	}
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
