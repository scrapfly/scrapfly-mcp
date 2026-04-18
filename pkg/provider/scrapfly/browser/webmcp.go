package browser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPToolInfo describes a discovered WebMCP tool.
type MCPToolInfo struct {
	OriginalName   string
	NamespacedName string
	Description    string
	InputSchema    json.RawMessage
	FrameID        string // "browser" for antibot tools, frame ID for page-registered tools
}

// AntibotSchemaOverride holds a schema override and optional description for antibot tools
// whose schemas Chrome returns as empty.
type AntibotSchemaOverride struct {
	Schema      any
	Description string
}

// Logger is an interface satisfied by *log.Logger, used to avoid coupling to a specific provider type.
type Logger interface {
	Printf(format string, v ...any)
}

// DiscoverTools calls Chrome's MCP endpoint to list available tools.
// It returns the discovered tool info but does NOT register them on the MCP server;
// the caller is responsible for registration.
func DiscoverTools(logger Logger, mcpEndpoint, sessionID string) []MCPToolInfo {
	if mcpEndpoint == "" {
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
		logger.Printf("Failed to create MCP tools/list request: %v", err)
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		logger.Printf("MCP tools/list request failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Printf("Failed to read MCP tools/list response: %v", err)
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
		logger.Printf("Failed to parse MCP tools/list response: %v", err)
		return nil
	}
	if rpcResp.Error != nil {
		logger.Printf("MCP tools/list returned error: %s", rpcResp.Error.Message)
		return nil
	}

	// Generate short session prefix (first 8 chars)
	shortID := sessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	shortID = strings.ReplaceAll(shortID, "-", "")

	var tools []MCPToolInfo
	for _, t := range rpcResp.Result.Tools {
		namespacedName := fmt.Sprintf("webmcp_%s_%s", shortID, t.Name)
		tools = append(tools, MCPToolInfo{
			OriginalName:   t.Name,
			NamespacedName: namespacedName,
			Description:    t.Description,
			InputSchema:    t.InputSchema,
		})
		logger.Printf("Registered WebMCP tool: %s (proxies to %s on session %s)", namespacedName, t.Name, shortID)
	}

	return tools
}

// CallTool calls an Antibot CDP command directly (Antibot.fill, Antibot.clickOn, etc.).
// For page-registered tools (navigator.modelContext), use InvokeTool instead.
func CallTool(logger Logger, session *Session, toolName string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	if time.Now().After(session.ExpiresAt) {
		return toolErrf("browser tool %s: session has expired", toolName), nil
	}

	// Parse and fix arguments — Claude may double-encode JSON objects as strings
	var params map[string]any
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &params); err != nil {
			params = map[string]any{}
		}
		for k, v := range params {
			if str, ok := v.(string); ok {
				if len(str) > 1 && str[0] == '{' {
					var parsed any
					if json.Unmarshal([]byte(str), &parsed) == nil {
						params[k] = parsed
						continue
					}
				}
				if str == "true" {
					params[k] = true
				} else if str == "false" {
					params[k] = false
				}
			}
		}
	}

	// Call Antibot.<toolName> directly via CDP
	cdpMethod := "Antibot." + toolName
	paramsJSON, _ := json.Marshal(params)
	logger.Printf("[Antibot] %s params=%s", cdpMethod, string(paramsJSON))
	result, err := session.SendCDP(cdpMethod, params)
	if err != nil {
		logger.Printf("[Antibot] %s error: %v", cdpMethod, err)
		// Remove dead session so FindSession("") won't return it again
		if strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "broken pipe") {
			session.Close()
			logger.Printf("[Antibot] Removed dead session %s", session.SessionID)
		}
		return toolErrf("browser tool %s: %v — session may have expired, use cloud_browser_open to start a new one", toolName, err), nil
	}

	logger.Printf("[Antibot] %s result: %s", cdpMethod, string(result))

	var callResult struct {
		Success      bool   `json:"success"`
		ErrorMessage string `json:"errorMessage"`
	}
	json.Unmarshal(result, &callResult)

	if !callResult.Success && callResult.ErrorMessage != "" {
		logger.Printf("[Antibot] %s failed: %s", cdpMethod, callResult.ErrorMessage)
		return toolErrf("browser tool %s failed: %s", toolName, callResult.ErrorMessage), nil
	}

	// After interaction tools, refresh page state and include snapshot
	resultText := "success"
	if toolName == "fill" || toolName == "clickOn" || toolName == "selectOption" || toolName == "pressKey" {
		time.Sleep(500 * time.Millisecond)
		session.Page.Refresh(session)
		resultText += "\n\n" + session.Page.Snapshot()
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: resultText}},
	}, nil
}

