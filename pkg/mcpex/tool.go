package mcpex

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// applySchema validates whether data is valid JSON according to the provided
// schema, after applying schema defaults.
//
// Returns the JSON value augmented with defaults.
func applySchema(data json.RawMessage, resolved *jsonschema.Resolved) (json.RawMessage, error) {
	// TODO: use reflection to create the struct type to unmarshal into.
	// Separate validation from assignment.

	// Use default JSON marshalling for validation.
	//
	// This avoids inconsistent representation due to custom marshallers, such as
	// time.Time (issue #449).
	//
	// Additionally, unmarshalling into a map ensures that the resulting JSON is
	// at least {}, even if data is empty. For example, arguments is technically
	// an optional property of callToolParams, and we still want to apply the
	// defaults in this case.
	//
	// TODO(rfindley): in which cases can resolved be nil?
	if resolved != nil {
		v := make(map[string]any)
		if len(data) > 0 {
			if err := json.Unmarshal(data, &v); err != nil {
				return nil, fmt.Errorf("unmarshaling arguments: %w", err)
			}
		}
		if err := resolved.ApplyDefaults(&v); err != nil {
			return nil, fmt.Errorf("applying schema defaults:\n%w", err)
		}
		if err := resolved.Validate(&v); err != nil {
			return nil, err
		}
		// We must re-marshal with the default values applied.
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshalling with defaults: %v", err)
		}
	}
	return data, nil
}
