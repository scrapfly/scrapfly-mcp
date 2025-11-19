package scrapflyprovider

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/go-scrapfly"
)

type ScreenshotToolInput struct {
	URL             string                      `json:"url" jsonschema:"The URL to screenshot."`
	Format          scrapfly.ScreenshotFormat   `json:"format,omitempty" jsonschema:"The format to use for the screenshot."`
	Capture         string                      `json:"capture,omitempty" jsonschema:"The capture to use for the screenshot. either fullpage or a CSS selector"`
	Resolution      string                      `json:"resolution,omitempty" jsonschema:"The resolution to use for the screenshot. e.g. 1920x1080"`
	Country         string                      `json:"country,omitempty" jsonschema:"The country to use for the screenshot."`
	RenderingWait   int                         `json:"rendering_wait,omitempty" jsonschema:"The rendering wait to use for the screenshot. "`
	WaitForSelector string                      `json:"wait_for_selector,omitempty" jsonschema:"The wait for selector to use for the screenshot."`
	Options         []scrapfly.ScreenshotOption `json:"options,omitempty" jsonschema:"The options to use for the screenshot."`
	AutoScroll      bool                        `json:"auto_scroll,omitempty" jsonschema:"If true, automatically scroll the page to load lazy content."`
	JS              string                      `json:"js,omitempty" jsonschema:"The JavaScript to execute before capturing."`
	Cache           bool                        `json:"cache,omitempty" jsonschema:"If true, enable response caching."`
	CacheTTL        int                         `json:"cache_ttl,omitempty" jsonschema:"The cache time-to-live in seconds."`
	CacheClear      bool                        `json:"cache_clear,omitempty" jsonschema:"If true, bypass & clear cache for this request."`
	Webhook         string                      `json:"webhook,omitempty" jsonschema:"The webhook to call after the request completes."`
}

func ScreenshotConfigFromScreenshotToolInput(input ScreenshotToolInput) (*scrapfly.ScreenshotConfig, error) {
	config := &scrapfly.ScreenshotConfig{
		URL:             input.URL,
		Format:          input.Format,
		Capture:         input.Capture,
		Resolution:      input.Resolution,
		Country:         input.Country,
		RenderingWait:   input.RenderingWait,
		WaitForSelector: input.WaitForSelector,
		AutoScroll:      input.AutoScroll,
		JS:              input.JS,
		Cache:           input.Cache,
		CacheTTL:        input.CacheTTL,
		CacheClear:      input.CacheClear,
		Webhook:         input.Webhook,
	}
	if len(input.Options) > 0 {
		config.Options = input.Options
	}
	return config, nil
}

func (p *ScrapflyToolProvider) Screenshot(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ScreenshotToolInput,
) (*mcp.CallToolResult, any, error) {
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("screenshot", err), nil, err
	}

	p.logger.Println("Executing screenshot call for client: ", client.APIKey())

	config, err := ScreenshotConfigFromScreenshotToolInput(input)
	if err != nil {
		return ToolErrFromError("screenshot", err), nil, err
	}

	stopChan := make(chan struct{})
	// ensure to notify progress routine to stop
	defer close(stopChan)
	go p.progressRoutine(ctx, req, config.URL, stopChan)

	screenshotResult, err := client.Screenshot(config)
	if err != nil {
		return ToolErrFromError("screenshot", err), nil, err
	}
	content := &mcp.ImageContent{
		Data:     screenshotResult.Image,
		MIMEType: fmt.Sprintf("image/%s", screenshotResult.Metadata.ExtensionName),
	}
	return &mcp.CallToolResult{Content: []mcp.Content{content}}, nil, err
}
