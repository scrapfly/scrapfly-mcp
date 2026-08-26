package scrapflyprovider

import (
	"context"
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

func TestScrapingToolSchemasRequireOnlyURL(t *testing.T) {
	for tool, schema := range scrapingToolSchemas() {
		if slices.Contains(schema.Required, "pow") {
			t.Errorf("%s input schema marks \"pow\" as required", tool)
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
