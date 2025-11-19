package scrapflyprovider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/scrapfly/scrapfly-mcp/internal/sanitizer"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
)

type GetPageToolInput struct {
	URL             string                    `json:"url" jsonschema:"The URL of the page to retrieve."`
	Country         string                    `json:"country,omitempty" jsonschema:"Optional: ISO code for proxy geolocation."`
	Format          scrapfly.Format           `json:"format,omitempty" jsonschema:"For Scraped Content (not data extraction).Format to return the SCRAPED CONTENT in . Example: clean_html,markdown,text,raw"`
	FormatOptions   []scrapfly.FormatOption   `json:"format_options,omitempty" jsonschema:"if format is either markdown or text, additional options to apply to the format (zero to all of 'no_links','no_images','only_content')"`
	ProxyPool       scrapfly.ProxyPool        `json:"proxy_pool,omitempty" jsonschema:"Proxy pool to use (e.g., 'public_residential_pool', default: 'public_datacenter_pool')."`
	RenderingWait   int                       `json:"rendering_wait,omitempty" jsonschema:"Wait for this number of milliseconds before returning the response."`
	CapturePage     bool                      `json:"capture_page,omitempty" jsonschema:"If true, capture the page as a screenshot"`
	CaptureFlags    []scrapfly.ScreenshotFlag `json:"capture_flags,omitempty" jsonschema:"Capture flags to use for the capture."`
	ExtractionModel scrapfly.ExtractionModel  `json:"extraction_model,omitempty" jsonschema:"if provided, the AI model to use for extraction. exclusive with extraction_template."`
	PoW             string                    `json:"pow" jsonschema:"use scraping_instruction_enhanced tool use for instructions"`
}

type Cookie struct {
	Name    string `json:"name" jsonschema:"The name of the cookie."`
	Value   string `json:"value" jsonschema:"The value of the cookie."`
	Domain  string `json:"domain,omitempty" jsonschema:"The domain of the cookie."`
	Path    string `json:"path,omitempty" jsonschema:"The path of the cookie."`
	Expires int    `json:"expires,omitempty" jsonschema:"The expiration date of the cookie."`
	MaxAge  int    `json:"max_age,omitempty" jsonschema:"The maximum age of the cookie in seconds."`
}

type ScreenshotTarget string

const (
	ScreenshotTargetFullpage ScreenshotTarget = "fullpage"
	ScreenshotTargetSelector ScreenshotTarget = "selector"
)

type ScreenshotParams struct {
	Name        string           `json:"name"`
	Target      ScreenshotTarget `json:"target"`
	CSSSelector string           `json:"css_selector,omitempty"`
}

func ScreenShotParamsArrayToMap(params []ScreenshotParams) (map[string]string, error) {
	screenshotMap := make(map[string]string)
	for _, param := range params {
		if param.Target == ScreenshotTargetFullpage {
			screenshotMap[param.Name] = "fullpage"
		} else {
			if param.CSSSelector == "" {
				return nil, fmt.Errorf("screenshot_params[%s].css_selector is required if target is selector", param.Name)
			}
			screenshotMap[param.Name] = param.CSSSelector
		}
	}
	return screenshotMap, nil
}

