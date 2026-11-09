package scrapflyprovider

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/resources"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/schemas"
)

var (
	// Pools that set the Unblocker themselves. The resolution code handles
	// them, the tool schemas do not offer them.
	newProxyPools = []string{"basic", "unblocker", "unblocker_ultra", "auto"}
	torPool       = "public_tor_pool"
	publicPools   = []string{"public_datacenter_pool", "public_residential_pool"}
)

func TestScrapingSchemasOfferThePublicPools(t *testing.T) {
	for tool, schema := range scrapingToolSchemas() {
		pool := schema.Properties["proxy_pool"]
		if pool == nil {
			t.Fatalf("%s declares no proxy_pool", tool)
		}
		// A default would be applied to every call that omits the pool.
		if pool.Default != nil {
			t.Errorf("%s proxy_pool carries a schema default %s", tool, pool.Default)
		}
		resolved, err := schema.Resolve(nil)
		if err != nil {
			t.Fatalf("resolve %s: %v", tool, err)
		}
		for _, value := range append(slices.Clone(publicPools), torPool) {
			if err := resolved.Validate(map[string]any{"url": "https://example.com", "proxy_pool": value}); err != nil {
				t.Errorf("%s rejects proxy_pool=%q: %v", tool, value, err)
			}
			if !strings.Contains(pool.Description, value) {
				t.Errorf("%s proxy_pool description never names %q", tool, value)
			}
		}
		for _, value := range append(slices.Clone(newProxyPools), "nonexistent_pool") {
			if err := resolved.Validate(map[string]any{"url": "https://example.com", "proxy_pool": value}); err == nil {
				t.Errorf("%s accepts proxy_pool=%q", tool, value)
			}
		}
	}
}

// cloud_browser_open takes the Cloud Browser's short names, the Tor pool and
// the public pool names.
func TestCloudBrowserOpenKeepsItsPoolNames(t *testing.T) {
	schema := schemas.MustRefineCloudBrowserToolInputSchema[CloudBrowserOpenInput]()
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range append([]string{"datacenter", "residential", torPool}, publicPools...) {
		if err := resolved.Validate(map[string]any{"url": "https://example.com", "proxy_pool": value}); err != nil {
			t.Errorf("cloud_browser_open rejects proxy_pool=%q: %v", value, err)
		}
	}
	for _, name := range newProxyPools {
		if err := resolved.Validate(map[string]any{"url": "https://example.com", "proxy_pool": name}); err == nil {
			t.Errorf("cloud_browser_open accepts proxy_pool=%q", name)
		}
	}
	registered := staticTools(NewScrapflyToolProvider(nil, nil, nil))["cloud_browser_open"].Tool.InputSchema
	encoded, err := json.Marshal(registered)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "unblocker_ultra") || strings.Contains(string(encoded), `"auto"`) {
		t.Error("the registered cloud_browser_open schema offers unblocker_ultra or auto")
	}
}

// The API sizes the timeout of an Unblocker request itself, whichever way the
// Unblocker was asked for.
func TestUnblockerPoolsTakeTheBypassRetryRule(t *testing.T) {
	for _, tc := range []struct {
		pool        scrapfly.ProxyPool
		wantRetry   bool
		wantTimeout int
	}{
		{"basic", false, 30000},
		{"unblocker", true, 0},
		{"unblocker_ultra", true, 0},
		{"auto", true, 0},
	} {
		config, err := ScrapeConfigFromScrapeToolInput(ScrapeToolInput{URL: "https://example.com", ProxyPool: tc.pool, Timeout: 30000})
		if err != nil {
			t.Fatal(err)
		}
		if config.Retry != tc.wantRetry || config.Timeout != tc.wantTimeout {
			t.Errorf("%s: retry=%v timeout=%d, want retry=%v timeout=%d", tc.pool, config.Retry, config.Timeout, tc.wantRetry, tc.wantTimeout)
		}
	}
}

