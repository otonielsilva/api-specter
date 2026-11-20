package generator

import (
	"reflect"
	"testing"

	"github.com/otonielsilva/api-specter/internal/contract"
)

func TestMinimal(t *testing.T) {
	tests := []struct {
		name    string
		schema  *contract.Schema
		want    any
		wantErr bool
	}{
		{name: "nil schema is an error", schema: nil, wantErr: true},
		{name: "string is empty", schema: &contract.Schema{Type: contract.SchemaTypeString}, want: ""},
		{name: "integer is zero", schema: &contract.Schema{Type: contract.SchemaTypeInteger}, want: int64(0)},
		{name: "number is zero", schema: &contract.Schema{Type: contract.SchemaTypeNumber}, want: float64(0)},
		{name: "boolean is false", schema: &contract.Schema{Type: contract.SchemaTypeBoolean}, want: false},
		{name: "array is empty, not nil", schema: &contract.Schema{Type: contract.SchemaTypeArray}, want: []any{}},
		{name: "unknown type is an error", schema: &contract.Schema{Type: "foo"}, wantErr: true},
		{
			name: "object has only the required properties",
			schema: &contract.Schema{
				Type:     contract.SchemaTypeObject,
				Required: []string{"name", "age"},
				Properties: map[string]*contract.Schema{
					"name":    {Type: contract.SchemaTypeString},
					"age":     {Type: contract.SchemaTypeInteger},
					"species": {Type: contract.SchemaTypeString}, // should not appear in the minimum output
				},
			},
			want: map[string]any{"name": "", "age": int64(0)},
		},
		{
			name: "object nests objects",
			schema: &contract.Schema{
				Type:     contract.SchemaTypeObject,
				Required: []string{"owner"},
				Properties: map[string]*contract.Schema{
					"owner": {
						Type: contract.SchemaTypeObject, Required: []string{"id"},
						Properties: map[string]*contract.Schema{"id": {Type: contract.SchemaTypeInteger}},
					},
				},
			},
			want: map[string]any{"owner": map[string]any{"id": int64(0)}},
		},
		{
			name: "required property missing from properties is an error",
			schema: &contract.Schema{
				Type:     contract.SchemaTypeObject,
				Required: []string{"ghost"},
			},
			wantErr: true,
		},
		{
			name: "error in a required property carries up",
			schema: &contract.Schema{
				Type:       contract.SchemaTypeObject,
				Required:   []string{"x"},
				Properties: map[string]*contract.Schema{"x": {Type: "foo"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Minimal(tt.schema)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Minimal() = %#v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Minimal() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Minimal() = %#v (%T), want %#v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}
