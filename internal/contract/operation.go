package contract

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
)

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
	Body   *Schema
	// BodyMediaType is the media type of Body, e.g. "application/json".
	// It is empty when Body is nil.
	BodyMediaType string
	Params        []Param
	// Responses maps the status key exactly as written in the OpenAPI
	// document ("200", "2XX" or "default") to the response body schema. The
	// schema is nil when the response has no body, e.g. a 204.
	Responses map[string]*Schema
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
	MinLength *int
	// MaxLength is the maximum length of a string. Nil means unbounded.
	MaxLength *int
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
