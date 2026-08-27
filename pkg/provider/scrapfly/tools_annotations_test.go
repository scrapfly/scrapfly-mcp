package scrapflyprovider

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/scrapfly-mcp/pkg/tools"
)

// requiredHints are the annotation keys directory listings (ChatGPT app
// submission, MCP registries) reject a server for omitting.
var requiredHints = []string{"readOnlyHint", "destructiveHint", "openWorldHint"}

// assertAnnotationHints marshals each tool the way tools/list does and checks
// the three hints are present on the wire. Two independent ways to lose one:
// leaving a *bool hint nil, or building against a go-sdk whose
// ToolAnnotations still tags the bool hints `omitempty` (<= v1.6.1, where
// ReadOnlyHint:false serialized to nothing). Asserting on the JSON rather than
// on the struct catches both.
func assertAnnotationHints(t *testing.T, set tools.HandledToolSet) {
	t.Helper()
	for name, handled := range set {
		if handled == nil || handled.Tool == nil {
			t.Errorf("tool %q: no tool definition", name)
			continue
		}
		if handled.Tool.Annotations == nil {
			t.Errorf("tool %q: no annotations", name)
			continue
		}
		raw, err := json.Marshal(handled.Tool.Annotations)
		if err != nil {
			t.Errorf("tool %q: marshal annotations: %v", name, err)
			continue
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Errorf("tool %q: unmarshal annotations: %v", name, err)
			continue
		}
		for _, hint := range requiredHints {
			if _, ok := got[hint]; !ok {
				t.Errorf("tool %q: %s missing from tools/list annotations (%s)", name, hint, raw)
			}
		}
	}
}

func TestToolAnnotationsCarryDirectoryHints(t *testing.T) {
	provider := NewScrapflyToolProvider(nil, nil, nil)
	sets := map[string]tools.HandledToolSet{
		"static":      staticTools(provider),
		"interaction": interactionTools(provider),
		"browser":     browserInteractionTools(provider),
		"dynamic":     provider.dynamicInteractionTools(),
	}
	for label, set := range sets {
		if len(set) == 0 {
			t.Fatalf("%s toolset is empty", label)
		}
		t.Run(label, func(t *testing.T) { assertAnnotationHints(t, set) })
	}
}

// The bool hints only reach the wire because the go-sdk in go.mod dropped
// `omitempty` from them. Pin that: a downgrade silently strips
// readOnlyHint:false from every mutating tool.
func TestSDKSerializesFalseBoolHints(t *testing.T) {
	raw, err := json.Marshal(&mcp.ToolAnnotations{DestructiveHint: &falseBool, OpenWorldHint: &falseBool})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got["readOnlyHint"]; !ok {
		t.Fatalf("go-sdk omits readOnlyHint:false (%s) — needs modelcontextprotocol/go-sdk >= v1.7.0", raw)
	}
}
