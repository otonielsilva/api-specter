// Package contract loads an OpenAPI document and converts it into specter's
// own model of operations, parameters and schemas.
//
// The rest of specter depends only on that internal model, never on the
// OpenAPI parsing library's types.
package contract