type ScrapeToolInput struct {
	URL              string                    `json:"url" jsonschema:"The URL to scrape."`
	Method           scrapfly.HttpMethod       `json:"method,omitempty" jsonschema:"HTTP method (GET, POST, etc.)."`
	Body             string                    `json:"body,omitempty" jsonschema:"Request body for POST/PUT/PATCH requests."`
	Headers          map[string]string         `json:"headers,omitempty" jsonschema:"HTTP headers to send."`
	Country          string                    `json:"country,omitempty" jsonschema:"ISO 3166-1 alpha-2 country code for proxy geolocation."`
	ProxyPool        scrapfly.ProxyPool        `json:"proxy_pool,omitempty" jsonschema:"Proxy pool to use (e.g., 'public_residential_pool', default: 'public_datacenter_pool')."`
	RenderJS         bool                      `json:"render_js,omitempty" jsonschema:"Enable JavaScript rendering with a headless browser."`
	RenderingWait    int                       `json:"rendering_wait,omitempty" jsonschema:"Wait for this number of milliseconds before returning the response."`
	ASP              bool                      `json:"asp,omitempty" jsonschema:"(prefer true)Enable Anti Scraping Protection solver."`
	Cache            bool                      `json:"cache,omitempty" jsonschema:"Enable caching of the response."`
	CacheTTL         int                       `json:"cache_ttl,omitempty" jsonschema:"Cache TTL in seconds when cache is true."`
	CacheClear       bool                      `json:"cache_clear,omitempty" jsonschema:"If true, bypass & clear cache for this URL."`
	Retry            bool                      `json:"retry,omitempty" jsonschema:"If false, disable automatic retry on transient errors."`
	WaitForSelector  string                    `json:"wait_for_selector,omitempty" jsonschema:"(Prefer rendering_wait). Wait for this CSS selector to appear in the page when rendering JS."`
	Lang             []string                  `json:"lang,omitempty" jsonschema:"Language to use for the request."`
	Cookies          []Cookie                  `json:"cookies,omitempty" jsonschema:"Cookies to send with the request."`
	Format           scrapfly.Format           `json:"format,omitempty" jsonschema:"For Scraped Content (not data extraction).Format to return the SCRAPED CONTENT in . Example: clean_html,markdown,text,raw"`
	FormatOptions    []scrapfly.FormatOption   `json:"format_options,omitempty" jsonschema:"if format is either markdown or text, additional options to apply to the format (zero to all of 'no_links','no_images','only_content')"`
	JS               string                    `json:"js,omitempty" jsonschema:"JavaScript to execute on the page."`
	JSScenario       []map[string]interface{}  `json:"js_scenario,omitempty" jsonschema:"JavaScript scenario to execute on the page. Documentation: https://scrapfly.io/docs/scrapfly-js/js-scenario/"`
	Screenshots      []ScreenshotParams        `json:"screenshots,omitempty" jsonschema:"array of screenshots with target (fullpage, selector). Example: [{ 'name': 'my_screenshot', 'target': 'fullpage' }, { 'name': 'my_screenshot2', 'target': 'selector', 'css_selector': '#price' }]"`
	ScreenshotFlags  []scrapfly.ScreenshotFlag `json:"screenshot_flags,omitempty" jsonschema:"Screenshot flags to use for the screenshot."`
	Timeout          int                       `json:"timeout,omitempty" jsonschema:"Server-side timeout in milliseconds. (Prefer rendering_wait + timeout) "`
	ExtractionPrompt string                    `json:"extraction_prompt,omitempty" jsonschema:"(Avoid if the llm is thinking and can process the data itself). If data extraction cannot be assumed by the current llm model,AI prompt to add step of llm assisted data extraction."`
	ExtractionModel  scrapfly.ExtractionModel  `json:"extraction_model,omitempty" jsonschema:"if provided, the AI model to use for extraction. exclusive with extraction_template."`
	PoW              string                    `json:"pow" jsonschema:"use scraping_instruction_enhanced tool use for instructions"`
	//ExtractionModel  string                   `json:"extraction_model,omitempty" jsonschema:"if provided, the AI model to use for extraction. exclusive with extraction_template."`
}

type LightScrapeResultData struct {
	Content          string                         `json:"content" jsonschema:"HTML/Text/JSON depending on format"`
	StatusCode       int                            `json:"status_code"`
	ContentType      string                         `json:"content_type"`
	Screenshots      map[string]scrapfly.Screenshot `json:"screenshots,omitempty" jsonschema:"Screenshots if any"`
	ExtractionResult map[string]any                 `json:"extraction_result,omitempty" jsonschema:"Extracted data if extraction_prompt was provided"`
	Errors           *scrapfly.APIErrorDetails      `json:"errors,omitempty" jsonschema:"Errors if any"`
}