func TestExtractionForcesTheToggleWithoutAPoolThatSetsIt(t *testing.T) {
	no := false
	for _, tc := range []struct {
		name     string
		input    ScrapeToolInput
		wantPool scrapfly.ProxyPool
		wantASP  bool
	}{
		{"no pool", ScrapeToolInput{URL: "https://example.com", ExtractionModel: "product"}, "", true},
		{"unblocker false", ScrapeToolInput{URL: "https://example.com", Unblocker: &no, ExtractionModel: "product"}, "", true},
		{"public pool", ScrapeToolInput{URL: "https://example.com", ProxyPool: "public_datacenter_pool", Unblocker: &no, ExtractionModel: "product"}, "public_datacenter_pool", true},
		{"basic", ScrapeToolInput{URL: "https://example.com", ProxyPool: "basic", ExtractionModel: "product"}, "basic", false},
		{"unblocker_ultra", ScrapeToolInput{URL: "https://example.com", ProxyPool: "unblocker_ultra", ExtractionModel: "product"}, "unblocker_ultra", false},
		{"auto", ScrapeToolInput{URL: "https://example.com", ProxyPool: "auto", ExtractionModel: "product"}, "auto", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := ScrapeConfigFromScrapeToolInput(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if config.ProxyPool != tc.wantPool || config.ASP != tc.wantASP {
				t.Errorf("extraction sent proxy_pool=%q asp=%v, want proxy_pool=%q asp=%v", config.ProxyPool, config.ASP, tc.wantPool, tc.wantASP)
			}
		})
	}
}

// web_get_page keeps the Unblocker it has always forced unless a new pool
// decides it.
func TestGetPageKeepsTheUnblockerWithoutANewPool(t *testing.T) {
	for _, tc := range []struct {
		pool     scrapfly.ProxyPool
		wantPool scrapfly.ProxyPool
		wantASP  bool
	}{
		{"", "", true},
		{"public_residential_pool", "public_residential_pool", true},
		{"basic", "basic", false},
		{"unblocker", "unblocker", false},
		{"unblocker_ultra", "unblocker_ultra", false},
		{"auto", "auto", false},
	} {
		config, err := ScrapeConfigFromGetPageToolInput(GetPageToolInput{URL: "https://example.com", ProxyPool: tc.pool})
		if err != nil {
			t.Fatal(err)
		}
		if config.ProxyPool != tc.wantPool || config.ASP != tc.wantASP || !config.RenderJS {
			t.Errorf("proxy_pool=%q: got pool=%q asp=%v render_js=%v, want pool=%q asp=%v render_js=true",
				tc.pool, config.ProxyPool, config.ASP, config.RenderJS, tc.wantPool, tc.wantASP)
		}
	}
}

// recordingTransport answers every request with one 200 scrape body and keeps
// the query each one carried.
type recordingTransport struct {
	mu      sync.Mutex
	queries []url.Values
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.queries = append(t.queries, req.URL.Query())
	t.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(billedScrapeBody)),
		Request:    req,
	}, nil
}

// Through the registered handler, so schema validation and defaults run as
// they do for a client.
func TestScrapingToolsSendTheResolvedPool(t *testing.T) {
	for _, tc := range []struct {
		tool     string
		args     string
		wantPool string
		wantASP  string
	}{
		// Naming nothing sends what v1.4.0 sent: no pool, asp=true.
		{"web_scrape", `{"url":"https://example.com"}`, "", "true"},
		{"web_scrape", `{"url":"https://example.com","unblocker":true}`, "", "true"},
		{"web_scrape", `{"url":"https://example.com","unblocker":false}`, "", ""},
		{"web_scrape", `{"url":"https://example.com","proxy_pool":"public_residential_pool","asp":true}`, "public_residential_pool", "true"},
		{"web_get_page", `{"url":"https://example.com"}`, "", "true"},
	} {
		t.Run(tc.tool+" "+tc.args, func(t *testing.T) {
			client, err := scrapfly.New("test-key")
			if err != nil {
				t.Fatal(err)
			}
			transport := &recordingTransport{}
			client.SetHTTPClient(&http.Client{Transport: transport})
			set := staticTools(NewScrapflyToolProvider(client, GetDefaultScrapflyClient, log.New(io.Discard, "", 0)))
			res, err := set[tc.tool].Handler(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
				Name:      tc.tool,
				Arguments: json.RawMessage(tc.args),
			}})
			if err != nil {
				t.Fatalf("handler: %v", err)
			}
			if res.IsError {
				t.Fatalf("tool error: %v", res.Content)
			}
			if len(transport.queries) != 1 {
				t.Fatalf("%d requests sent, want 1", len(transport.queries))
			}
			query := transport.queries[0]
			if got := query.Get("proxy_pool"); got != tc.wantPool {
				t.Errorf("proxy_pool=%q, want %q", got, tc.wantPool)
			}
			if got := query.Get("asp"); got != tc.wantASP {
				t.Errorf("asp=%q, want %q", got, tc.wantASP)
			}
		})
	}
}

