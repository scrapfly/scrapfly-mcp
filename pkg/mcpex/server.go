package mcpex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func toolForErr[In, Out any](t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) (*mcp.Tool, mcp.ToolHandler, error) {
	tt := *t

	// Special handling for an "any" input: treat as an empty object.
	if reflect.TypeFor[In]() == reflect.TypeFor[any]() && t.InputSchema == nil {
		tt.InputSchema = &jsonschema.Schema{Type: "object"}
	}

	var inputResolved *jsonschema.Resolved
	if _, err := setSchema[In](&tt.InputSchema, &inputResolved); err != nil {
		return nil, nil, fmt.Errorf("input schema: %w", err)
	}

	// Handling for zero values:
	//
	// If Out is a pointer type and we've derived the output schema from its
	// element type, use the zero value of its element type in place of a typed
	// nil.
	var (
		elemZero       any // only non-nil if Out is a pointer type
		outputResolved *jsonschema.Resolved
	)
	if t.OutputSchema != nil || reflect.TypeFor[Out]() != reflect.TypeFor[any]() {
		var err error
		elemZero, err = setSchema[Out](&tt.OutputSchema, &outputResolved)
		if err != nil {
			return nil, nil, fmt.Errorf("output schema: %v", err)
		}
	}

	th := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input json.RawMessage
		if req.Params.Arguments != nil {
			input = req.Params.Arguments
		}

		// Validate input and apply defaults.
		var err error
		input, err = applySchema(input, inputResolved)
		if err != nil {
			// TODO(#450): should this be considered a tool error? (and similar below)
			return nil, fmt.Errorf("%w: validating \"arguments\": %v", NewError(-32602, "invalid params"), err)
		}

		// Unmarshal and validate args.
		var in In
		if input != nil {
			if err := json.Unmarshal(input, &in); err != nil {
				return nil, fmt.Errorf("%w: %v", NewError(-32602, "invalid params"), err)
			}
		}

		// Call typed handler.
		res, out, err := h(ctx, req, in)
		// Handle server errors appropriately:
		// - If the handler returns a structured error (like jsonrpc2.WireError), return it directly
		// - If the handler returns a regular error, wrap it in a CallToolResult with IsError=true
		// - This allows tools to distinguish between protocol errors and tool execution errors
		if err != nil {
			// Check if this is already a structured JSON-RPC error
			if wireErr, ok := err.(*WireError); ok {
				return nil, wireErr
			}
			// For regular errors, embed them in the tool result as per MCP spec
			var errRes mcp.CallToolResult
			setError(&errRes, err)
			return &errRes, nil
		}

		if res == nil {
			res = &mcp.CallToolResult{}
		}

		// Marshal the output and put the RawMessage in the StructuredContent field.
		var outval any = out
		if elemZero != nil {
			// Avoid typed nil, which will serialize as JSON null.
			// Instead, use the zero value of the unpointered type.
			var z Out
			if any(out) == any(z) { // zero is only non-nil if Out is a pointer type
				outval = elemZero
			}
		}
		if outval != nil {
			outbytes, err := json.Marshal(outval)
			if err != nil {
				return nil, fmt.Errorf("marshaling output: %w", err)
			}
			outJSON := json.RawMessage(outbytes)
			// Validate the output JSON, and apply defaults.
			//
			// We validate against the JSON, rather than the output value, as
			// some types may have custom JSON marshalling (issue #447).
			outJSON, err = applySchema(outJSON, outputResolved)
			if err != nil {
				return nil, fmt.Errorf("validating tool output: %w", err)
			}
			res.StructuredContent = outJSON // avoid a second marshal over the wire

			// If the Content field isn't being used, return the serialized JSON in a
			// TextContent block, as the spec suggests:
			// https://modelcontextprotocol.io/specification/2025-06-18/server/tools#structured-content.
			if res.Content == nil {
				res.Content = []mcp.Content{&mcp.TextContent{
					Text: string(outJSON),
				}}
			} else {
				// to handle prepopulating Binary content , we check a PlaceHOLDER text content to trigger the
				// fullfill
				if len(res.Content) > 0 {
					if textc, ok := res.Content[0].(*mcp.TextContent); ok && textc.Text == "STRUCTURED_CONTENT_PLACEHOLDER" {
						res.Content[0] = &mcp.TextContent{
							Text: string(outJSON),
						}
					}
				}
			}
		}
		return res, nil
	} // end of handler

	return &tt, th, nil
}

// remarshal marshals from to JSON, and then unmarshals into to, which must be
// a pointer type.
func remarshal(from, to any) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, to); err != nil {
		return err
	}
	return nil
}

func setSchema[T any](sfield *any, rfield **jsonschema.Resolved) (zero any, err error) {
	var internalSchema *jsonschema.Schema
	if *sfield == nil {
		rt := reflect.TypeFor[T]()
		if rt.Kind() == reflect.Pointer {
			rt = rt.Elem()
			zero = reflect.Zero(rt).Interface()
		}
		// TODO: we should be able to pass nil opts here.
		internalSchema, err = jsonschema.ForType(rt, &jsonschema.ForOptions{})
		if err == nil {
			*sfield = internalSchema
		}
	} else {
		if err := remarshal(*sfield, &internalSchema); err != nil {
			return zero, err
		}
	}
	if err != nil {
		return zero, err
	}
	*rfield, err = internalSchema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	return zero, err
}

func ToToolHandler[In, Out any](t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) (*mcp.Tool, mcp.ToolHandler, error) {
	tt, hh, err := toolForErr(t, h)
	if err != nil {
		return nil, nil, fmt.Errorf("AddTool: tool %q: %v", t.Name, err)
	}
	return tt, hh, nil
}

func unmarshalSchema(data json.RawMessage, resolved *jsonschema.Resolved, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("unmarshaling: %w", err)
	}
	return validateSchema(resolved, v)
}

func validateSchema(resolved *jsonschema.Resolved, value any) error {
	if resolved != nil {
		if err := resolved.ApplyDefaults(value); err != nil {
			return fmt.Errorf("applying defaults from \n\t%s\nto\n\t%v:\n%w", schemaJSON(resolved.Schema()), value, err)
		}
		if err := resolved.Validate(value); err != nil {
			return fmt.Errorf("validating\n\t%v\nagainst\n\t %s:\n %w", value, schemaJSON(resolved.Schema()), err)
		}
	}
	return nil
}

func schemaJSON(s *jsonschema.Schema) string {
	m, err := json.Marshal(s)
	if err != nil {
		return fmt.Sprintf("<!%s>", err)
	}
	return string(m)
}

type WireError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func NewError(code int64, message string) error {
	return &WireError{
		Code:    code,
		Message: message,
	}
}

func (err *WireError) Error() string {
	return err.Message
}

func (err *WireError) Is(other error) bool {
	w, ok := other.(*WireError)
	if !ok {
		return false
	}
	return err.Code == w.Code
}