func InvalidPoWError() (*mcp.CallToolResult, *LightScrapeResultData, error) {
	lightScrapeResult := &LightScrapeResultData{
		Content:     "Dear Assistant: Please execute scraping_instruction_enhanced tool first then reuse this one with new aquired knowledge (pow required)",
		StatusCode:  200,
		ContentType: "text/plain",
		Screenshots: make(map[string]scrapfly.Screenshot),
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: lightScrapeResult.Content}}, IsError: true}, lightScrapeResult, nil
}

func ScrapingInputElement[In ScrapingInput, Out any](element string, input In) (Out, bool) {
	e, ok := input.AsMap()[element].(Out)
	return e, ok
}

// Constraint
type ScrapingInput interface {
	ScrapeToolInput | GetPageToolInput
	AsMap() map[string]any
}

func (input ScrapeToolInput) AsMap() map[string]any {
	return map[string]any{
		"url":               input.URL,
		"format":            input.Format,
		"format_options":    input.FormatOptions,
		"proxy_pool":        input.ProxyPool,
		"render_js":         input.RenderJS,
		"rendering_wait":    input.RenderingWait,
		"asp":               input.ASP,
		"cache":             input.Cache,
		"cache_ttl":         input.CacheTTL,
		"cache_clear":       input.CacheClear,
		"body":              input.Body,
		"headers":           input.Headers,
		"country":           input.Country,
		"lang":              input.Lang,
		"cookies":           input.Cookies,
		"js":                input.JS,
		"js_scenario":       input.JSScenario,
		"screenshot_flags":  input.ScreenshotFlags,
		"screenshots":       input.Screenshots,
		"timeout":           input.Timeout,
		"extraction_prompt": input.ExtractionPrompt,
		"extraction_model":  input.ExtractionModel,
		"pow":               input.PoW,
		"method":            input.Method,
		"retry":             input.Retry,
		"wait_for_selector": input.WaitForSelector,
	}
}

func (input GetPageToolInput) AsMap() map[string]any {
	return map[string]any{
		"url":              input.URL,
		"format":           input.Format,
		"format_options":   input.FormatOptions,
		"proxy_pool":       input.ProxyPool,
		"rendering_wait":   input.RenderingWait,
		"pow":              input.PoW,
		"country":          input.Country,
		"capture_page":     input.CapturePage,
		"capture_flags":    input.CaptureFlags,
		"extraction_model": input.ExtractionModel,
	}
}

