package browser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterCDPTools adds Chrome DevTools Protocol tools (screenshot, JS eval, snapshot, etc.)
// that are NOT exposed via WebMCP but are essential for browser automation.
// The mcpServer, meta, and bool pointers are passed from the parent package to avoid circular imports.
func RegisterCDPTools(
	logger Logger,
	mcpServer *mcp.Server,
	sessionID, shortID string,
	toolNames *[]string,
	permissionsMeta mcp.Meta,
) {
	prefix := fmt.Sprintf("browser_%s", shortID)
	emptySchema := map[string]any{"type": "object", "properties": map[string]any{}}

	// take_screenshot — CDP Page.captureScreenshot
	screenshotName := prefix + "_take_screenshot"
	mcpServer.AddTool(&mcp.Tool{
		Name:        screenshotName,
		Title:       "[Browser] Take Screenshot",
		Description: "Take a screenshot of the current page in the browser. Returns a PNG image.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: emptySchema,
		Meta:        permissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := Store.Load(sessionID)
		if !ok {
			return toolErrf("take_screenshot: session not found"), nil
		}
		s := val.(*Session)
		result, err := s.SendCDP("Page.captureScreenshot", map[string]any{"format": "png"})
		if err != nil {
			return toolErrf("take_screenshot: %v", err), nil
		}
		var ss struct{ Data string `json:"data"` }
		json.Unmarshal(result, &ss)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{Data: []byte(ss.Data), MIMEType: "image/png"}},
		}, nil
	})
	*toolNames = append(*toolNames, screenshotName)

	// evaluate_script — CDP Runtime.evaluate
	evalName := prefix + "_evaluate_script"
	mcpServer.AddTool(&mcp.Tool{
		Name:        evalName,
		Title:       "[Browser] Evaluate JavaScript",
		Description: "Execute JavaScript code in the browser page and return the result. Use for reading page content, DOM queries, or any JS logic.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]any{
					"type":        "string",
					"description": "JavaScript expression to evaluate in the page context.",
				},
			},
			"required": []string{"expression"},
		},
		Meta: permissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := Store.Load(sessionID)
		if !ok {
			return toolErrf("evaluate_script: session not found"), nil
		}
		s := val.(*Session)
		var args struct {
			Expression string `json:"expression"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		if args.Expression == "" {
			return toolErrf("evaluate_script: expression is required"), nil
		}
		result, err := s.SendCDP("Runtime.evaluate", map[string]any{
			"expression":    args.Expression,
			"returnByValue": true,
		})
		if err != nil {
			return toolErrf("evaluate_script: %v", err), nil
		}
		var evalResult struct {
			Result struct {
				Type  string `json:"type"`
				Value any    `json:"value"`
			} `json:"result"`
			ExceptionDetails *struct {
				Text string `json:"text"`
			} `json:"exceptionDetails"`
		}
		json.Unmarshal(result, &evalResult)
		if evalResult.ExceptionDetails != nil {
			return toolErrf("evaluate_script error: %s", evalResult.ExceptionDetails.Text), nil
		}
		b, _ := json.MarshalIndent(evalResult.Result.Value, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil
	})
	*toolNames = append(*toolNames, evalName)

	// take_snapshot — get page text content via DOM snapshot
	snapName := prefix + "_take_snapshot"
	mcpServer.AddTool(&mcp.Tool{
		Name:        snapName,
		Title:       "[Browser] Page Content Snapshot",
		Description: "Get the text content of the current page. Useful for reading page content without a screenshot. Returns the document title and body text.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: emptySchema,
		Meta:        permissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := Store.Load(sessionID)
		if !ok {
			return toolErrf("take_snapshot: session not found"), nil
		}
		s := val.(*Session)
		result, err := s.SendCDP("Runtime.evaluate", map[string]any{
			"expression":    `JSON.stringify({title: document.title, url: location.href, text: document.body?.innerText?.substring(0, 50000) || ''})`,
			"returnByValue": true,
		})
		if err != nil {
			return toolErrf("take_snapshot: %v", err), nil
		}
		var evalResult struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		}
		json.Unmarshal(result, &evalResult)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: evalResult.Result.Value}},
		}, nil
	})
	*toolNames = append(*toolNames, snapName)

	// get_page_url — quick way to check current URL
	urlName := prefix + "_get_page_url"
	mcpServer.AddTool(&mcp.Tool{
		Name:        urlName,
		Title:       "[Browser] Get Current URL",
		Description: "Get the current page URL and title from the browser.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: emptySchema,
		Meta:        permissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := Store.Load(sessionID)
		if !ok {
			return toolErrf("get_page_url: session not found"), nil
		}
		s := val.(*Session)
		result, err := s.SendCDP("Runtime.evaluate", map[string]any{
			"expression":    `JSON.stringify({url: location.href, title: document.title})`,
			"returnByValue": true,
		})
		if err != nil {
			return toolErrf("get_page_url: %v", err), nil
		}
		var evalResult struct {
			Result struct{ Value string `json:"value"` } `json:"result"`
		}
		json.Unmarshal(result, &evalResult)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: evalResult.Result.Value}},
		}, nil
	})
	*toolNames = append(*toolNames, urlName)

	// get_performance_metrics — PSI-style lab run (cold-cache reload with
	// Moto G4 + slow 4G + 4× CPU throttling by default). Returns Core Web
	// Vitals, Speed Index, TBT, TTI, resource waterfall, and Lighthouse-style
	// performance score and ratings. See browser/performance.go.
	perfName := prefix + "_get_performance_metrics"
	mcpServer.AddTool(&mcp.Tool{
		Name:        perfName,
		Title:       "[Browser] PageSpeed Lab Run",
		Description: "PSI-style lab run: cold-cache reload with mobile throttling by default. Returns FCP, LCP, CLS, TTFB, INP, TBT, TTI, Speed Index, resource waterfall, and Lighthouse-style performance score (0-100) with Good/Needs-Improvement/Poor ratings.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"preset": map[string]any{
					"type":        "string",
					"enum":        []string{"mobile", "desktop"},
					"description": "Throttling preset. 'mobile' (default) = Moto G4 + slow 4G + 4× CPU. 'desktop' = 1350×940 wired, no CPU throttle.",
				},
				"timeout_ms": map[string]any{
					"type":        "integer",
					"description": "Total budget in ms (default 30000, max 45000).",
				},
			},
		},
		Meta: permissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := Store.Load(sessionID)
		if !ok {
			return toolErrf("get_performance_metrics: session not found"), nil
		}
		s := val.(*Session)

		var args struct {
			Preset    string `json:"preset"`
			TimeoutMs int    `json:"timeout_ms"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &args)

		report, err := CollectPSI(s, PSIOptions{
			Preset:    Preset(args.Preset),
			TimeoutMs: args.TimeoutMs,
		})
		if err != nil {
			return toolErrf("get_performance_metrics: %v", err), nil
		}
		logger.Printf("[PSI] %s — %s", shortID, SummarizeReport(report))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: FormatReport(report)}},
		}, nil
	})
	*toolNames = append(*toolNames, perfName)

	logger.Printf("Registered %d CDP DevTools tools for session %s", 5, shortID)
}
