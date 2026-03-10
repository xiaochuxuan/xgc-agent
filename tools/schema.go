package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Schema is a minimal JSON Schema representation for tool parameters.
type Schema struct {
	// Type is the JSON Schema type. E.g., "object", "string", "number", etc.
	// See https://json-schema.org/understanding-json-schema/reference/type.html
	Type        string `json:"type"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	// Properties defines the properties of an object type.
	Properties map[string]*Schema `json:"properties,omitempty"`
	// Items defines the item schema for array types.
	Items *Schema `json:"items,omitempty"`
	// Required lists the required properties for object types.
	Required             []string `json:"required,omitempty"`
	Enum                 []string `json:"enum,omitempty"`
	Default              any      `json:"default,omitempty"`
	AdditionalProperties any      `json:"additionalProperties,omitempty"`
}

// JSONSchema is kept for backward compatibility with older code.
// It is equivalent to Schema.
type JSONSchema = Schema

// ToolSchema describes a tool using JSON Schema for its input/output.
type ToolSchema struct {
	Input  *Schema `json:"input,omitempty"`
	Output *Schema `json:"output,omitempty"`
}

// copyed from trpc-openai-go
// GenerateJSONSchema generates a basic JSON schema from a reflect.Type.
func GenerateJSONSchema(t reflect.Type) *Schema {
	schema := &Schema{Type: "object"}

	// Handle different kinds of types.
	switch t.Kind() {
	case reflect.Struct:
		properties := map[string]*Schema{}
		required := make([]string, 0)

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Skip unexported fields. e.g., lowercase fields.
			if !field.IsExported() {
				continue
			}

			// Get JSON tag or use field name.
			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue // Skip fields marked with json:"-"
			}

			fieldName := field.Name
			isOmitEmpty := false

			if jsonTag != "" {
				// Parse json tag (handle omitempty, etc.)
				if commaIdx := strings.Index(jsonTag, ","); commaIdx != -1 {
					fieldName = jsonTag[:commaIdx]
					isOmitEmpty = strings.Contains(jsonTag[commaIdx:], "omitempty")
				} else {
					fieldName = jsonTag
				}
			}

			// Generate schema for field type.
			fieldSchema := GenerateFieldSchema(field.Type)

			properties[fieldName] = fieldSchema

			// Check if field is required (not a pointer and no omitempty).
			if field.Type.Kind() != reflect.Ptr && !isOmitEmpty {
				required = append(required, fieldName)
			}
		}

		schema.Properties = properties
		if len(required) > 0 {
			schema.Required = required
		}

	case reflect.Ptr:
		// For function tool parameters, we typically use value types
		// So we can just return the element type schema.
		return GenerateFieldSchema(t.Elem())

	default:
		return GenerateFieldSchema(t)
	}

	return schema
}

// GenerateFieldSchema generates schema for a specific field type.
func GenerateFieldSchema(t reflect.Type) *Schema {
	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &Schema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Slice, reflect.Array:
		return &Schema{
			Type:  "array",
			Items: GenerateFieldSchema(t.Elem()),
		}
	case reflect.Map:
		return &Schema{
			Type:                 "object",
			AdditionalProperties: GenerateFieldSchema(t.Elem()),
		}
	case reflect.Pointer:
		// For function tool parameters, we typically use value types
		// So we can just return the element type schema
		return GenerateFieldSchema(t.Elem())
	case reflect.Struct:
		nestedSchema := &Schema{
			Type:       "object",
			Properties: make(map[string]*Schema),
		}

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}

			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}

			fieldName := field.Name
			if jsonTag != "" {
				if commaIdx := strings.Index(jsonTag, ","); commaIdx != -1 {
					fieldName = jsonTag[:commaIdx]
				} else {
					fieldName = jsonTag
				}
			}

			nestedSchema.Properties[fieldName] = GenerateFieldSchema(field.Type)
		}

		return nestedSchema
	default:
		// Default to any type
		return &Schema{Type: "object"}
	}
}

// Hepler function to create Arguments from any input struct.
func NewArgumentsFromStruct(input any) (json.RawMessage, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input struct: %v", err)
	}
	return json.RawMessage(data), nil
}
