package contract

import (
	"context"

	"github.com/getkin/kin-openapi/openapi3"
)

// ParseOpenAPI loads the OpenAPI document at fileLocation.
func ParseOpenAPI(ctx context.Context, fileLocation *string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(*fileLocation)
	if err != nil {
		return nil, err
	}
	if err := doc.Validate(ctx); err != nil {
		return nil, err
	}

	return doc, err
}
