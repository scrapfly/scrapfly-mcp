package scrapflyprovider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/resources"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/schemas"
)

// The scraping tools must not require an attestation argument: a value gated
// on a fixed magic prefix, or tool output addressed to the model, reads as
// prompt injection to MCP clients and gets the tools refused.
const attestationPrefix = "i_know_what_i_am_doing"

func propertyNames(s *jsonschema.Schema) []string {
	names := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		names = append(names, name)
	}
	return names
}

func scrapingToolSchemas() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"web_scrape":   schemas.MustRefineScrapingToolInputSchema[ScrapeToolInput](),
		"web_get_page": schemas.MustRefineScrapingToolInputSchema[GetPageToolInput](),
	}
}

// Only ScrapeToolInput exposes the anti-bot toggle; web_get_page forces it on
// and declares neither name.
func unblockerToolSchemas() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"web_scrape": schemas.MustRefineScrapingToolInputSchema[ScrapeToolInput](),
	}
}

func TestScrapingToolSchemasRequireOnlyURL(t *testing.T) {
	for tool, schema := range scrapingToolSchemas() {
		// A required deprecated alias would force every caller to send a name
		// they are being moved off; a required new name breaks pinned callers.
		for _, optional := range []string{"pow", "asp", "unblocker"} {
			if slices.Contains(schema.Required, optional) {
				t.Errorf("%s input schema marks %q as required", tool, optional)
			}
		}
		if !slices.Contains(schema.Required, "url") {
			t.Errorf("%s input schema does not require \"url\": %v", tool, schema.Required)
		}
		if len(schema.Required) != 1 {
			t.Errorf("%s input schema requires more than url: %v", tool, schema.Required)
		}
	}
}

// `pow` stays declared but ignored: the schemas are closed
// (additionalProperties: false), so dropping the property outright would
// reject every caller written against the version that demanded it.
func TestScrapingToolSchemasStillAcceptPoW(t *testing.T) {
	for tool, schema := range scrapingToolSchemas() {
		if !slices.Contains(propertyNames(schema), "pow") {
			t.Errorf("%s input schema no longer accepts \"pow\"; existing callers would be rejected", tool)
		}
	}
}

func TestScrapingInputMapsHaveNoPoWKey(t *testing.T) {
	maps := map[string]map[string]any{
		"ScrapeToolInput":  ScrapeToolInput{}.AsMap(),
		"GetPageToolInput": GetPageToolInput{}.AsMap(),
	}
	for name, m := range maps {
		if _, ok := m["pow"]; ok {
			t.Errorf("%s.AsMap() still carries a \"pow\" key", name)
		}
	}
}

// `asp` is the retired customer-facing name for the unblocker and stays
// declared for the same reason `pow` does: the schemas are closed
// (additionalProperties: false), so dropping the property is a hard call
// rejection for every pinned client that still sends it.
func TestScrapingToolSchemasStillAcceptASP(t *testing.T) {
	for tool, schema := range unblockerToolSchemas() {
		if !slices.Contains(propertyNames(schema), "asp") {
			t.Errorf("%s input schema no longer accepts \"asp\"; existing callers would be rejected", tool)
		}
	}
}

func TestScrapingToolSchemasDeclareUnblocker(t *testing.T) {
	for tool, schema := range unblockerToolSchemas() {
		unblocker, ok := schema.Properties["unblocker"]
		if !ok {
			t.Fatalf("%s input schema does not declare \"unblocker\"", tool)
		}
		if unblocker.Type != "boolean" {
			t.Errorf("%s unblocker is %q, want boolean", tool, unblocker.Type)
		}
		// Neither name may carry a schema Default: the go-sdk applies defaults
		// before unmarshal, so a default makes the property always present and
		// the resolver can no longer tell "omitted" from "sent". The effective
		// default lives in ResolveUnblocker.
		if unblocker.Default != nil {
			t.Errorf("%s unblocker carries a schema default %s; presence-based precedence would break", tool, unblocker.Default)
		}
		if asp := schema.Properties["asp"]; asp != nil && asp.Default != nil {
			t.Errorf("%s asp carries a schema default %s; presence-based precedence would break", tool, asp.Default)
		}
	}
}

