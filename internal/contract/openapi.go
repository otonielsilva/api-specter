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

	contract := &Contract{
		Title:      doc.Info.Title,
		Version:    doc.Info.Version,
		Operations: make([]Operation, 0),
	}

	for path, pathItem := range doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			requestBodies, required := convertBody(op.RequestBody)

			operation := Operation{
				ID:           op.OperationID,
				Method:       Method(method),
				Path:         path,
				Params:       convertParams(pathItem.Parameters, op.Parameters),
				Bodies:       requestBodies,
				BodyRequired: required,
				Responses:    convertResponses(op.Responses),
			}

			contract.Operations = append(contract.Operations, operation)

		}
	}

	// Sort so the result does not depend on map iteration order.
	slices.SortFunc(contract.Operations, compareOperations)

	return contract, nil
}

func convertBody(body *openapi3.RequestBodyRef) (map[string]*Schema, bool) {
	if body == nil || body.Value == nil {
		return nil, false
	}

	bodies := make(map[string]*Schema, len(body.Value.Content))
	for mediaType, mt := range body.Value.Content {
		// A media type declared without a schema is kept with a nil value.
		var schema *Schema
		if mt != nil && mt.Schema != nil {
			schema = convertSchema(mt.Schema.Value, nil)
		}
		bodies[mediaType] = schema
	}
	return bodies, body.Value.Required
}

// convertParams merges the parameters shared by every operation of a path
// with the operation's own. A parameter is identified by its location and
// name, and the operation's version replaces the path's, as the OpenAPI spec
// requires. The order of first appearance is kept so the result is stable.
func convertParams(pathParams, opParams openapi3.Parameters) []Param {
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
			param.Schema = convertSchema(p.Schema.Value, nil)
		case len(p.Content) > 0:
			// The spec requires exactly one entry in a parameter's content,
			// so this loop runs once.
			for _, mt := range p.Content {
				if mt != nil && mt.Schema != nil {
					param.Schema = convertSchema(mt.Schema.Value, nil)
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

// convertSchema converts a kin-openapi schema into specter's Schema. seen
// holds the schemas on the current recursion path, so a schema that refers
// to itself is cut off instead of recursing forever.
func convertSchema(s *openapi3.Schema, seen map[*openapi3.Schema]bool) *Schema {
	if s == nil || seen[s] {
		return nil
	}
	// Copy-on-write: each branch of the recursion gets its own set, so two
	// sibling properties may share a schema without being seen as a cycle.
	next := make(map[*openapi3.Schema]bool, len(seen)+1)
	for k := range seen {
		next[k] = true
	}
	next[s] = true

	out := &Schema{
		Min:      s.Min,
		Max:      s.Max,
		Pattern:  s.Pattern,
		Format:   s.Format,
		Enum:     s.Enum,
		Nullable: s.Nullable,
		Required: s.Required,
	}

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
			out.Properties[name] = convertSchema(ref.Value, next)
		}
	}
	if s.Items != nil {
		out.Items = convertSchema(s.Items.Value, next)
	}

	out.OneOf = convertAlternatives(s.OneOf, next)
	out.AnyOf = convertAlternatives(s.AnyOf, next)

	// allOf means the value satisfies every sub-schema, so merging their
	// properties and required names is a faithful translation.
	for _, ref := range s.AllOf {
		if ref == nil {
			continue
		}
		sub := convertSchema(ref.Value, next)
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
// that cannot be converted (nil, or cut off as a cycle) are skipped, and the
// result is nil when there is nothing left.
func convertAlternatives(refs openapi3.SchemaRefs, seen map[*openapi3.Schema]bool) []*Schema {
	var alts []*Schema
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		if alt := convertSchema(ref.Value, seen); alt != nil {
			alts = append(alts, alt)
		}
	}
	return alts
}

// convertResponses converts the responses of an operation, keyed by status
// exactly as written in the document.
func convertResponses(_ *openapi3.Responses) map[string]*Response {
	// TODO: implement.
	return nil
}
