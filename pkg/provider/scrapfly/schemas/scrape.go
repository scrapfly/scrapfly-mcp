package schemas

import (
	"encoding/json"
	"log"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/scrapfly/go-scrapfly"
	js_scenario "github.com/scrapfly/go-scrapfly/scenario"
)

type screenshotTarget string

const (
	screenshotTargetFullpage screenshotTarget = "fullpage"
	screenshotTargetSelector screenshotTarget = "selector"
)

type screenshotParams struct {
	Name        string           `json:"name"`
	Target      screenshotTarget `json:"target"`
	CSSSelector string           `json:"css_selector,omitempty"`
}

type cookie struct {
	Name    string `json:"name" jsonschema:"The name of the cookie."`
	Value   string `json:"value" jsonschema:"The value of the cookie."`
	Domain  string `json:"domain,omitempty" jsonschema:"The domain of the cookie."`
	Path    string `json:"path,omitempty" jsonschema:"The path of the cookie."`
	Expires int    `json:"expires,omitempty" jsonschema:"The expiration date of the cookie."`
	MaxAge  int    `json:"max_age,omitempty" jsonschema:"The maximum age of the cookie in seconds."`
}

func Ptr[T any](v T) *T {
	return &v
}

func SchemaFor[T any]() *jsonschema.Schema {
	var v T
	schema, err := jsonschema.For[T](&jsonschema.ForOptions{})
	if err != nil {
		log.Fatalf("Failed to make schema for %T: %v", v, err)
	}
	return schema
}

func MakeCapturePageSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Capture Page",
		Type:        "boolean",
		Description: "If true, also capture the page as a screenshot.",
		Default:     json.RawMessage(`false`),
	}
}

func MakeCookiesSchema() *jsonschema.Schema {
	schema := SchemaFor[[]cookie]()
	schema.Title = "Cookies"
	schema.Description = "Cookies to send with the request."
	schema.Default = json.RawMessage(`[]`)
	return schema
}

func MakeScreenshotsSchema() *jsonschema.Schema {
	schema := SchemaFor[[]screenshotParams]()
	schema.Title = "Screenshots"
	schema.Items.Properties["target"] = &jsonschema.Schema{
		Type: "string",
		Enum: []any{string(screenshotTargetFullpage), string(screenshotTargetSelector)},
	}
	schema.Description = "Screenshots with target (fullpage, selector). Example: [{ 'name': 'my_screenshot', 'target': 'fullpage' }, { 'name': 'my_screenshot2', 'target': 'selector', 'css_selector': '#price' }]"
	schema.Default = json.RawMessage(`[]`)
	return schema
}

func MakeASPSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Anti Scraping Protection",
		Type:        "boolean",
		Description: "Enable Anti Scraping Protection.",
		Default:     json.RawMessage(`true`),
	}
}

func MakeRetrySchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Retry",
		Type:        "boolean",
		Description: "If false, disable automatic retry on transient errors.",
		Default:     json.RawMessage(`true`),
	}
}

func MakeLangSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Language",
		Type:        "array",
		Description: "Languages to use for the request (Accept-Language header). Empty for auto-detection/Proxy Location alignment",
		Items: &jsonschema.Schema{
			Type: "string",
		},
		Default: json.RawMessage(`[]`),
	}
}

func MakeRenderJSSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Render JavaScript",
		Type:        "boolean",
		Description: "Enable JavaScript rendering with a headless browser.",
		Default:     json.RawMessage(`true`),
	}
}

func MakeRenderingWaitSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Rendering Wait",
		Type:        "integer",
		Description: "Wait for this number of milliseconds before returning the response.",
	}
}

func MakeUrlSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Target URL",
		Type:        "string",
		Format:      "uri",
		Description: "The target URL to scrape.",
		Pattern:     "^https?://.*$",
	}
}

func MakeHeadersSchema() *jsonschema.Schema {
	schema := SchemaFor[map[string]string]()
	schema.Title = "HTTP Headers"
	schema.Description = "HTTP headers to send."
	schema.Default = json.RawMessage(`{}`)
	return schema
}

func MakeCountrySchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Proxy Country",
		Type:        "string",
		Format:      "iso-3166-1-alpha-2",
		Description: "The country to use for the proxy. Supports ISO 3166-1 alpha-2 country codes.",
		Pattern:     "^([a-zA-Z]{2}|)$",
		Default:     json.RawMessage(`""`),
	}
}

func MakeExtractionModelSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "Extraction Model",
		Type:        "string",
		Enum:        scrapfly.GetAnyEnumFor[scrapfly.ExtractionModel](),
		Default:     json.RawMessage(`""`),
		Description: "The extraction model to use for the offloaded extraction. Exclusive with extraction_template and extraction_prompt.",
	}
}

func MakeProxyPoolSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Enum:        scrapfly.GetAnyEnumFor[scrapfly.ProxyPool](),
		Default:     json.RawMessage(`"public_datacenter_pool"`),
		Description: "The proxy pool to use. Supports public_datacenter_pool and public_residential_pool, defaults: public_datacenter_pool",
	}
}

func MakeFormatSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Enum:        scrapfly.GetAnyEnumFor[scrapfly.Format](),
		Default:     json.RawMessage(`"markdown"`),
		Description: "The desired output format for the content. Supports clean_html, markdown, text, and json",
	}
}

func MakeMethodSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "string",
		Enum:        scrapfly.GetAnyEnumFor[scrapfly.HttpMethod](),
		Default:     json.RawMessage(`"GET"`),
		Description: "The HTTP method to use for the request.",
	}
}
func MakeFormatOptionsSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "array",
		Description: "Additional options (only available for markdown and text formats)",
		Items: &jsonschema.Schema{
			Type: "string",
			Enum: scrapfly.GetAnyEnumFor[scrapfly.FormatOption](),
		},
		Default: json.RawMessage(`[]`),
	}
}

func MakeScreenshotFlagsSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "array",
		Description: "Screenshot flags to use for the screenshot.",
		Items: &jsonschema.Schema{
			Type: "string",
			Enum: scrapfly.GetAnyEnumFor[scrapfly.ScreenshotFlag](),
		},
		Default: json.RawMessage(`[]`),
	}
}

func MustRefineScrapingToolInputSchema[T any]() *jsonschema.Schema {

	schema := SchemaFor[T]()
	schema.Properties["url"] = MakeUrlSchema()
	schema.Properties["country"] = MakeCountrySchema()
	schema.Properties["format"] = MakeFormatSchema()
	schema.Properties["format_options"] = MakeFormatOptionsSchema()
	schema.Properties["proxy_pool"] = MakeProxyPoolSchema()
	schema.Properties["rendering_wait"] = MakeRenderingWaitSchema()
	schema.Properties["extraction_model"] = MakeExtractionModelSchema()

	// if capture_page is in the schema, its GetPageToolInput so add the property
	if _, ok := schema.Properties["capture_page"]; ok {
		schema.Properties["capture_page"] = MakeCapturePageSchema()
		schema.Properties["capture_flags"] = MakeScreenshotFlagsSchema()
	}

	// if asp is in the schema, its full ScrapeToolInput so add all the properties
	if _, ok := schema.Properties["asp"]; ok {
		schema.Properties["asp"] = MakeASPSchema()
		schema.Properties["retry"] = MakeRetrySchema()
		schema.Properties["lang"] = MakeLangSchema()
		schema.Properties["render_js"] = MakeRenderJSSchema()
		schema.Properties["cookies"] = MakeCookiesSchema()
		schema.Properties["screenshots"] = MakeScreenshotsSchema()
		schema.Properties["screenshot_flags"] = MakeScreenshotFlagsSchema()
		schema.Properties["headers"] = MakeHeadersSchema()
		schema.Properties["js_scenario"] = js_scenario.JsScenarioSchemaFlattened
		schema.Properties["method"] = MakeMethodSchema()
	}

	return schema
}