func ScrapeConfigFromScrapeToolInput(input ScrapeToolInput) (*scrapfly.ScrapeConfig, error) {
	if input.Method == "" {
		input.Method = "GET"
	}

	if input.ExtractionModel != "" && input.ExtractionPrompt != "" {
		return nil, fmt.Errorf("extraction_model and extraction_prompt cannot be used together")
	}

	if input.ExtractionPrompt != "" {
		if strings.Contains(input.ExtractionPrompt, " ") {
			input.ExtractionPrompt = url.QueryEscape(input.ExtractionPrompt)
		}
	}

	cookies := make(map[string]string, len(input.Cookies))
	for _, cookie := range input.Cookies {
		cookies[cookie.Name] = cookie.Value
	}
	var err error
	screenshots := make(map[string]string, len(input.Screenshots))
	screenshots, err = ScreenShotParamsArrayToMap(input.Screenshots)
	if err != nil {
		return nil, err
	}
	config := &scrapfly.ScrapeConfig{
		URL:              input.URL,
		Method:           input.Method,
		Body:             input.Body,
		Headers:          input.Headers,
		Country:          input.Country,
		ProxyPool:        input.ProxyPool,
		RenderJS:         input.RenderJS,
		RenderingWait:    input.RenderingWait,
		ASP:              input.ASP,
		Cache:            input.Cache,
		CacheTTL:         input.CacheTTL,
		CacheClear:       input.CacheClear,
		Retry:            input.Retry,
		WaitForSelector:  input.WaitForSelector,
		Lang:             input.Lang,
		Cookies:          cookies,
		JS:               input.JS,
		JSScenario:       input.JSScenario,
		Screenshots:      screenshots,
		ScreenshotFlags:  input.ScreenshotFlags,
		Timeout:          input.Timeout,
		ExtractionPrompt: input.ExtractionPrompt,
		ExtractionModel:  input.ExtractionModel,
	}

	if input.ExtractionPrompt != "" || input.ExtractionModel != "" {
		config.ASP = true
		config.Timeout = config.Timeout + 15000
		if config.Timeout < 45000 {
			config.Timeout = 45000
		}
	}

	if input.RenderingWait != 0 {
		config.Timeout = config.Timeout + input.RenderingWait
	}

	if config.ASP {
		config.Retry = true
	}
	if config.Retry {
		config.Timeout = 0
	}

	return config, nil
}
func ScrapeConfigFromGetPageToolInput(input GetPageToolInput) (*scrapfly.ScrapeConfig, error) {
	config := &scrapfly.ScrapeConfig{
		URL: input.URL,
	}
	if input.CapturePage {
		config.Screenshots = map[string]string{
			"capture": "fullpage",
		}
	}
	if input.ExtractionModel != "" {
		config.ASP = true
		config.Timeout = config.Timeout + 15000
		if config.Timeout < 45000 {
			config.Timeout = 45000
		}
	}
	config.RenderJS = true
	config.ASP = true
	config.Country = input.Country
	config.Format = scrapfly.FormatMarkdown
	config.ProxyPool = input.ProxyPool
	config.RenderingWait = input.RenderingWait
	config.ScreenshotFlags = input.CaptureFlags
	config.ExtractionModel = input.ExtractionModel
	return config, nil
}

func ScrapeConfigFromInput[T ScrapingInput](input T) (*scrapfly.ScrapeConfig, error) {
	var config *scrapfly.ScrapeConfig
	var err error

	switch any(input).(type) {
	case ScrapeToolInput:
		config, err = ScrapeConfigFromScrapeToolInput(any(input).(ScrapeToolInput))
	case GetPageToolInput:
		config, err = ScrapeConfigFromGetPageToolInput(any(input).(GetPageToolInput))
	}
	if err != nil {
		return nil, err
	}

	if format, ok := ScrapingInputElement[T, scrapfly.Format]("format", input); ok {
		if format != "" {
			config.Format = format
			if formatOptions, ok := ScrapingInputElement[T, []scrapfly.FormatOption]("format_options", input); ok {
				config.FormatOptions = formatOptions
			}
		}
	}

	return config, nil
}

func (p *ScrapflyToolProvider) LightScrapeResultFromScrapeConfig(ctx context.Context, req *mcp.CallToolRequest, config *scrapfly.ScrapeConfig) (*mcp.CallToolResult, *LightScrapeResultData, error) {
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("scrape", err), nil, err
	}

	stopChan := make(chan struct{})
	// ensure to notify progress routine to stop
	defer close(stopChan)
	go p.progressRoutine(ctx, req, config.URL, stopChan)

	scrapeResult, err := client.Scrape(config)
	if scrapeResult == nil {
		return ToolErrFromError("scrape", err), nil, err
	}

	sanitizer.BasicSanitizeNils(scrapeResult)
	resultdata := scrapeResult.Result
	lightScrapeResult := &LightScrapeResultData{
		Content:     resultdata.Content,
		StatusCode:  resultdata.StatusCode,
		ContentType: resultdata.ContentType,
	}
	if err != nil {
		if resultdata.Error != nil {
			lightScrapeResult.Errors = resultdata.Error
		}
		return ToolErrFromError("scrape", err), lightScrapeResult, err
	}

	// This is disabled because either clients are not properly handling multimodal content
	// or it makes CallToolResult too big and majority of clients are not able to handle it properly.
	// (constated hard limit is usually arbitrary 1MB, even in official mcp go client sdk it is hardcoded to 1MB and not configurable)

	// var callToolResult *mcp.CallToolResult
	// if len(scrapeResult.Result.Screenshots) > 0 {
	// 	callToolResult = mcpex.NewPlaceholderMCPCallToolResult()
	// 	images, err := MCPImageContentFromScrapeResult(scrapeResult)
	// 	if err != nil {
	// 		return nil, lightScrapeResult, err
	// 	}
	// 	for _, image := range images {
	// 		callToolResult.Content = append(callToolResult.Content, &image)
	// 	}
	// }

	if scrapeResult.Result.ExtractedData != nil {
		extractedData := map[string]any{}
		extractedData["data"] = scrapeResult.Result.ExtractedData.Data
		extractedData["data_quality"] = scrapeResult.Result.ExtractedData.DataQuality
		extractedData["content_type"] = scrapeResult.Result.ExtractedData.ContentType
		lightScrapeResult.ExtractionResult = extractedData
	}
	return nil, lightScrapeResult, err
}