// The refinement block is gated on a discriminator property. Gating it on a
// name that is being renamed makes every refinement below vanish silently, so
// assert the refined shapes survive.
func TestScrapeSchemaKeepsRefinementsAfterRename(t *testing.T) {
	schema := schemas.MustRefineScrapingToolInputSchema[ScrapeToolInput]()
	for property, wantTitle := range map[string]string{
		"retry":     "Retry",
		"lang":      "Language",
		"render_js": "Render JavaScript",
		"cookies":   "Cookies",
	} {
		got, ok := schema.Properties[property]
		if !ok {
			t.Errorf("web_scrape lost property %q", property)
			continue
		}
		if got.Title != wantTitle {
			t.Errorf("web_scrape %q was not refined: title %q, want %q", property, got.Title, wantTitle)
		}
	}
	for _, property := range []string{"js_scenario", "screenshots", "screenshot_flags", "headers", "method"} {
		if _, ok := schema.Properties[property]; !ok {
			t.Errorf("web_scrape lost property %q", property)
		}
	}
}

// `unblocker` is what callers say; `asp` is what the Scrapfly API is sent.
// Precedence is presence-based, never an OR, and an explicit false on the
// winning name turns the feature off.
func TestResolveUnblockerPrecedence(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name      string
		asp       *bool
		unblocker *bool
		want      bool
	}{
		{"neither supplied defaults to on", nil, nil, true},
		{"unblocker false alone turns it off", nil, &no, false},
		{"unblocker true alone turns it on", nil, &yes, true},
		{"asp true wins over unblocker false", &yes, &no, true},
		{"asp false wins over unblocker true", &no, &yes, false},
		{"asp false alone turns it off", &no, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveUnblocker(tc.asp, tc.unblocker); got != tc.want {
				t.Errorf("ResolveUnblocker(%v, %v) = %v, want %v", tc.asp, tc.unblocker, got, tc.want)
			}
		})
	}
}

// The resolved value must reach scrapfly.ScrapeConfig.ASP, which serialises to
// the `asp` query parameter. The rename moved the caller-facing name only.
func TestScrapeConfigCarriesResolvedUnblocker(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name  string
		input ScrapeToolInput
		want  bool
	}{
		{"omitted", ScrapeToolInput{URL: "https://example.com"}, true},
		{"unblocker false", ScrapeToolInput{URL: "https://example.com", Unblocker: &no}, false},
		{"asp false beats unblocker true", ScrapeToolInput{URL: "https://example.com", ASP: &no, Unblocker: &yes}, false},
		{"asp true beats unblocker false", ScrapeToolInput{URL: "https://example.com", ASP: &yes, Unblocker: &no}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := ScrapeConfigFromScrapeToolInput(tc.input)
			if err != nil {
				t.Fatalf("ScrapeConfigFromScrapeToolInput: %v", err)
			}
			if config.ASP != tc.want {
				t.Errorf("config.ASP = %v, want %v", config.ASP, tc.want)
			}
			if got, _ := tc.input.AsMap()["asp"].(bool); got != tc.want {
				t.Errorf("AsMap()[\"asp\"] = %v, want %v", got, tc.want)
			}
		})
	}
}

