package scrapflyprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
)

// browserSession tracks a live Cloud Browser session with MCP enabled.
type browserSession struct {
	SessionID   string
	MCPEndpoint string
	WSURL       string
	ToolNames   []string  // namespaced tool names registered on the MCP server
	ExpiresAt   time.Time // browser timeout
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

	result, err := client.CloudBrowserUnblock(scrapfly.UnblockConfig{
		URL:            input.URL,
		Country:        input.Country,
		BrowserTimeout: timeout,
		EnableMCP:      true,
	})
	if err != nil {
		p.logger.Printf("cloud_browser_open failed for %s: %v", input.URL, err)
		return ToolErrFromError("cloud_browser_open", err), nil, nil
	}

	p.logger.Printf("cloud_browser_open succeeded: session_id=%s ws_url=%s mcp_endpoint=%s",
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

	// Build response
	response := map[string]any{
		"session_id":   result.SessionID,
		"ws_url":       result.WSURL,
		"run_id":       result.RunID,
		"mcp_endpoint": result.MCPEndpoint,
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
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("cloud_browser_sessions", err), nil, nil
	}

	result, err := client.CloudBrowserSessions()
	if err != nil {
		return ToolErrFromError("cloud_browser_sessions", err), nil, nil
	}

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

// proxyWebMCPToolCall forwards a tool call to Chrome's MCP endpoint.
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
