package scrapflyprovider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
)

// browserSession tracks a live Cloud Browser session with an active CDP WebSocket.
type browserSession struct {
	SessionID       string
	MCPEndpoint     string
	WSURL           string
	ToolNames       []string        // namespaced tool names registered on the MCP server
	ExpiresAt       time.Time       // browser timeout
	cdpConn         *websocket.Conn // live CDP WebSocket connection
	cdpMu           sync.Mutex      // protects cdpConn writes
	cdpID           atomic.Int64    // CDP message ID counter
	cdpPageSessionID string         // flattened session ID for page-level CDP commands
}

// cdpSessionID is the flattened session ID for page-level CDP commands.
// Set after Target.attachToTarget. Empty means browser-level.
// stored directly on browserSession

// sendCDP sends a CDP command and waits for the response with matching ID.
// If the session has a cdpSessionID, commands are scoped to that page session.
func (s *browserSession) sendCDP(method string, params any) (json.RawMessage, error) {
	id := s.cdpID.Add(1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if s.cdpPageSessionID != "" {
		msg["sessionId"] = s.cdpPageSessionID
	}

	s.cdpMu.Lock()
	err := s.cdpConn.WriteJSON(msg)
	s.cdpMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("CDP write: %w", err)
	}

	// Read until we get the response with our ID (skip events)
	for i := 0; i < 100; i++ {
		_, raw, err := s.cdpConn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("CDP read: %w", err)
		}
		var resp struct {
			ID     int64           `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Method string `json:"method"` // for events
		}
		json.Unmarshal(raw, &resp)
		if resp.ID == id {
			if resp.Error != nil {
				return nil, fmt.Errorf("CDP error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			return resp.Result, nil
		}
		// It's an event — skip it
	}
	return nil, fmt.Errorf("CDP response timeout: no response for ID %d after 100 messages", id)
}

// sendCDPCollectEvents sends a CDP command and collects events until the response arrives.
func (s *browserSession) sendCDPCollectEvents(method string, params any, eventName string) (json.RawMessage, []json.RawMessage, error) {
	id := s.cdpID.Add(1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if s.cdpPageSessionID != "" {
		msg["sessionId"] = s.cdpPageSessionID
	}

	s.cdpMu.Lock()
	err := s.cdpConn.WriteJSON(msg)
	s.cdpMu.Unlock()
	if err != nil {
		return nil, nil, fmt.Errorf("CDP write: %w", err)
	}

	var events []json.RawMessage
	for i := 0; i < 100; i++ {
		_, raw, err := s.cdpConn.ReadMessage()
		if err != nil {
			return nil, events, fmt.Errorf("CDP read: %w", err)
		}
		var resp struct {
			ID     int64           `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.Unmarshal(raw, &resp)
		if resp.ID == id {
			if resp.Error != nil {
				return nil, events, fmt.Errorf("CDP error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			return resp.Result, events, nil
		}
		if resp.Method == eventName {
			events = append(events, resp.Params)
		}
	}
	return nil, events, fmt.Errorf("CDP timeout for ID %d", id)
}

// browserSessionStore is a per-provider in-memory store of active browser sessions.
// Thread-safe via sync.Map. Keyed by session_id.
var browserSessionStore sync.Map

// ── Tool inputs ─────────────────────────────────────────────────────────────

type CloudBrowserOpenInput struct {
	URL         string `json:"url" jsonschema:"Target URL to open in the cloud browser."`
	Country     string `json:"country,omitempty" jsonschema:"Proxy country code (ISO 3166-1 alpha-2)."`
	ProxyPool   string `json:"proxy_pool,omitempty" jsonschema:"Proxy pool: datacenter or residential."`
	Timeout     int    `json:"timeout,omitempty" jsonschema:"Session timeout in seconds (default 900, max 1800)."`
	BlockImages bool   `json:"block_images,omitempty" jsonschema:"Stub image requests to save bandwidth."`
	BlockStyles bool   `json:"block_styles,omitempty" jsonschema:"Stub stylesheet requests."`
	BlockMedia  bool   `json:"block_media,omitempty" jsonschema:"Stub video/audio requests."`
	Cache       bool   `json:"cache,omitempty" jsonschema:"Enable HTTP cache for static resources."`
	Debug       bool   `json:"debug,omitempty" jsonschema:"Enable session recording for replay."`
	Unblock     bool   `json:"unblock,omitempty" jsonschema:"Use anti-bot bypass (ASP). Only use if the site blocks normal access."`
}

type CloudBrowserScreenshotInput struct {
	SessionID string `json:"session_id,omitempty" jsonschema:"Browser session ID. If omitted, uses the most recent session."`
}

type CloudBrowserCloseInput struct {
	SessionID string `json:"session_id" jsonschema:"Cloud Browser session ID to terminate."`
}

type CloudBrowserNavigateInput struct {
	SessionID string `json:"session_id" jsonschema:"Active Cloud Browser session ID."`
	URL       string `json:"url" jsonschema:"URL to navigate to."`
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

	timeout := input.Timeout
	if timeout == 0 {
		timeout = 900
	}

	if input.Unblock {
		// Anti-bot bypass path: POST /unblock → navigates, bypasses protection,
		// returns session with cookies pre-loaded.
		p.logger.Printf("cloud_browser_open (unblock mode) for %s", input.URL)
		result, err := client.CloudBrowserUnblock(scrapfly.UnblockConfig{
			URL:            input.URL,
			Country:        input.Country,
			BrowserTimeout: timeout,
			EnableMCP:      true,
		})
		if err != nil {
			p.logger.Printf("cloud_browser_open unblock failed for %s: %v", input.URL, err)
			return ToolErrFromError("cloud_browser_open", err), nil, nil
		}
		p.logger.Printf("cloud_browser_open unblock succeeded: session_id=%s ws_url=%s mcp_endpoint=%s",
			result.SessionID, result.WSURL, result.MCPEndpoint)

		session := &browserSession{
			SessionID:   result.SessionID,
			MCPEndpoint: result.MCPEndpoint,
			WSURL:       result.WSURL,
			ExpiresAt:   time.Now().Add(time.Duration(timeout) * time.Second),
		}

		// Discover WebMCP tools from Chrome's MCP endpoint
		var discoveredTools []mcpToolInfo
		if result.MCPEndpoint != "" {
			discoveredTools = discoverWebMCPTools(p, result.MCPEndpoint, result.SessionID)
			session.ToolNames = make([]string, len(discoveredTools))
			for i, t := range discoveredTools {
				session.ToolNames[i] = t.NamespacedName
			}
		}

		browserSessionStore.Store(result.SessionID, session)

		response := map[string]any{
			"session_id":   result.SessionID,
			"ws_url":       result.WSURL,
			"run_id":       result.RunID,
			"mcp_endpoint": result.MCPEndpoint,
			"mode":         "unblock",
		}
		if len(discoveredTools) > 0 {
			toolList := make([]map[string]string, len(discoveredTools))
			for i, t := range discoveredTools {
				toolList[i] = map[string]string{
					"tool_name":   t.NamespacedName,
					"description": t.Description,
				}
			}
			response["webmcp_tools"] = toolList
		}
		b, _ := json.MarshalIndent(response, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	}

	// Default path: connect to browser via WebSocket CDP, navigate, discover WebMCP tools.
	browserConfig := &scrapfly.CloudBrowserConfig{
		ProxyPool:   input.ProxyPool,
		Country:     input.Country,
		BlockImages: input.BlockImages,
		BlockStyles: input.BlockStyles,
		BlockMedia:  input.BlockMedia,
		Cache:       input.Cache,
		Debug:       input.Debug,
		Timeout:     timeout,
		EnableMCP:   true,
	}
	wsURL := client.CloudBrowser(browserConfig)
	p.logger.Printf("cloud_browser_open: connecting to %s", wsURL)

	// Connect via WebSocket CDP
	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		p.logger.Printf("cloud_browser_open: WebSocket connect failed: %v", err)
		return ToolErrFromError("cloud_browser_open", fmt.Errorf("browser connection failed: %w", err)), nil, nil
	}

	session := &browserSession{
		WSURL:     wsURL,
		ExpiresAt: time.Now().Add(time.Duration(timeout) * time.Second),
		cdpConn:   conn,
	}

	// Get browser targets to find the page
	targetsResult, err := session.sendCDP("Target.getTargets", nil)
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
	attachResult, err := session.sendCDP("Target.attachToTarget", map[string]any{
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
	session.cdpPageSessionID = attach.SessionId
	p.logger.Printf("cloud_browser_open: attached to page target %s, sessionId=%s", pageTargetID, attach.SessionId)

	// Navigate to the target URL
	_, err = session.sendCDP("Page.navigate", map[string]any{
		"url": input.URL,
	})
	if err != nil {
		p.logger.Printf("cloud_browser_open: navigate failed (non-fatal): %v", err)
	}
	// Wait for page load
	time.Sleep(3 * time.Second)

	// Enable WebMCP to discover Scrapium's Antibot tools
	_, toolEvents, err := session.sendCDPCollectEvents("WebMCP.enable", nil, "WebMCP.toolsAdded")
	if err != nil {
		p.logger.Printf("cloud_browser_open: WebMCP.enable failed: %v", err)
	}

	// Parse discovered tools from toolsAdded events
	var discoveredTools []mcpToolInfo
	shortID := pageTargetID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	for _, eventParams := range toolEvents {
		var event struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		}
		json.Unmarshal(eventParams, &event)
		for _, t := range event.Tools {
			namespacedName := fmt.Sprintf("webmcp_%s_%s", shortID, t.Name)
			discoveredTools = append(discoveredTools, mcpToolInfo{
				OriginalName:   t.Name,
				NamespacedName: namespacedName,
				Description:    t.Description,
				InputSchema:    t.InputSchema,
			})
		}
	}

	// Register discovered WebMCP tools + CDP DevTools tools on the MCP server
	if p.MCPServer != nil {
		var allToolNames []string

		// 1. Register WebMCP page tools (from Scrapium Antibot)
		for _, t := range discoveredTools {
			tool := &mcp.Tool{
				Name:        t.NamespacedName,
				Title:       fmt.Sprintf("[Browser] %s", t.OriginalName),
				Description: t.Description,
				Annotations: &mcp.ToolAnnotations{
					Title:           fmt.Sprintf("[Browser] %s", t.OriginalName),
					DestructiveHint: &falseBool,
					OpenWorldHint:   &trueBool,
				},
				Meta: standardPermissionsMeta,
			}
			if len(t.InputSchema) > 0 {
				var schema any
				if err := json.Unmarshal(t.InputSchema, &schema); err == nil {
					tool.InputSchema = schema
				}
			}
			if tool.InputSchema == nil {
				tool.InputSchema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			origName := t.OriginalName
			sid := pageTargetID
			p.MCPServer.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return proxyWebMCPToolCallCDP(p, sid, origName, req.Params.Arguments)
			})
			allToolNames = append(allToolNames, t.NamespacedName)
		}

		// 2. Register CDP DevTools tools (screenshot, evaluate_script, snapshot, get_page_content)
		registerCDPTools(p, pageTargetID, shortID, &allToolNames)

		session.ToolNames = allToolNames
	}

	p.logger.Printf("cloud_browser_open: %d WebMCP + CDP tools registered for %s", len(session.ToolNames), input.URL)
	browserSessionStore.Store(pageTargetID, session)

	// Build response
	response := map[string]any{
		"session_id": pageTargetID,
		"status":     "connected",
		"url":        input.URL,
		"mode":       "direct",
	}
	if len(discoveredTools) > 0 {
		toolList := make([]map[string]string, len(discoveredTools))
		for i, t := range discoveredTools {
			toolList[i] = map[string]string{
				"tool_name":   t.NamespacedName,
				"description": t.Description,
			}
		}
		response["browser_tools"] = toolList
		response["instructions"] = fmt.Sprintf("Browser connected to %s with %d tools available. Call them directly by name.", input.URL, len(discoveredTools))
	}

	b, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
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

	// Stop the browser session
	if err := client.CloudBrowserSessionStop(input.SessionID); err != nil {
		return ToolErrFromError("cloud_browser_close", err), nil, nil
	}

	// Clean up dynamic tools
	if val, ok := browserSessionStore.LoadAndDelete(input.SessionID); ok {
		session := val.(*browserSession)
		if p.MCPServer != nil && len(session.ToolNames) > 0 {
			p.MCPServer.RemoveTools(session.ToolNames...)
			p.logger.Printf("Removed %d WebMCP tools for session %s", len(session.ToolNames), input.SessionID)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Session %s closed successfully.", input.SessionID)}},
	}, nil, nil
}

func (p *ScrapflyToolProvider) CloudBrowserSessions(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input struct{},
) (*mcp.CallToolResult, any, error) {
	// List local active CDP sessions
	var sessions []map[string]any
	browserSessionStore.Range(func(key, value any) bool {
		s := value.(*browserSession)
		sessions = append(sessions, map[string]any{
			"session_id": key,
			"ws_url":     s.WSURL,
			"tools":      len(s.ToolNames),
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
	// Find the session
	var session *browserSession
	if input.SessionID != "" {
		val, ok := browserSessionStore.Load(input.SessionID)
		if !ok {
			return ToolErrf("cloud_browser_screenshot: session %s not found", input.SessionID), nil, nil
		}
		session = val.(*browserSession)
	} else {
		// Use the most recent session
		browserSessionStore.Range(func(key, value any) bool {
			session = value.(*browserSession)
			return false // stop at first
		})
		if session == nil {
			return ToolErrf("cloud_browser_screenshot: no active browser session"), nil, nil
		}
	}

	// CDP Page.captureScreenshot
	result, err := session.sendCDP("Page.captureScreenshot", map[string]any{
		"format": "png",
	})
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

	// CDP returns base64 string, ImageContent.Data is []byte (base64-encoded)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.ImageContent{
			Data:     []byte(screenshot.Data),
			MIMEType: "image/png",
		}},
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

	val, ok := browserSessionStore.Load(input.SessionID)
	if !ok {
		return ToolErrf("cloud_browser_navigate: session %s not found", input.SessionID), nil, nil
	}
	session := val.(*browserSession)

	if session.MCPEndpoint == "" {
		return ToolErrf("cloud_browser_navigate: session %s has no MCP endpoint", input.SessionID), nil, nil
	}

	p.logger.Printf("Navigating session %s to %s", input.SessionID, input.URL)

	// Navigate via Chrome's MCP endpoint (call the built-in navigate tool if available)
	// Fall back to returning the ws_url for CDP-based navigation
	navigateResult := map[string]any{
		"session_id": input.SessionID,
		"url":        input.URL,
		"ws_url":     session.WSURL,
	}

	// Remove old WebMCP tools
	if p.MCPServer != nil && len(session.ToolNames) > 0 {
		p.MCPServer.RemoveTools(session.ToolNames...)
	}

	// Re-discover tools on the new page (with a short delay for page load)
	time.Sleep(2 * time.Second)
	discoveredTools := discoverWebMCPTools(p, session.MCPEndpoint, session.SessionID)
	session.ToolNames = make([]string, len(discoveredTools))
	for i, t := range discoveredTools {
		session.ToolNames[i] = t.NamespacedName
	}
	browserSessionStore.Store(input.SessionID, session)

	if len(discoveredTools) > 0 {
		toolList := make([]map[string]string, len(discoveredTools))
		for i, t := range discoveredTools {
			toolList[i] = map[string]string{
				"tool_name":   t.NamespacedName,
				"description": t.Description,
			}
		}
		navigateResult["webmcp_tools"] = toolList
	}

	b, _ := json.MarshalIndent(navigateResult, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

// ── WebMCP tool discovery + dynamic registration ────────────────────────────

type mcpToolInfo struct {
	OriginalName   string
	NamespacedName string
	Description    string
	InputSchema    json.RawMessage
}

// discoverWebMCPTools calls Chrome's MCP endpoint to list available tools,
// then registers each as a dynamic tool on the Scrapfly MCP server.
func discoverWebMCPTools(p *ScrapflyToolProvider, mcpEndpoint, sessionID string) []mcpToolInfo {
	if mcpEndpoint == "" || p.MCPServer == nil {
		return nil
	}

	// Build MCP tools/list JSON-RPC request
	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	body, _ := json.Marshal(rpcReq)

	httpReq, err := http.NewRequest(http.MethodPost, mcpEndpoint, bytes.NewReader(body))
	if err != nil {
		p.logger.Printf("Failed to create MCP tools/list request: %v", err)
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		p.logger.Printf("MCP tools/list request failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logger.Printf("Failed to read MCP tools/list response: %v", err)
		return nil
	}

	// Parse JSON-RPC response
	var rpcResp struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		p.logger.Printf("Failed to parse MCP tools/list response: %v", err)
		return nil
	}
	if rpcResp.Error != nil {
		p.logger.Printf("MCP tools/list returned error: %s", rpcResp.Error.Message)
		return nil
	}

	// Generate short session prefix (first 8 chars)
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	shortID = strings.ReplaceAll(shortID, "-", "")

	var tools []mcpToolInfo
	for _, t := range rpcResp.Result.Tools {
		namespacedName := fmt.Sprintf("webmcp_%s_%s", shortID, t.Name)

		info := mcpToolInfo{
			OriginalName:   t.Name,
			NamespacedName: namespacedName,
			Description:    t.Description,
			InputSchema:    t.InputSchema,
		}
		tools = append(tools, info)

		// Build the MCP tool definition
		mcpTool := &mcp.Tool{
			Name:        namespacedName,
			Title:       fmt.Sprintf("[WebMCP] %s", t.Name),
			Description: fmt.Sprintf("WebMCP page tool from browser session %s. %s", shortID, t.Description),
			Annotations: &mcp.ToolAnnotations{
				Title:           fmt.Sprintf("[WebMCP] %s", t.Name),
				DestructiveHint: &falseBool,
				IdempotentHint:  false,
				OpenWorldHint:   &trueBool,
				ReadOnlyHint:    false,
			},
			Meta: standardPermissionsMeta,
		}

		// Parse inputSchema into the tool if available
		if len(t.InputSchema) > 0 {
			var schema any
			if err := json.Unmarshal(t.InputSchema, &schema); err == nil {
				mcpTool.InputSchema = schema
			}
		}

		// Create a proxy handler closure for this tool
		originalName := t.Name
		sid := sessionID
		handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return proxyWebMCPToolCall(p, sid, originalName, req.Params.Arguments)
		}

		p.MCPServer.AddTool(mcpTool, handler)
		p.logger.Printf("Registered WebMCP tool: %s (proxies to %s on session %s)", namespacedName, originalName, shortID)
	}

	return tools
}

// registerCDPTools adds Chrome DevTools Protocol tools (screenshot, JS eval, snapshot, etc.)
// that are NOT exposed via WebMCP but are essential for browser automation.
func registerCDPTools(p *ScrapflyToolProvider, sessionID, shortID string, toolNames *[]string) {
	prefix := fmt.Sprintf("browser_%s", shortID)

	emptySchema := map[string]any{"type": "object", "properties": map[string]any{}}

	// take_screenshot — CDP Page.captureScreenshot
	screenshotName := prefix + "_take_screenshot"
	p.MCPServer.AddTool(&mcp.Tool{
		Name:        screenshotName,
		Title:       "[Browser] Take Screenshot",
		Description: "Take a screenshot of the current page in the browser. Returns a PNG image.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: emptySchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := browserSessionStore.Load(sessionID)
		if !ok {
			return ToolErrf("take_screenshot: session not found"), nil
		}
		s := val.(*browserSession)
		result, err := s.sendCDP("Page.captureScreenshot", map[string]any{"format": "png"})
		if err != nil {
			return ToolErrf("take_screenshot: %v", err), nil
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
	p.MCPServer.AddTool(&mcp.Tool{
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
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := browserSessionStore.Load(sessionID)
		if !ok {
			return ToolErrf("evaluate_script: session not found"), nil
		}
		s := val.(*browserSession)
		var args struct {
			Expression string `json:"expression"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		if args.Expression == "" {
			return ToolErrf("evaluate_script: expression is required"), nil
		}
		result, err := s.sendCDP("Runtime.evaluate", map[string]any{
			"expression":    args.Expression,
			"returnByValue": true,
		})
		if err != nil {
			return ToolErrf("evaluate_script: %v", err), nil
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
			return ToolErrf("evaluate_script error: %s", evalResult.ExceptionDetails.Text), nil
		}
		b, _ := json.MarshalIndent(evalResult.Result.Value, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil
	})
	*toolNames = append(*toolNames, evalName)

	// take_snapshot — get page text content via DOM snapshot
	snapName := prefix + "_take_snapshot"
	p.MCPServer.AddTool(&mcp.Tool{
		Name:        snapName,
		Title:       "[Browser] Page Content Snapshot",
		Description: "Get the text content of the current page. Useful for reading page content without a screenshot. Returns the document title and body text.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: emptySchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := browserSessionStore.Load(sessionID)
		if !ok {
			return ToolErrf("take_snapshot: session not found"), nil
		}
		s := val.(*browserSession)
		result, err := s.sendCDP("Runtime.evaluate", map[string]any{
			"expression":    `JSON.stringify({title: document.title, url: location.href, text: document.body?.innerText?.substring(0, 50000) || ''})`,
			"returnByValue": true,
		})
		if err != nil {
			return ToolErrf("take_snapshot: %v", err), nil
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
	p.MCPServer.AddTool(&mcp.Tool{
		Name:        urlName,
		Title:       "[Browser] Get Current URL",
		Description: "Get the current page URL and title from the browser.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: emptySchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := browserSessionStore.Load(sessionID)
		if !ok {
			return ToolErrf("get_page_url: session not found"), nil
		}
		s := val.(*browserSession)
		result, err := s.sendCDP("Runtime.evaluate", map[string]any{
			"expression":    `JSON.stringify({url: location.href, title: document.title})`,
			"returnByValue": true,
		})
		if err != nil {
			return ToolErrf("get_page_url: %v", err), nil
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

	// get_performance_metrics — CDP Performance.getMetrics + Navigation Timing
	perfName := prefix + "_get_performance_metrics"
	p.MCPServer.AddTool(&mcp.Tool{
		Name:        perfName,
		Title:       "[Browser] Get Performance Metrics",
		Description: "Get detailed performance metrics of the current page including load times, DOM size, JS heap, layout count, and Web Vitals. Use this to analyze page performance.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		InputSchema: emptySchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		val, ok := browserSessionStore.Load(sessionID)
		if !ok {
			return ToolErrf("get_performance_metrics: session not found"), nil
		}
		s := val.(*browserSession)

		// Enable and get CDP Performance metrics
		s.sendCDP("Performance.enable", nil)
		cdpMetrics, err := s.sendCDP("Performance.getMetrics", nil)
		if err != nil {
			return ToolErrf("get_performance_metrics CDP: %v", err), nil
		}

		// Get Navigation Timing + Web Vitals via JS
		navTiming, _ := s.sendCDP("Runtime.evaluate", map[string]any{
			"expression": `JSON.stringify({
				navigation: (() => {
					const t = performance.getEntriesByType('navigation')[0];
					if (!t) return null;
					return {
						url: t.name,
						duration_ms: Math.round(t.duration),
						dns_ms: Math.round(t.domainLookupEnd - t.domainLookupStart),
						connect_ms: Math.round(t.connectEnd - t.connectStart),
						ttfb_ms: Math.round(t.responseStart - t.requestStart),
						download_ms: Math.round(t.responseEnd - t.responseStart),
						dom_interactive_ms: Math.round(t.domInteractive - t.startTime),
						dom_complete_ms: Math.round(t.domComplete - t.startTime),
						load_event_ms: Math.round(t.loadEventEnd - t.startTime),
						transfer_size_kb: Math.round(t.transferSize / 1024),
					};
				})(),
				resources: {
					count: performance.getEntriesByType('resource').length,
					total_transfer_kb: Math.round(performance.getEntriesByType('resource').reduce((s,r) => s + (r.transferSize||0), 0) / 1024),
				},
				paint: performance.getEntriesByType('paint').map(p => ({name: p.name, time_ms: Math.round(p.startTime)})),
				memory: performance.memory ? {
					used_mb: Math.round(performance.memory.usedJSHeapSize / 1048576),
					total_mb: Math.round(performance.memory.totalJSHeapSize / 1048576),
					limit_mb: Math.round(performance.memory.jsHeapSizeLimit / 1048576),
				} : null,
			})`,
			"returnByValue": true,
		})

		// Combine CDP metrics + JS timing
		var combined strings.Builder
		combined.WriteString("=== CDP Performance Metrics ===\n")
		combined.Write(cdpMetrics)
		combined.WriteString("\n\n=== Navigation Timing & Web Vitals ===\n")
		if navTiming != nil {
			var evalResult struct {
				Result struct{ Value string `json:"value"` } `json:"result"`
			}
			json.Unmarshal(navTiming, &evalResult)
			// Pretty-print the JSON
			var parsed any
			if json.Unmarshal([]byte(evalResult.Result.Value), &parsed) == nil {
				pretty, _ := json.MarshalIndent(parsed, "", "  ")
				combined.Write(pretty)
			} else {
				combined.WriteString(evalResult.Result.Value)
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: combined.String()}},
		}, nil
	})
	*toolNames = append(*toolNames, perfName)

	p.logger.Printf("Registered %d CDP DevTools tools for session %s", 5, shortID)
}

// proxyWebMCPToolCallCDP forwards a WebMCP tool call through the CDP WebSocket connection.
func proxyWebMCPToolCallCDP(p *ScrapflyToolProvider, sessionID, toolName string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	val, ok := browserSessionStore.Load(sessionID)
	if !ok {
		return ToolErrf("browser tool %s: session %s not found or expired", toolName, sessionID), nil
	}
	session := val.(*browserSession)

	if time.Now().After(session.ExpiresAt) {
		browserSessionStore.Delete(sessionID)
		return ToolErrf("browser tool %s: session %s has expired", toolName, sessionID), nil
	}

	// Build WebMCP.callTool CDP command
	params := map[string]any{"name": toolName}
	if len(arguments) > 0 {
		var args any
		json.Unmarshal(arguments, &args)
		params["arguments"] = args
	}

	result, err := session.sendCDP("WebMCP.callTool", params)
	if err != nil {
		return ToolErrf("browser tool %s: %v", toolName, err), nil
	}

	// Parse the result
	var callResult struct {
		Success bool   `json:"success"`
		Result  string `json:"result"`
		Error   string `json:"error"`
	}
	json.Unmarshal(result, &callResult)

	if !callResult.Success && callResult.Error != "" {
		return ToolErrf("browser tool %s failed: %s", toolName, callResult.Error), nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: callResult.Result}},
	}, nil
}

// proxyWebMCPToolCall forwards a tool call to Chrome's MCP endpoint (HTTP).
func proxyWebMCPToolCall(p *ScrapflyToolProvider, sessionID, toolName string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	val, ok := browserSessionStore.Load(sessionID)
	if !ok {
		return ToolErrf("webmcp proxy: session %s not found or expired", sessionID), nil
	}
	session := val.(*browserSession)

	if time.Now().After(session.ExpiresAt) {
		browserSessionStore.Delete(sessionID)
		return ToolErrf("webmcp proxy: session %s has expired", sessionID), nil
	}

	// Build MCP tools/call JSON-RPC request
	rpcReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": arguments,
		},
	}
	body, _ := json.Marshal(rpcReq)

	httpReq, err := http.NewRequest(http.MethodPost, session.MCPEndpoint, bytes.NewReader(body))
	if err != nil {
		return ToolErrf("webmcp proxy: failed to create request: %v", err), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return ToolErrf("webmcp proxy: request failed: %v", err), nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ToolErrf("webmcp proxy: failed to read response: %v", err), nil
	}

	// Parse JSON-RPC response and extract the result
	var rpcResp struct {
		Result *mcp.CallToolResult `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		// Return raw response as text if we can't parse it
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(respBody)}},
		}, nil
	}
	if rpcResp.Error != nil {
		return ToolErrf("webmcp %s: %s", toolName, rpcResp.Error.Message), nil
	}
	if rpcResp.Result != nil {
		return rpcResp.Result, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(respBody)}},
	}, nil
}
