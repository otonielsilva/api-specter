package contract

import (
	"context"
	"slices"

	"github.com/getkin/kin-openapi/openapi3"
)

// ParseOpenAPI loads the OpenAPI document at fileLocation.
func ParseOpenAPI(ctx context.Context, fileLocation *string) (*Contract, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(*fileLocation)
	if err != nil {
		return nil, err
	}
	if err := doc.Validate(ctx); err != nil {
		return nil, err
	}

	// One memo for the whole document: a schema used by several bodies,
	// parameters and responses converts to a single *Schema, so request and
	// response share the same pointer.
	memo := make(map[*openapi3.Schema]*Schema)

	contract := &Contract{
		Title:      doc.Info.Title,
		Version:    doc.Info.Version,
		Operations: make([]Operation, 0),
	}

	for path, pathItem := range doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			requestBodies, required := convertBody(op.RequestBody, memo)

			operation := Operation{
				ID:           op.OperationID,
				Method:       Method(method),
				Path:         path,
				Params:       convertParams(pathItem.Parameters, op.Parameters, memo),
				Bodies:       requestBodies,
				BodyRequired: required,
				Responses:    convertResponses(op.Responses, memo),
			}

			contract.Operations = append(contract.Operations, operation)

		}
	}

	// Sort so the result does not depend on map iteration order.
	slices.SortFunc(contract.Operations, compareOperations)

	return contract, nil
}

// convertContent converts a media type map into schemas. A media type declared
// without a schema is kept with a nil value. The result is never nil. memo is
// the conversion memo shared by the caller, see convertSchema.
func convertContent(content openapi3.Content, memo map[*openapi3.Schema]*Schema) map[string]*Schema {
	bodies := make(map[string]*Schema, len(content))
	for mediaType, mt := range content {
		var schema *Schema
		if mt != nil && mt.Schema != nil {
			schema = convertSchema(mt.Schema.Value, memo)
		}
		bodies[mediaType] = schema
	}
	return bodies
}

func convertBody(body *openapi3.RequestBodyRef, memo map[*openapi3.Schema]*Schema) (map[string]*Schema, bool) {
	if body == nil || body.Value == nil {
		return nil, false
	}

	return convertContent(body.Value.Content, memo), body.Value.Required
}

// convertParams merges the parameters shared by every operation of a path
// with the operation's own. A parameter is identified by its location and
// name, and the operation's version replaces the path's, as the OpenAPI spec
// requires. The order of first appearance is kept so the result is stable.
func convertParams(pathParams, opParams openapi3.Parameters, memo map[*openapi3.Schema]*Schema) []Param {
	type key struct {
		in   ParamIn
		name string
	}

	params := make([]Param, 0, len(pathParams)+len(opParams))
	index := make(map[key]int)

	for _, ref := range append(append(openapi3.Parameters{}, pathParams...), opParams...) {
		if ref == nil || ref.Value == nil {
			continue
		}
		p := ref.Value

		param := Param{
			In:       ParamIn(p.In),
			Name:     p.Name,
			Required: p.Required,
		}

		switch {
		case p.Schema != nil:
			param.Schema = convertSchema(p.Schema.Value, memo)
		case len(p.Content) > 0:
			// The spec requires exactly one entry in a parameter's content,
			// so this loop runs once.
			for _, mt := range p.Content {
				if mt != nil && mt.Schema != nil {
					param.Schema = convertSchema(mt.Schema.Value, memo)
				}
			}
		}

		k := key{in: param.In, name: param.Name}
		if i, ok := index[k]; ok {
			params[i] = param
			continue
		}
		index[k] = len(params)
		params = append(params, param)
	}
	return params
}

