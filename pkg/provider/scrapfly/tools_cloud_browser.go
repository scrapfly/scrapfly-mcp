package scrapflyprovider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/browser"
)

// antibotToolOverride holds a schema override and optional description for antibot tools
// whose schemas Chrome returns as empty.
type antibotToolOverride struct {
	schema      any
	description string
}

// ── Tool inputs ─────────────────────────────────────────────────────────────

type CloudBrowserOpenInput struct {
	URL         string `json:"url" jsonschema:"Target URL to open in the cloud browser."`
	Country     string `json:"country,omitempty" jsonschema:"Proxy country. ISO 3166-1 alpha-2: 'US', 'DE'. Comma-separated for multiple: 'fr,us,es,de'. Prefix '-' to exclude: '-ru'."`
	ProxyPool   string `json:"proxy_pool,omitempty" jsonschema:"Proxy pool: datacenter or residential."`
	Timeout     int    `json:"timeout,omitempty" jsonschema:"Session timeout in seconds (default 900, max 1800)."`
	BlockImages       bool   `json:"block_images,omitempty" jsonschema:"Stub image requests with empty responses."`
	BlockStyles       bool   `json:"block_styles,omitempty" jsonschema:"Stub stylesheet requests with empty responses."`
	BlockFonts        bool   `json:"block_fonts,omitempty" jsonschema:"Stub font requests with empty responses."`
	BlockMedia        bool   `json:"block_media,omitempty" jsonschema:"Stub video/audio requests with empty responses."`
	Blacklist         bool   `json:"blacklist,omitempty" jsonschema:"Stub known analytics, tracking, and telemetry URLs with empty responses."`
	Cache             bool   `json:"cache,omitempty" jsonschema:"Cache static resources (CSS, JS, fonts, images)."`
	OptimizeBandwidth bool   `json:"optimize_bandwidth,omitempty" jsonschema:"Enable all bandwidth optimizations (block images, styles, fonts, media, trackers + cache). Shortcut for setting all stub and cache options to true."`
	Debug             bool   `json:"debug,omitempty" jsonschema:"Enable session recording for replay."`
}

type CloudBrowserScreenshotInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Browser session ID. If omitted, uses the most recent session."`
	FullPage  bool   `json:"full_page,omitempty" jsonschema:"Capture the full scrollable page, not just the viewport. Default: false."`
	Selector  string `json:"selector,omitempty" jsonschema:"CSS selector of an element to screenshot. If provided, only that element is captured."`
}

type CloudBrowserEvalInput struct {
	Expression string `json:"expression" jsonschema:"JavaScript expression to evaluate in the browser page."`
	SessionID  string `json:"session_id,omitempty" jsonschema:"Browser session ID. If omitted, uses the most recent session."`
}

type CloudBrowserSnapshotInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Browser session ID. If omitted, uses the most recent session."`
}

type CloudBrowserPerformanceInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Browser session ID. If omitted, uses the most recent session."`
	Preset    string `json:"preset,omitempty" jsonschema:"Throttling preset: 'mobile' (default, Moto G4 + slow 4G + 4x CPU) or 'desktop' (1350x940 wired, no CPU throttle). Matches PSI mobile/desktop views."`
	TimeoutMs int    `json:"timeout_ms,omitempty" jsonschema:"Total budget for the lab run in ms (default 30000, max 45000)."`
}

type CloudBrowserCloseInput struct {
	SessionID string `json:"session_id" jsonschema:"Cloud Browser session ID to terminate."`
}

type CloudBrowserDownloadsInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Browser session ID. If omitted, uses the most recent session."`
	Filename  string `json:"filename,omitempty" jsonschema:"Retrieve a specific file by name. If omitted, lists all downloads."`
}

type CloudBrowserNavigateInput struct {
	SessionID string `json:"session_id" jsonschema:"Active Cloud Browser session ID."`
	URL       string `json:"url" jsonschema:"URL to navigate to."`
}

type BrowserUnblockInput struct {
	URL     string `json:"url" jsonschema:"Target URL to open with anti-bot bypass."`
	Country string `json:"country,omitempty" jsonschema:"Proxy country. ISO 3166-1 alpha-2: 'US', 'DE'. Comma-separated for multiple: 'fr,us,es,de'. Prefix '-' to exclude: '-ru'."`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Session timeout in seconds (default 900, max 1800)."`
}