// A pool the schema does not offer is refused before any request.
func TestScrapingToolsRejectAPoolTheyDoNotOffer(t *testing.T) {
	for _, tool := range []string{"web_scrape", "web_get_page"} {
		for _, pool := range append(slices.Clone(newProxyPools), "nonexistent_pool") {
			t.Run(tool+" "+pool, func(t *testing.T) {
				client, err := scrapfly.New("test-key")
				if err != nil {
					t.Fatal(err)
				}
				transport := &recordingTransport{}
				client.SetHTTPClient(&http.Client{Transport: transport})
				set := staticTools(NewScrapflyToolProvider(client, GetDefaultScrapflyClient, log.New(io.Discard, "", 0)))
				res, err := set[tool].Handler(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
					Name:      tool,
					Arguments: json.RawMessage(`{"url":"https://example.com","proxy_pool":"` + pool + `"}`),
				}})
				if err == nil && (res == nil || !res.IsError) {
					t.Errorf("proxy_pool=%q was accepted", pool)
				}
				if len(transport.queries) != 0 {
					t.Errorf("proxy_pool=%q reached the API", pool)
				}
			})
		}
	}
}

// What a model reads names none of the pools the schemas do not offer, nor
// the error or the page that go with them.
func TestModelFacingTextNamesNoPoolTheToolsDoNotOffer(t *testing.T) {
	texts := map[string]string{
		"instruction prompt": resources.InstructionPromptString,
		"OpenAPI resource":   resources.WebScrapingApi,
	}
	for label, set := range allToolSets(t) {
		for name, handled := range set {
			texts[label+" tool "+name] = handled.Tool.Description
			for param, spec := range toolProperties(t, name, handled.Tool.InputSchema) {
				description, _ := spec["description"].(string)
				texts[label+" tool "+name+" param "+param] = description
				if values, ok := spec["enum"].([]any); ok {
					for _, value := range values {
						pool, _ := value.(string)
						if param == "proxy_pool" && slices.Contains(newProxyPools, pool) {
							t.Errorf("%s tool %s offers proxy_pool=%q", label, name, pool)
						}
					}
				}
			}
		}
	}
	hidden := regexp.MustCompile("unblocker_ultra|UNBLOCKER_ULTRA_REQUIRED|new-pricing|proxy_pool[=:] ?`?(basic|unblocker|auto)\\b|\"(basic|auto)\"")
	for name, text := range texts {
		if found := hidden.FindString(text); found != "" {
			t.Errorf("%s names %q", name, found)
		}
	}
}

// No date switch: a call that names no pool keeps the request v1.4.0 sent,
// no pool and asp=true.
func TestDefaultsKeepTheUnblockerOn(t *testing.T) {
	scrape, err := ScrapeConfigFromScrapeToolInput(ScrapeToolInput{URL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if scrape.ProxyPool != "" || !scrape.ASP {
		t.Errorf("web_scrape default: pool=%q asp=%v, want pool=\"\" asp=true", scrape.ProxyPool, scrape.ASP)
	}
	page, err := ScrapeConfigFromGetPageToolInput(GetPageToolInput{URL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if page.ProxyPool != "" || !page.ASP || !page.RenderJS {
		t.Errorf("web_get_page default: pool=%q asp=%v render_js=%v, want pool=\"\" asp=true render_js=true", page.ProxyPool, page.ASP, page.RenderJS)
	}
}