// convertSchema converts a kin-openapi schema into specter's Schema. memo
// maps each kin schema already converted to its result, so a schema that
// refers to itself comes back as a pointer to the same *Schema: the cycle
// stays in the graph instead of being cut off. A nil memo starts a fresh one.
func convertSchema(s *openapi3.Schema, memo map[*openapi3.Schema]*Schema) *Schema {
	if s == nil {
		return nil
	}
	if memo == nil {
		memo = make(map[*openapi3.Schema]*Schema)
	}
	if got, ok := memo[s]; ok {
		return got
	}

	out := &Schema{
		Min:      s.Min,
		Max:      s.Max,
		Pattern:  s.Pattern,
		Format:   s.Format,
		Enum:     slices.Clone(s.Enum),
		Nullable: s.Nullable,
		Required: slices.Clone(s.Required),
	}
	// Register before recursing: a reference back to s finds out, which is
	// still being filled in, and that closes the cycle.
	memo[s] = out

	// OpenAPI 3.1 allows several types, e.g. ["string", "null"]. Keep the
	// first real type and fold "null" into Nullable.
	if s.Type != nil {
		for _, t := range s.Type.Slice() {
			if t == "null" {
				out.Nullable = true
				continue
			}
			if out.Type == "" {
				out.Type = SchemaType(t)
			}
		}
	}

	// MinLength is a plain uint64 in kin, so 0 and "unset" are the same.
	if s.MinLength > 0 {
		v := s.MinLength
		out.MinLength = &v
	}
	if s.MaxLength != nil {
		v := *s.MaxLength
		out.MaxLength = &v
	}

	if len(s.Properties) > 0 {
		out.Properties = make(map[string]*Schema, len(s.Properties))
		for name, ref := range s.Properties {
			if ref == nil {
				continue
			}
			out.Properties[name] = convertSchema(ref.Value, memo)
		}
	}
	if s.Items != nil {
		out.Items = convertSchema(s.Items.Value, memo)
	}

	out.OneOf = convertAlternatives(s.OneOf, memo)
	out.AnyOf = convertAlternatives(s.AnyOf, memo)

	// allOf means the value satisfies every sub-schema, so merging their
	// properties and required names is a faithful translation.
	//
	// Known limit: if a sub-schema refers back to an ancestor still being
	// built (a cycle through allOf), sub is incomplete when it is merged here,
	// so the merge copies only what exists so far.
	for _, ref := range s.AllOf {
		if ref == nil {
			continue
		}
		sub := convertSchema(ref.Value, memo)
		if sub == nil {
			continue
		}
		if out.Type == "" {
			out.Type = sub.Type
		}
		if len(sub.Properties) > 0 && out.Properties == nil {
			out.Properties = make(map[string]*Schema, len(sub.Properties))
		}
		for name, prop := range sub.Properties {
			if _, exists := out.Properties[name]; !exists {
				out.Properties[name] = prop
			}
		}
		for _, name := range sub.Required {
			if !slices.Contains(out.Required, name) {
				out.Required = append(out.Required, name)
			}
		}
		if out.Items == nil {
			out.Items = sub.Items
		}
	}
	return out
}

// convertAlternatives converts the sub-schemas of a oneOf or anyOf. Entries
// that cannot be converted (a nil ref or a ref without a value) are skipped,
// and the result is nil when there is nothing left. An alternative that refers
// back to an ancestor is kept, as a pointer to that ancestor.
func convertAlternatives(refs openapi3.SchemaRefs, memo map[*openapi3.Schema]*Schema) []*Schema {
	var alts []*Schema
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		if alt := convertSchema(ref.Value, memo); alt != nil {
			alts = append(alts, alt)
		}
	}
	return alts
}

// convertResponses converts the responses of an operation, keyed by status
// exactly as written in the document. A response without content is kept with
// empty Bodies; a nil ref or a ref without a value is skipped.
func convertResponses(r *openapi3.Responses, memo map[*openapi3.Schema]*Schema) map[string]*Response {
	if r == nil {
		return nil
	}

	responses := make(map[string]*Response, r.Len())
	for status, ref := range r.Map() {
		if ref == nil || ref.Value == nil {
			continue
		}
		responses[status] = &Response{Bodies: convertContent(ref.Value.Content, memo)}
	}
	return responses
}