// ── Tool handlers ───────────────────────────────────────────────────────────

func (p *ScrapflyToolProvider) CloudBrowserOpen(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CloudBrowserOpenInput,
) (*mcp.CallToolResult, any, error) {
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("cloud_browser_open", err), nil, nil
	}

	p.logger.Printf("Opening cloud browser for %s (enable_mcp=true)", input.URL)

	// Close any existing sessions + release from pool before allocating a new one
	browser.Store.Range(func(key, value any) bool {
		s := value.(*browser.Session)
		sid := key.(string)
		s.Close()
		if stopErr := client.CloudBrowserSessionStop(sid); stopErr != nil {
			p.logger.Printf("cloud_browser_open: releasing old session %s failed (non-fatal): %v", sid, stopErr)
		}
		return true
	})

	timeout := input.Timeout
	if timeout == 0 {
		timeout = 900
	}

	// Expand optimize_bandwidth shortcut
	if input.OptimizeBandwidth {
		input.BlockImages = true
		input.BlockStyles = true
		input.BlockFonts = true
		input.BlockMedia = true
		input.Blacklist = true
		input.Cache = true
	}

	// Connect to browser via WebSocket CDP, navigate, discover WebMCP tools.
	browserConfig := &scrapfly.CloudBrowserConfig{
		ProxyPool:   input.ProxyPool,
		Country:     input.Country,
		BlockImages: input.BlockImages,
		BlockStyles: input.BlockStyles,
		BlockFonts:  input.BlockFonts,
		BlockMedia:  input.BlockMedia,
		Blacklist:   input.Blacklist,
		Cache:       input.Cache,
		Debug:       input.Debug,
		Timeout:     timeout,
		EnableMCP:   true,
	}
	wsURL := client.CloudBrowser(browserConfig)
	p.logger.Printf("cloud_browser_open: connecting to %s", wsURL)

	// Connect via WebSocket CDP (15s timeout for allocation)
	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
		HandshakeTimeout: 15 * time.Second,
	}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		p.logger.Printf("cloud_browser_open: WebSocket connect failed: %v", err)
		return ToolErrFromError("cloud_browser_open", fmt.Errorf("browser connection failed: %w", err)), nil, nil
	}

	p.logger.Printf("cloud_browser_open: connected (local=%s remote=%s)", conn.LocalAddr(), conn.RemoteAddr())
	session := &browser.Session{
		WSURL:     wsURL,
		ExpiresAt: time.Now().Add(time.Duration(timeout) * time.Second),
		CdpConn:   conn,
	}

	// Start the CDP multiplexer — must be before any SendCDP calls
	session.StartReader()

	// Get browser targets to find the page
	targetsResult, err := session.SendCDP("Target.getTargets", nil)
	if err != nil {
		conn.Close()
		return ToolErrFromError("cloud_browser_open", fmt.Errorf("get targets failed: %w", err)), nil, nil
	}
	var targets struct {
		TargetInfos []struct {
			TargetID string `json:"targetId"`
			Type     string `json:"type"`
			URL      string `json:"url"`
		} `json:"targetInfos"`
	}
	json.Unmarshal(targetsResult, &targets)
	var pageTargetID string
	for _, t := range targets.TargetInfos {
		if t.Type == "page" {
			pageTargetID = t.TargetID
			break
		}
	}
	if pageTargetID == "" {
		conn.Close()
		return ToolErrFromError("cloud_browser_open", fmt.Errorf("no page target found")), nil, nil
	}

	// Attach to the page target
	attachResult, err := session.SendCDP("Target.attachToTarget", map[string]any{
		"targetId": pageTargetID,
		"flatten":  true,
	})
	if err != nil {
		conn.Close()
		return ToolErrFromError("cloud_browser_open", fmt.Errorf("attach failed: %w", err)), nil, nil
	}
	var attach struct {
		SessionId string `json:"sessionId"`
	}
	json.Unmarshal(attachResult, &attach)
	session.SessionID = attach.SessionId
	session.CdpPageSessionID = attach.SessionId
	p.logger.Printf("cloud_browser_open: attached to page target %s, sessionId=%s", pageTargetID, attach.SessionId)

	// Enable WebMCP + Accessibility before navigation so the domain is active
	// when page JavaScript registers tools via navigator.modelContext.registerTool().
	// Register persistent event handlers to store/remove tools on the PageState.
	session.OnEvent("WebMCP.toolsAdded", func(method string, params json.RawMessage) bool {
		var event struct {
			Tools []browser.WebMCPToolInfo `json:"tools"`
		}
		if json.Unmarshal(params, &event) == nil {
			session.Page.AddWebMCPTools(event.Tools)
			p.logger.Printf("[WebMCP] toolsAdded: %d tools", len(event.Tools))
		}
		return true // keep listening
	})
	session.OnEvent("WebMCP.toolsRemoved", func(method string, params json.RawMessage) bool {
		var event struct {
			Tools []struct{ Name string `json:"name"` } `json:"tools"`
		}
		if json.Unmarshal(params, &event) == nil {
			names := make([]string, len(event.Tools))
			for i, t := range event.Tools {
				names[i] = t.Name
			}
			session.Page.RemoveWebMCPTools(names)
			p.logger.Printf("[WebMCP] toolsRemoved: %d tools", len(names))
		}
		return true
	})
	session.SendCDP("WebMCP.enable", nil)
	session.SendCDP("Accessibility.enable", nil)

	// Navigate to the target URL
	_, err = session.SendCDP("Page.navigate", map[string]any{
		"url": input.URL,
	})
	if err != nil {
		p.logger.Printf("cloud_browser_open: navigate failed (non-fatal): %v", err)
	}

	// Wait for page load + JS execution
	time.Sleep(2 * time.Second)

	// Store session — static tools (click, fill, etc.) use browser.FindSession("") to locate it
	browser.Store.Store(pageTargetID, session)
	p.logger.Printf("cloud_browser_open: session %s stored for %s", pageTargetID, input.URL)

	// Auto-cleanup: close the session when the timeout expires.
	cleanupTimer := time.AfterFunc(time.Until(session.ExpiresAt), func() {
		if val, ok := browser.Store.Load(pageTargetID); ok {
			s := val.(*browser.Session)
			s.Close()
			p.logger.Printf("Auto-closed expired browser session %s", pageTargetID)
		}
	})
	session.CancelCleanup = func() { cleanupTimer.Stop() }

	// Build response
	response := map[string]any{
		"session_id": pageTargetID,
		"status":     "connected",
		"url":        input.URL,
		"mode":       "direct",
	}
	response["instructions"] = fmt.Sprintf(
		"[BROWSER MODE ACTIVE on %s] "+
			"FIRST: check the page snapshot below — if the page title or content looks like a challenge/captcha/block page (e.g. 'Just a moment', 'Verify you are human', 'Access denied'), close this session with cloud_browser_close and retry with cloud_browser_open(url, unblock=true). "+
			"Use click/fill/type_text/hover/press_key/scroll for interaction. "+
			"Use list_webmcp_tools to discover page-specific actions, then call_webmcp_tool to execute them. "+
			"Use take_snapshot for page content, take_screenshot for visual capture. "+
			"NEVER use standalone screenshot/web_scrape/web_get_page during browser session. "+
			"KEEP THE SESSION OPEN across follow-up turns — the user may ask more questions about this page. "+
			"Only call cloud_browser_close when the user explicitly asks to close, navigates to an unrelated site, or says they are done.",
		input.URL)

	// Refresh page state and include snapshot in response
	session.Page.Refresh(session)

	// Make the per-session interaction tools (click, fill,
	// take_snapshot, evaluate_script, ...) visible in tools/list
	// now that a browser is open. Fires
	// notifications/tools/list_changed automatically so connected
	// MCP clients refetch and the LLM sees the new surface.
	p.mountInteractionTools()

	b, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b) + "\n\n" + session.Page.Snapshot()}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserClose(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CloudBrowserCloseInput,
) (*mcp.CallToolResult, any, error) {
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("cloud_browser_close", err), nil, nil
	}

	p.logger.Printf("Closing cloud browser session %s", input.SessionID)

	// Close WebSocket and clean up session
	if val, ok := browser.Store.Load(input.SessionID); ok {
		session := val.(*browser.Session)
		session.SendCDP("WebMCP.disable", nil)
		session.Close() // closes WebSocket, removes from Store, cancels cleanup timer
		p.logger.Printf("Closed session %s", input.SessionID)
	}

	// Best-effort: also call the API stop endpoint
	if err := client.CloudBrowserSessionStop(input.SessionID); err != nil {
		p.logger.Printf("cloud_browser_close: API stop call failed (non-fatal, WebSocket already closed): %v", err)
	}

	// Hide the per-session interaction tools from tools/list now
	// that no browser is open. Fires notifications/tools/list_changed
	// so connected MCP clients refetch and stop offering them.
	p.unmountInteractionTools()

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Session %s closed successfully.", input.SessionID)}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserSessions(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input struct{},
) (*mcp.CallToolResult, any, error) {
	var sessions []map[string]any
	browser.Store.Range(func(key, value any) bool {
		s := value.(*browser.Session)
		sessions = append(sessions, map[string]any{
			"session_id": key,
			"ws_url":     s.WSURL,
			"page_url":   s.Page.URL,
			"expires_at": s.ExpiresAt.Format(time.RFC3339),
			"active":     time.Now().Before(s.ExpiresAt),
		})
		return true
	})
	b, _ := json.MarshalIndent(map[string]any{"sessions": sessions}, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserScreenshot(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CloudBrowserScreenshotInput,
) (*mcp.CallToolResult, any, error) {
	session, err := browser.FindSession(input.SessionID)
	if err != nil {
		return ToolErrf("cloud_browser_screenshot: %v", err), nil, nil
	}

	// CDP Page.captureScreenshot
	params := map[string]any{
		"format":           "png",
		"optimizeForSpeed": true,
	}
	if input.FullPage {
		params["captureBeyondViewport"] = true
	}

	// Element screenshot: get bounding box via JS, then set clip
	if input.Selector != "" {
		boxResult, boxErr := session.SendCDP("Runtime.evaluate", map[string]any{
			"expression": fmt.Sprintf(`JSON.stringify((function() {
				var el = document.querySelector(%q);
				if (!el) return null;
				var r = el.getBoundingClientRect();
				return {x: r.x, y: r.y, width: r.width, height: r.height};
			})())`, input.Selector),
			"returnByValue": true,
		})
		if boxErr == nil {
			var evalRes struct{ Result struct{ Value string `json:"value"` } `json:"result"` }
			json.Unmarshal(boxResult, &evalRes)
			var box struct{ X, Y, Width, Height float64 }
			if json.Unmarshal([]byte(evalRes.Result.Value), &box) == nil && box.Width > 0 {
				params["clip"] = map[string]any{
					"x": box.X, "y": box.Y, "width": box.Width, "height": box.Height, "scale": 1,
				}
			}
		}
	}

	result, err := session.SendCDP("Page.captureScreenshot", params)
	if err != nil {
		return ToolErrf("cloud_browser_screenshot: %v", err), nil, nil
	}

	var screenshot struct {
		Data string `json:"data"` // base64-encoded PNG
	}
	json.Unmarshal(result, &screenshot)

	if screenshot.Data == "" {
		return ToolErrf("cloud_browser_screenshot: empty screenshot data"), nil, nil
	}

	// Return a TextContent sidecar alongside the image. ADK's
	// mcptoolset drops image-only results and errors with "no text
	// content in tool response"; the text makes the call succeed for
	// the LLM while UI clients still receive the PNG.
	textSummary := fmt.Sprintf("Screenshot captured (PNG, %d bytes base64).", len(screenshot.Data))
	if input.Selector != "" {
		textSummary = fmt.Sprintf("Screenshot of %q captured (PNG, %d bytes base64).", input.Selector, len(screenshot.Data))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: textSummary},
			&mcp.ImageContent{
				Data:     []byte(screenshot.Data),
				MIMEType: "image/png",
			},
		},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserEval(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CloudBrowserEvalInput,
) (*mcp.CallToolResult, any, error) {
	session, err := browser.FindSession(input.SessionID)
	if err != nil {
		return ToolErrf("cloud_browser_eval: %v", err), nil, nil
	}
	result, err := session.SendCDP("Runtime.evaluate", map[string]any{
		"expression":    input.Expression,
		"returnByValue": true,
	})
	if err != nil {
		return ToolErrf("cloud_browser_eval: %v", err), nil, nil
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
		return ToolErrf("cloud_browser_eval error: %s", evalResult.ExceptionDetails.Text), nil, nil
	}
	b, _ := json.MarshalIndent(evalResult.Result.Value, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserSnapshot(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CloudBrowserSnapshotInput,
) (*mcp.CallToolResult, any, error) {
	session, err := browser.FindSession(input.SessionID)
	if err != nil {
		return ToolErrf("cloud_browser_snapshot: %v", err), nil, nil
	}
	session.Page.Refresh(session)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: session.Page.Snapshot()}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserPerformance(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CloudBrowserPerformanceInput,
) (*mcp.CallToolResult, any, error) {
	session, err := browser.FindSession(input.SessionID)
	if err != nil {
		return ToolErrf("cloud_browser_performance: %v", err), nil, nil
	}
	report, err := browser.CollectPSI(session, browser.PSIOptions{
		Preset:    browser.Preset(input.Preset),
		TimeoutMs: input.TimeoutMs,
	})
	if err != nil {
		return ToolErrf("cloud_browser_performance: %v", err), nil, nil
	}
	p.logger.Printf("[PSI] %s — %s", session.SessionID, browser.SummarizeReport(report))
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: browser.FormatReport(report)}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserDownloads(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CloudBrowserDownloadsInput,
) (*mcp.CallToolResult, any, error) {
	session, err := browser.FindSession(input.SessionID)
	if err != nil {
		return ToolErrf("cloud_browser_downloads: %v", err), nil, nil
	}

	if input.Filename != "" {
		// Retrieve a specific file
		data, err := session.GetDownload(input.Filename)
		if err != nil {
			return ToolErrf("cloud_browser_downloads: %v", err), nil, nil
		}
		// Return as a resource/blob for the client to download
		result := map[string]any{
			"filename": input.Filename,
			"data":     data,
			"encoding": "base64",
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	}

	// List all downloads
	downloads, err := session.ListDownloads()
	if err != nil {
		return ToolErrf("cloud_browser_downloads: %v", err), nil, nil
	}
	if len(downloads) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "No files downloaded yet."}},
		}, nil, nil
	}

	result := map[string]any{"downloads": downloads}
	b, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserNavigate(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CloudBrowserNavigateInput,
) (*mcp.CallToolResult, any, error) {
	_, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("cloud_browser_navigate", err), nil, nil
	}

	session, err2 := browser.FindSession(input.SessionID)
	if err2 != nil {
		return ToolErrf("cloud_browser_navigate: %v", err2), nil, nil
	}

	p.logger.Printf("Navigating session %s to %s", input.SessionID, input.URL)

	// Navigate via CDP
	_, err2 = session.SendCDP("Page.navigate", map[string]any{"url": input.URL})
	if err2 != nil {
		return ToolErrf("cloud_browser_navigate: navigation failed: %v", err2), nil, nil
	}

	// Wait for page load + JS execution
	time.Sleep(2 * time.Second)

	// Clear old page tools + re-enable WebMCP on the new page
	// (toolsAdded event handler from cloud_browser_open will repopulate)
	session.Page.ClearWebMCPTools()
	session.SendCDP("WebMCP.disable", nil)
	session.SendCDP("WebMCP.enable", nil)

	// Refresh page state and include snapshot
	session.Page.Refresh(session)
	navigateResult := map[string]any{
		"session_id": input.SessionID,
		"url":        input.URL,
		"status":     "navigated",
	}
	navigateResult["instructions"] = "Use list_webmcp_tools to discover page-specific actions on the new page."
	b, _ := json.MarshalIndent(navigateResult, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b) + "\n\n" + session.Page.Snapshot()}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) BrowserUnblock(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input BrowserUnblockInput,
) (*mcp.CallToolResult, any, error) {
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("browser_unblock", err), nil, nil
	}

	timeout := input.Timeout
	if timeout == 0 {
		timeout = 900
	}

	p.logger.Printf("[browser_unblock] START url=%s country=%s timeout=%d", input.URL, input.Country, timeout)

	// Step 0: Close any existing browser session AND release from pool.
	// Must call API stop endpoint to free the pool slot, not just close WebSocket.
	sessionCount := 0
	browser.Store.Range(func(key, value any) bool {
		s := value.(*browser.Session)
		sid := key.(string)
		p.logger.Printf("[browser_unblock] Step 0: closing + releasing session %s", sid)
		s.Close()
		if err := client.CloudBrowserSessionStop(sid); err != nil {
			p.logger.Printf("[browser_unblock] Step 0: stop API call for %s failed (non-fatal): %v", sid, err)
		} else {
			p.logger.Printf("[browser_unblock] Step 0: session %s released from pool", sid)
		}
		sessionCount++
		return true
	})
	p.logger.Printf("[browser_unblock] Step 0: closed %d existing sessions", sessionCount)
	// Give the pool a moment to reclaim the slot
	time.Sleep(1 * time.Second)

	// Step 1: Call unblock API
	p.logger.Printf("[browser_unblock] Step 1: calling unblock API for %s", input.URL)
	result, err := client.CloudBrowserUnblock(scrapfly.UnblockConfig{
		URL:            input.URL,
		Country:        input.Country,
		BrowserTimeout: timeout,
		EnableMCP:      true,
	})
	if err != nil {
		p.logger.Printf("[browser_unblock] Step 1 FAILED: %v", err)
		return ToolErrFromError("browser_unblock", err), nil, nil
	}
	p.logger.Printf("[browser_unblock] Step 1 OK: session_id=%s ws_url=%s", result.SessionID, result.WSURL)

	// Step 2: Connect to the unblock browser via internal service (bypass Traefik).
	// Use the same internal cloud-browser service as cloud_browser_open.
	browserConfig := &scrapfly.CloudBrowserConfig{
		Session: result.SessionID,
		Timeout: timeout,
	}
	internalWSURL := client.CloudBrowser(browserConfig)
	p.logger.Printf("[browser_unblock] Step 2: connecting CDP WebSocket to %s (internal, bypassing proxy)", internalWSURL)
	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
		HandshakeTimeout: 15 * time.Second,
	}
	conn, _, err := dialer.Dial(internalWSURL, nil)
	if err != nil {
		p.logger.Printf("[browser_unblock] Step 2 FAILED: %v", err)
		return ToolErrFromError("browser_unblock", fmt.Errorf("browser connection failed: %w", err)), nil, nil
	}
	p.logger.Printf("[browser_unblock] Step 2 OK: WebSocket connected (local=%s remote=%s)", conn.LocalAddr(), conn.RemoteAddr())

	session := &browser.Session{
		SessionID: result.SessionID,
		WSURL:     result.WSURL,
		ExpiresAt: time.Now().Add(time.Duration(timeout) * time.Second),
		CdpConn:   conn,
	}
	session.StartReader()

	// Step 3: Attach to page target
	p.logger.Printf("[browser_unblock] Step 3: finding page target")
	targetsResult, err := session.SendCDP("Target.getTargets", nil)
	if err != nil {
		p.logger.Printf("[browser_unblock] Step 3 FAILED: getTargets: %v", err)
		conn.Close()
		return ToolErrFromError("browser_unblock", fmt.Errorf("get targets failed: %w", err)), nil, nil
	}
	var targets struct {
		TargetInfos []struct {
			TargetID string `json:"targetId"`
			Type     string `json:"type"`
		} `json:"targetInfos"`
	}
	json.Unmarshal(targetsResult, &targets)
	var pageTargetID string
	for _, t := range targets.TargetInfos {
		if t.Type == "page" {
			pageTargetID = t.TargetID
			break
		}
	}
	if pageTargetID == "" {
		p.logger.Printf("[browser_unblock] Step 3 FAILED: no page target found in %d targets", len(targets.TargetInfos))
		conn.Close()
		return ToolErrFromError("browser_unblock", fmt.Errorf("no page target found")), nil, nil
	}
	p.logger.Printf("[browser_unblock] Step 3 OK: page target %s", pageTargetID)

	attachResult, err := session.SendCDP("Target.attachToTarget", map[string]any{
		"targetId": pageTargetID,
		"flatten":  true,
	})
	if err != nil {
		conn.Close()
		return ToolErrFromError("browser_unblock", fmt.Errorf("attach failed: %w", err)), nil, nil
	}
	var attach struct {
		SessionId string `json:"sessionId"`
	}
	json.Unmarshal(attachResult, &attach)
	session.CdpPageSessionID = attach.SessionId
	p.logger.Printf("[browser_unblock] Step 3 OK: attached sessionId=%s", attach.SessionId)

	// Step 4: Enable WebMCP + Accessibility
	p.logger.Printf("[browser_unblock] Step 4: enabling WebMCP + Accessibility")
	session.OnEvent("WebMCP.toolsAdded", func(method string, params json.RawMessage) bool {
		var event struct {
			Tools []browser.WebMCPToolInfo `json:"tools"`
		}
		if json.Unmarshal(params, &event) == nil {
			session.Page.AddWebMCPTools(event.Tools)
			p.logger.Printf("[WebMCP] toolsAdded: %d tools", len(event.Tools))
		}
		return true
	})
	session.OnEvent("WebMCP.toolsRemoved", func(method string, params json.RawMessage) bool {
		var event struct {
			Tools []struct{ Name string `json:"name"` } `json:"tools"`
		}
		if json.Unmarshal(params, &event) == nil {
			names := make([]string, len(event.Tools))
			for i, t := range event.Tools {
				names[i] = t.Name
			}
			session.Page.RemoveWebMCPTools(names)
		}
		return true
	})
	session.SendCDP("WebMCP.enable", nil)
	session.SendCDP("Accessibility.enable", nil)

	// Navigate to the target URL — the browser starts on a blank tab with cookies pre-loaded
	p.logger.Printf("[browser_unblock] Step 4: navigating to %s", input.URL)
	_, err = session.SendCDP("Page.navigate", map[string]any{"url": input.URL})
	if err != nil {
		p.logger.Printf("[browser_unblock] Step 4: navigate failed (non-fatal): %v", err)
	}

	// Wait for page load + JS execution
	p.logger.Printf("[browser_unblock] Step 4 OK: waiting for page load")
	time.Sleep(2 * time.Second)

	// Step 5: Store session + auto-cleanup
	p.logger.Printf("[browser_unblock] Step 5: storing session %s", result.SessionID)
	browser.Store.Store(result.SessionID, session)
	cleanupTimer := time.AfterFunc(time.Until(session.ExpiresAt), func() {
		if val, ok := browser.Store.Load(result.SessionID); ok {
			s := val.(*browser.Session)
			s.Close()
			p.logger.Printf("Auto-closed expired browser session %s", result.SessionID)
		}
	})
	session.CancelCleanup = func() { cleanupTimer.Stop() }

	// Step 6: Build response with snapshot
	p.logger.Printf("[browser_unblock] Step 6: refreshing page state and building response")
	session.Page.Refresh(session)
	p.logger.Printf("[browser_unblock] DONE: session=%s url=%s title=%s", result.SessionID, session.Page.URL, session.Page.Title)
	response := map[string]any{
		"session_id": result.SessionID,
		"status":     "connected",
		"url":        input.URL,
		"mode":       "unblock",
	}
	response["instructions"] = fmt.Sprintf(
		"[BROWSER MODE ACTIVE on %s — anti-bot bypassed] "+
			"The page is already loaded — do NOT navigate to the same URL again. "+
			"Use click/fill/type_text/hover/press_key/scroll for interaction. "+
			"Use list_webmcp_tools to discover page-specific actions. "+
			"Do NOT call browser_unblock or cloud_browser_open again while this session is active. "+
			"KEEP THE SESSION OPEN across follow-up turns — the user may ask more questions about this page. "+
			"Only call cloud_browser_close when the user explicitly asks to close, navigates to an unrelated site, or says they are done.",
		input.URL)

	// Make the per-session interaction tools visible — same as
	// cloud_browser_open. See mountInteractionTools docstring.
	p.mountInteractionTools()

	b, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b) + "\n\n" + session.Page.Snapshot()}},
	}, nil, nil
}

// proxyWebMCPToolCallCDP dispatches WebMCP tool calls via CDP.
// - Antibot tools (fill, clickOn, etc.) use WebMCP.callTool (Scrapium's custom handler)
// - Page-registered tools (searchProducts, etc.) use WebMCP.invokeTool + toolResponded event
