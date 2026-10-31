package contract

import "cmp"

// Method is an HTTP method, independent of the OpenAPI parsing library.
type Method string

// HTTP methods supported by the model.
const (
	GET     Method = "GET"
	POST    Method = "POST"
	PUT     Method = "PUT"
	PATCH   Method = "PATCH"
	DELETE  Method = "DELETE"
	HEAD    Method = "HEAD"
	OPTIONS Method = "OPTIONS"
	TRACE   Method = "TRACE"
	CONNECT Method = "CONNECT"
)

// methodRank defines the order of methods inside the same path. Methods
// missing from the map rank after all known ones.
var methodRank = map[Method]int{
	POST:    1,
	GET:     2,
	PUT:     3,
	PATCH:   4,
	OPTIONS: 5,
	HEAD:    6,
	DELETE:  7,
	TRACE:   8,
	CONNECT: 9,
}

func rankOf(m Method) int {
	if r, ok := methodRank[m]; ok {
		return r
	}
	return len(methodRank) + 1
}

// compareOperations orders operations by path, then by methodRank, then by
// method name, so two different methods never compare equal.
func compareOperations(a, b Operation) int {
	if c := cmp.Compare(a.Path, b.Path); c != 0 {
		return c
	}
	if c := cmp.Compare(rankOf(a.Method), rankOf(b.Method)); c != 0 {
		return c
	}
	return cmp.Compare(a.Method, b.Method)
}

// Contract is specter's model of an API description.
type Contract struct {
	Title      string
	Version    string
	Operations []Operation
}

// Operation is a single HTTP operation exposed by the API.
type Operation struct {
	ID     string
	Method Method
	Path   string
	// Bodies maps each media type the operation accepts (e.g.
	// "application/json") to the schema of its request body. It is empty
	// when the operation takes no body. A media type declared without a
	// schema is present with a nil value.
	Bodies map[string]*Schema
	// BodyRequired reports whether the request must carry a body.
	BodyRequired bool
	Params       []Param
	// Responses maps the status key exactly as written in the OpenAPI
	// document ("200", "2XX" or "default") to the documented response.
	Responses map[string]*Response
}

// Response is one documented response of an Operation.
type Response struct {
	// Bodies maps each media type the response can have (e.g.
	// "application/json") to the schema of its body. It is empty when the
	// response has no body, e.g. a 204. A media type declared without a
	// schema is present with a nil value.
	Bodies map[string]*Schema
}

// Schema describes the shape of a request or response body.
type Schema struct {
	// Type is the JSON type of the value.
	Type SchemaType
	// Properties maps property names to their schemas. Only used when Type
	// is SchemaTypeObject.
	Properties map[string]*Schema
	// Required lists the names in Properties that must be present. Only used
	// when Type is SchemaTypeObject.
	Required []string
	// Min is the inclusive lower bound of a number or integer. Nil means
	// unbounded.
	Min *float64
	// Max is the inclusive upper bound of a number or integer. Nil means
	// unbounded.
	Max *float64
	// MinLength is the minimum length of a string. Nil means unbounded.
	MinLength *uint64
	// MaxLength is the maximum length of a string. Nil means unbounded.
	MaxLength *uint64
	// Pattern is the regular expression a string must match. Empty means no
	// constraint.
	Pattern string
	// Format is the OpenAPI format hint, e.g. "date-time" or "uuid". Empty
	// means no format.
	Format string
	// Enum lists the only values the schema accepts. Empty means any value of
	// Type is accepted.
	Enum []any
	// Nullable reports whether the value may be null in addition to Type.
	Nullable bool
	// Items is the schema of the elements. Only used when Type is
	// SchemaTypeArray.
	Items *Schema
	// OneOf lists alternatives of which the value must match exactly one.
	// Empty means the schema is not a oneOf.
	OneOf []*Schema
	// AnyOf lists alternatives of which the value must match at least one.
	// Empty means the schema is not an anyOf.
	AnyOf []*Schema
}

// SchemaType is the JSON type of a Schema.
type SchemaType string

// JSON types supported by the model.
const (
	SchemaTypeObject  SchemaType = "object"
	SchemaTypeArray   SchemaType = "array"
	SchemaTypeString  SchemaType = "string"
	SchemaTypeInteger SchemaType = "integer"
	SchemaTypeNumber  SchemaType = "number"
	SchemaTypeBoolean SchemaType = "boolean"
	SchemaTypeNull    SchemaType = "null"
)

// Locations where a Param can appear in a request.
const (
	ParamInPath   ParamIn = "path"
	ParamInQuery  ParamIn = "query"
	ParamInHeader ParamIn = "header"
	ParamInCookie ParamIn = "cookie"
)

// ParamIn is the location of a Param in a request.
type ParamIn string

// Param is an input of an Operation outside the request body.
type Param struct {
	In       ParamIn
	Name     string
	Required bool
	Schema   *Schema
}

// BuildID returns the id of the operation, "METHOD /path", which is also its
// key in a plan.
func (op *Operation) BuildID() string {
	return string(op.Method) + " " + op.Path
}