func ScrapingHandlerFor[T ScrapingInput](p *ScrapflyToolProvider) mcp.ToolHandlerFor[T, *LightScrapeResultData] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input T) (*mcp.CallToolResult, *LightScrapeResultData, error) {
		pow, ok := ScrapingInputElement[T, string]("pow", input)
		if !ok {
			return InvalidPoWError()
		}
		switch any(input).(type) {
		case ScrapeToolInput:
			return ScrapeCall(p, ctx, req, any(input).(ScrapeToolInput).PoW, any(input).(ScrapeToolInput))
		case GetPageToolInput:
			return ScrapeCall(p, ctx, req, any(input).(GetPageToolInput).PoW, any(input).(GetPageToolInput))
		}
		return ScrapeCall(p, ctx, req, pow, input)
	}
}

func ScrapeCall[T ScrapingInput](p *ScrapflyToolProvider,
	ctx context.Context,
	req *mcp.CallToolRequest,
	pow string,
	input T,
) (*mcp.CallToolResult, *LightScrapeResultData, error) {

	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("scrape", err), nil, err
	}

	p.logger.Println("Executing scraping call for client: ", client.APIKey())
	if !strings.HasPrefix(pow, "i_know_what_i_am_doing:") {
		return InvalidPoWError()
	}

	config, err := ScrapeConfigFromInput(input)
	if err != nil {
		return ToolErrFromError("scrape", err), nil, err
	}

	return p.LightScrapeResultFromScrapeConfig(ctx, req, config)
}

// This is disabled because either clients are not properly handling multimodal content
// or it makes CallToolResult too big and majority of clients are not able to handle it properly.
// (constated hard limit is usually arbitrary 1MB, even in official mcp go client sdk it is hardcoded to 1MB and not configurable)
// func MCPImageContentFromScrapeResult(result *scrapfly.ScrapeResult) ([]mcp.ImageContent, error) {
// 	if result.Result.Screenshots == nil {
// 		return nil, fmt.Errorf("no screenshots found")
// 	}
// 	screenshots := result.Result.Screenshots
// 	contents := []mcp.ImageContent{}
// 	for name, screenshot := range screenshots {
// 		response, err := http.Get(screenshot.URL)
// 		if err != nil {
// 			return nil, err
// 		}
// 		image, err := io.ReadAll(response.Body)
// 		if err != nil {
// 			response.Body.Close()
// 			return nil, err
// 		}
// 		contents = append(contents, mcp.ImageContent{
// 			Data:     image,
// 			MIMEType: fmt.Sprintf("image/%s", screenshot.Extension),
// 			Meta: mcp.Meta{
// 				"screenshot_name": name,
// 			},
// 		})
// 		response.Body.Close()
// 	}
// 	return contents, nil
// }