// InvokeTool forwards a WebMCP page-registered tool call via CDP (async).
// Flow: invokeTool → response(invocationId) → toolInvoked event → toolResponded event.
func InvokeTool(logger Logger, session *Session, toolName string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	if time.Now().After(session.ExpiresAt) {
		return toolErrf("browser tool %s: session has expired", toolName), nil
	}

	// Get the frame ID
	var frameId string
	frameResult, _ := session.SendCDP("Page.getFrameTree", nil)
	if frameResult != nil {
		var ft struct {
			FrameTree struct {
				Frame struct{ Id string `json:"id"` } `json:"frame"`
			} `json:"frameTree"`
		}
		json.Unmarshal(frameResult, &ft)
		frameId = ft.FrameTree.Frame.Id
	}

	// Build input as object — fix double-encoded JSON strings from Claude
	var inputObj map[string]any
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &inputObj); err != nil {
			inputObj = map[string]any{}
		}
		for k, v := range inputObj {
			if str, ok := v.(string); ok {
				if len(str) > 1 && str[0] == '{' {
					var parsed any
					if json.Unmarshal([]byte(str), &parsed) == nil {
						inputObj[k] = parsed
						continue
					}
				}
				if str == "true" {
					inputObj[k] = true
				} else if str == "false" {
					inputObj[k] = false
				}
			}
		}
	}
	if inputObj == nil {
		inputObj = map[string]any{}
	}

	// Refresh page state before action so the agent has current context
	session.Page.Refresh(session)

	invokeParams := map[string]any{
		"frameId":  frameId,
		"toolName": toolName,
		"input":    inputObj,
	}
	invokeJSON, _ := json.Marshal(invokeParams)
	logger.Printf("WebMCP.invokeTool: tool=%s params=%s", toolName, string(invokeJSON))

	// toolRespondedEvent is a loose version of webmcp.EventToolResponded
	// that accepts any status string (Chrome v149 sends "completed" instead of "Success").
	type toolRespondedEvent struct {
		InvocationID string          `json:"invocationId"`
		Status       string          `json:"status"`
		Output       json.RawMessage `json:"output"`
		ErrorText    string          `json:"errorText"`
	}

	// Step 1: Register event handler BEFORE sending the command
	resultCh := make(chan *toolRespondedEvent, 1)
	session.OnEvent("WebMCP.toolResponded", func(method string, params json.RawMessage) bool {
		var responded toolRespondedEvent
		if err := json.Unmarshal(params, &responded); err != nil {
			logger.Printf("[InvokeTool] toolResponded unmarshal error: %v", err)
			return true
		}
		logger.Printf("[InvokeTool] toolResponded: invocationId=%s status=%s", responded.InvocationID, responded.Status)
		resultCh <- &responded
		return false // stop listening
	})

	// Step 2: Send invokeTool — get invocationId
	result, err := session.SendCDP("WebMCP.invokeTool", invokeParams)
	if err != nil {
		logger.Printf("WebMCP.invokeTool error: tool=%s err=%v", toolName, err)
		return toolErrf("browser tool %s: %v", toolName, err), nil
	}

	var invokeResult struct {
		InvocationId string `json:"invocationId"`
	}
	json.Unmarshal(result, &invokeResult)
	logger.Printf("WebMCP.invokeTool: invocationId=%s", invokeResult.InvocationId)

	// Step 3: Wait for toolResponded event via the multiplexer (30s timeout)
	select {
	case responded := <-resultCh:
		status := strings.ToLower(responded.Status)
		if status == "error" || status == "canceled" {
			logger.Printf("WebMCP.invokeTool failed: tool=%s error=%s", toolName, responded.ErrorText)
			return toolErrf("browser tool %s error: %s", toolName, responded.ErrorText), nil
		}
		// The output may be a JSON string wrapping the actual result — unwrap it
		output := responded.Output
		var unwrapped string
		if json.Unmarshal(output, &unwrapped) == nil {
			// It was a JSON-encoded string — try to parse the inner value
			var inner any
			if json.Unmarshal([]byte(unwrapped), &inner) == nil {
				pretty, _ := json.MarshalIndent(inner, "", "  ")
				output = pretty
			} else {
				output = []byte(unwrapped)
			}
		} else {
			// Already a raw JSON object/array
			pretty, _ := json.MarshalIndent(json.RawMessage(output), "", "  ")
			output = pretty
		}
		logger.Printf("WebMCP.invokeTool result: tool=%s output=%s", toolName, string(output))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(output)}},
		}, nil
	case <-time.After(30 * time.Second):
		logger.Printf("WebMCP.invokeTool: timeout waiting for toolResponded: tool=%s", toolName)
		return toolErrf("browser tool %s: timeout waiting for response", toolName), nil
	}
}

// ProxyHTTPToolCall forwards a tool call to Chrome's MCP endpoint (HTTP).
// Used for the unblock path where there is no CDP WebSocket.
func ProxyHTTPToolCall(logger Logger, session *Session, toolName string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	if time.Now().After(session.ExpiresAt) {
		Store.Delete(session.SessionID)
		return toolErrf("webmcp proxy: session %s has expired", session.SessionID), nil
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
		return toolErrf("webmcp proxy: failed to create request: %v", err), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return toolErrf("webmcp proxy: request failed: %v", err), nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return toolErrf("webmcp proxy: failed to read response: %v", err), nil
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
		return toolErrf("webmcp %s: %s", toolName, rpcResp.Error.Message), nil
	}
	if rpcResp.Result != nil {
		return rpcResp.Result, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(respBody)}},
	}, nil
}

// toolErrf creates an error CallToolResult (local helper, mirrors parent package's ToolErrf).
func toolErrf(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
		IsError: true,
	}
}
