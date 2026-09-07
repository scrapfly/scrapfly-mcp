package scrapflyprovider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/scrapfly/scrapfly-mcp/pkg/tools"
)

// allToolSets is every toolset that can reach tools/list. Kept in one place
// so a new family is covered by all the quality assertions at once.
func allToolSets(t *testing.T) map[string]tools.HandledToolSet {
	t.Helper()
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
	}
	return sets
}

// toolProperties renders a tool's input schema the way tools/list does and
// returns its properties. Asserting on the marshalled schema rather than the
// Go struct is what catches inference bugs (a []byte field that reflects to
// an array of uint8, a struct tag copied verbatim into the description).
func toolProperties(t *testing.T, name string, raw any) map[string]map[string]any {
	t.Helper()
	if raw == nil {
		return nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("tool %q: marshal input schema: %v", name, err)
	}
	var schema struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(buf, &schema); err != nil {
		t.Fatalf("tool %q: unmarshal input schema: %v", name, err)
	}
	return schema.Properties
}

// The go-sdk assigns the WHOLE `jsonschema` struct tag as the property
// description (jsonschema-go infer.go: `fs.Description = tag`). Writing
// `jsonschema:"description: ..."` therefore ships a literal "description: "
// prefix to every client. Its guard only rejects a `WORD=` prefix, so
// `WORD:` slipped through on 30 alert parameters unnoticed.
func TestParamDescriptionsHaveNoTagKeyPrefix(t *testing.T) {
	for label, set := range allToolSets(t) {
		t.Run(label, func(t *testing.T) {
			for name, handled := range set {
				for param, spec := range toolProperties(t, name, handled.Tool.InputSchema) {
					desc, _ := spec["description"].(string)
					if strings.HasPrefix(strings.ToLower(strings.TrimSpace(desc)), "description:") {
						t.Errorf("tool %q param %q: jsonschema tag leaks its own key into the description (%q) — the tag value IS the description, drop the \"description: \" prefix", name, param, desc)
					}
				}
			}
		})
	}
}

// Every declared parameter needs a description. Tool-definition scorers treat
// schema coverage below 80% as an outright failure on parameter semantics,
// and a model that cannot tell `sustained_minutes` from `evaluation_window_m`
// guesses. 100% is the bar because nothing here is self-evident from its name.
func TestEveryParameterIsDocumented(t *testing.T) {
	for label, set := range allToolSets(t) {
		t.Run(label, func(t *testing.T) {
			for name, handled := range set {
				for param, spec := range toolProperties(t, name, handled.Tool.InputSchema) {
					if desc, _ := spec["description"].(string); strings.TrimSpace(desc) == "" {
						t.Errorf("tool %q param %q: no description — add a `jsonschema:\"...\"` tag", name, param)
					}
				}
			}
		})
	}
}

// A description short enough to be a restatement of the tool name carries no
// information the name and schema don't already give. `info_api_key` sat at
// 34 characters ("Return the Users' ScrapFly API key") and dragged the whole
// server's score down, because server-level scoring weights the WORST tool at
// 40%. The floor is deliberately low — it catches tautologies, not prose.
func TestDescriptionsAreNotTautologies(t *testing.T) {
	const minDescriptionLen = 120
	for label, set := range allToolSets(t) {
		t.Run(label, func(t *testing.T) {
			for name, handled := range set {
				desc := strings.TrimSpace(handled.Tool.Description)
				if len(desc) < minDescriptionLen {
					t.Errorf("tool %q: description is %d chars, under the %d floor — say what it returns and when to reach for it instead of its siblings: %q",
						name, len(desc), minDescriptionLen, desc)
				}
			}
		})
	}
}
