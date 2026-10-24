package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// ResponseFormat is a constraint, not a prompt hint. Unsupported engines must
// reject it rather than silently generating unconstrained prose.
type ResponseFormat struct {
	Type       string              `json:"type"`
	JSONSchema *ResponseJSONSchema `json:"json_schema,omitempty"`
}

type ResponseJSONSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
	Schema      map[string]any `json:"schema"`
}

func (f *ResponseFormat) UnmarshalJSON(data []byte) error {
	type wire ResponseFormat
	var value wire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*f = ResponseFormat(value)
	return f.Validate()
}

func (f *ResponseFormat) Constrained() bool { return f != nil && f.Type != "text" }

func (f *ResponseFormat) Validate() error {
	if f == nil {
		return nil
	}
	switch f.Type {
	case "text", "json_object":
		if f.JSONSchema != nil {
			return fmt.Errorf("json_schema requires json_schema response type")
		}
	case "json_schema":
		if f.JSONSchema == nil || f.JSONSchema.Name == "" || len(f.JSONSchema.Name) > 64 || len(f.JSONSchema.Schema) == 0 {
			return fmt.Errorf("response format requires a named JSON schema")
		}
		for _, c := range f.JSONSchema.Name {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
				return fmt.Errorf("invalid response schema name")
			}
		}
		data, err := json.Marshal(f.JSONSchema.Schema)
		if err != nil || len(data) > 64<<10 {
			return fmt.Errorf("response schema exceeds safety limits")
		}
		var validate func(any, int) error
		validate = func(node any, depth int) error {
			if depth > 32 {
				return fmt.Errorf("response schema exceeds nesting limit")
			}
			switch v := node.(type) {
			case map[string]any:
				for k, child := range v {
					if k == "$ref" {
						ref, ok := child.(string)
						if !ok || !strings.HasPrefix(ref, "#") {
							return fmt.Errorf("response schema references must be local")
						}
					}
					if err := validate(child, depth+1); err != nil {
						return err
					}
				}
			case []any:
				for _, child := range v {
					if err := validate(child, depth+1); err != nil {
						return err
					}
				}
			}
			return nil
		}
		return validate(f.JSONSchema.Schema, 0)
	default:
		return &UnsupportedControlError{Control: "response_format.type"}
	}
	return nil
}
