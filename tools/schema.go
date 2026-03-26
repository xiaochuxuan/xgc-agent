package tools

import (
	"bytes"
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

// ValidateArgs validates raw JSON args against the given input schema.
// It checks required field presence and field type compatibility.
func ValidateArgs(toolName string, args []byte, schema *Schema) error {
	if schema == nil {
		return nil
	}

	hasRequired := len(schema.Required) > 0

	// reject empty or null args when schema has required fields
	if len(args) == 0 || string(args) == "null" {
		if hasRequired {
			return fmt.Errorf("tool %s: empty args but required fields expected: %v", toolName, schema.Required)
		}
		return nil
	}

	if schema.Type != "object" || len(schema.Properties) == 0 {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return nil // let the main unmarshal handle parse errors
	}

	// check required fields
	for _, field := range schema.Required {
		if _, ok := raw[field]; !ok {
			return fmt.Errorf("tool %s: missing required field %q", toolName, field)
		}
	}

	// check field types against schema
	for name, value := range raw {
		propSchema, ok := schema.Properties[name]
		if !ok || propSchema == nil {
			continue
		}
		if err := checkJSONType(value, propSchema.Type); err != nil {
			return fmt.Errorf("tool %s: field %q: %w", toolName, name, err)
		}
	}

	return nil
}

// checkJSONType checks if a raw JSON value matches the expected schema type.
func checkJSONType(raw json.RawMessage, expectedType string) error {
	if len(raw) == 0 || expectedType == "" {
		return nil
	}

	b := bytes.TrimSpace(raw)
	if len(b) == 0 {
		return nil
	}

	// null is acceptable for any type (represents optional/missing)
	if string(b) == "null" {
		return nil
	}

	first := b[0]

	switch expectedType {
	case "string":
		if first != '"' {
			return fmt.Errorf("expected string, got %s", jsonTokenDesc(first))
		}
	case "integer":
		if first == '"' || first == '{' || first == '[' || first == 't' || first == 'f' {
			return fmt.Errorf("expected integer, got %s", jsonTokenDesc(first))
		}
		if bytes.ContainsAny(b, ".eE") {
			return fmt.Errorf("expected integer, got number with decimal")
		}
	case "number":
		if first == '"' || first == '{' || first == '[' || first == 't' || first == 'f' {
			return fmt.Errorf("expected number, got %s", jsonTokenDesc(first))
		}
	case "boolean":
		if first != 't' && first != 'f' {
			return fmt.Errorf("expected boolean, got %s", jsonTokenDesc(first))
		}
	case "array":
		if first != '[' {
			return fmt.Errorf("expected array, got %s", jsonTokenDesc(first))
		}
	case "object":
		if first != '{' {
			return fmt.Errorf("expected object, got %s", jsonTokenDesc(first))
		}
	}

	return nil
}

// jsonTokenDesc returns a human-readable type name from a JSON token's first byte.
func jsonTokenDesc(first byte) string {
	switch first {
	case '"':
		return "string"
	case '{':
		return "object"
	case '[':
		return "array"
	case 't', 'f':
		return "boolean"
	default:
		return "number"
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