// The go-sdk applies schema defaults to the raw argument map and re-marshals
// it before unmarshalling into the input struct (go-sdk mcp/tool.go). A
// Default on either name would land in that map and make both pointers
// non-nil, so precedence has to hold through this path, not just on the Go
// structs.
func TestSchemaDefaultsDoNotForgeUnblockerPresence(t *testing.T) {
	resolved, err := schemas.MustRefineScrapingToolInputSchema[ScrapeToolInput]().Resolve(nil)
	if err != nil {
		t.Fatalf("resolving web_scrape input schema: %v", err)
	}
	for _, tc := range []struct {
		name string
		args string
		want bool
	}{
		{"neither name sent", `{"url":"https://example.com"}`, true},
		{"unblocker false", `{"url":"https://example.com","unblocker":false}`, false},
		{"asp false", `{"url":"https://example.com","asp":false}`, false},
		{"asp true beats unblocker false", `{"url":"https://example.com","asp":true,"unblocker":false}`, true},
		{"asp false beats unblocker true", `{"url":"https://example.com","asp":false,"unblocker":true}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw any
			if err := json.Unmarshal([]byte(tc.args), &raw); err != nil {
				t.Fatalf("unmarshaling arguments: %v", err)
			}
			if err := resolved.ApplyDefaults(&raw); err != nil {
				t.Fatalf("ApplyDefaults: %v", err)
			}
			withDefaults, err := json.Marshal(raw)
			if err != nil {
				t.Fatalf("re-marshaling arguments: %v", err)
			}
			var input ScrapeToolInput
			if err := json.Unmarshal(withDefaults, &input); err != nil {
				t.Fatalf("unmarshaling into ScrapeToolInput: %v", err)
			}
			config, err := ScrapeConfigFromScrapeToolInput(input)
			if err != nil {
				t.Fatalf("ScrapeConfigFromScrapeToolInput: %v", err)
			}
			if config.ASP != tc.want {
				t.Errorf("args %s -> config.ASP = %v, want %v (after defaults: %s)", tc.args, config.ASP, tc.want, withDefaults)
			}
		})
	}
}

// The rename is presentational, so nothing but a text assertion guards it: the
// cheat sheet and the tool descriptions are what teaches a model which name to
// use, and every renamed sentence could be reverted with the rest of the suite
// still green. `asp` may still be named as the retired spelling — that
// sentence is the deprecation notice — but the feature is called the unblocker.
func TestModelFacingTextUsesTheCurrentName(t *testing.T) {
	texts := map[string]string{"instruction prompt": resources.InstructionPromptString}
	for name, handled := range staticTools(NewScrapflyToolProvider(nil, nil, nil)) {
		if handled != nil && handled.Tool != nil {
			texts["tool "+name+" description"] = handled.Tool.Description
		}
	}
	for name, text := range texts {
		if strings.Contains(strings.ToLower(text), "anti-scraping") {
			t.Errorf("%s still calls the unblocker \"anti-scraping\"", name)
		}
	}
	if !strings.Contains(strings.ToLower(resources.InstructionPromptString), "unblocker") {
		t.Error("instruction prompt never names the unblocker")
	}
}

func TestInstructionPromptCarriesNoAttestation(t *testing.T) {
	prompt := resources.InstructionPromptString
	if strings.Contains(prompt, attestationPrefix) {
		t.Errorf("instruction prompt still asks the model to attest with %q", attestationPrefix)
	}
	if strings.Contains(strings.ToLower(prompt), "dear assistant") {
		t.Error("instruction prompt addresses the assistant directly; keep it descriptive")
	}
}

// The SDK always populates Params; the progress notifier dereferences it.
func newCallToolRequest() *mcp.CallToolRequest {
	return &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "test"}}
}

type countingTransport struct{ calls atomic.Int32 }

func (t *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return nil, errors.New("no network in tests")
}

func testProvider(t *testing.T) (*ScrapflyToolProvider, *countingTransport) {
	t.Helper()
	client, err := scrapfly.New("test-key")
	if err != nil {
		t.Fatalf("scrapfly.New: %v", err)
	}
	transport := &countingTransport{}
	client.SetHTTPClient(&http.Client{Transport: transport})
	return NewScrapflyToolProvider(client, GetDefaultScrapflyClient, log.New(io.Discard, "", 0)), transport
}

// A schema check alone would still pass if the runtime gate came back, so
// drive the registered handler with no attestation anywhere and assert it
// reaches the API call.
func TestScrapingHandlersRunWithoutAttestation(t *testing.T) {
	t.Run("web_scrape", func(t *testing.T) {
		p, transport := testProvider(t)
		res, _, _ := ScrapingHandlerFor[ScrapeToolInput](p)(
			context.Background(), newCallToolRequest(), ScrapeToolInput{URL: "https://example.com"})
		assertReachedUpstream(t, res, transport)
	})
	t.Run("web_get_page", func(t *testing.T) {
		p, transport := testProvider(t)
		res, _, _ := ScrapingHandlerFor[GetPageToolInput](p)(
			context.Background(), newCallToolRequest(), GetPageToolInput{URL: "https://example.com"})
		assertReachedUpstream(t, res, transport)
	})
}

func assertReachedUpstream(t *testing.T, res *mcp.CallToolResult, transport *countingTransport) {
	t.Helper()
	if res != nil {
		for _, content := range res.Content {
			text, ok := content.(*mcp.TextContent)
			if !ok {
				continue
			}
			lowered := strings.ToLower(text.Text)
			if strings.Contains(lowered, "dear assistant") || strings.Contains(lowered, attestationPrefix) {
				t.Fatalf("handler answered with an attestation demand: %s", text.Text)
			}
		}
	}
	if transport.calls.Load() == 0 {
		t.Fatal("handler short-circuited before calling the API")
	}
}
