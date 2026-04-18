package scrapflyprovider

// Static browser interaction tools — registered once at startup with flat names.
// Each tool looks up the active browser session via browser.FindSession("").
// Follows the Chrome DevTools MCP pattern (flat names, no session prefix).

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/browser"
	"github.com/scrapfly/scrapfly-mcp/pkg/tools"
)

func browserInteractionTools(provider *ScrapflyToolProvider) tools.HandledToolSet {
	ts := tools.NewHandledToolset()
	logger := provider.logger

	// ── Interaction tools (Antibot CDP domain) ─────────────────────────────

	// Simple schema for element-targeting tools — accepts uid from the page snapshot
	uidSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"uid": map[string]any{"type": "string", "description": "The element id from the page snapshot (e.g. \"183\")"},
		},
		"required": []string{"uid"},
	}
	uidTextSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"uid":   map[string]any{"type": "string", "description": "The element id from the page snapshot"},
			"value": map[string]any{"type": "string", "description": "Text to fill in"},
		},
		"required": []string{"uid", "value"},
	}

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "click",
		Title:       "Click on an element",
		Description: "Clicks on an element from the page snapshot. Use the element id from take_snapshot output.",
		Annotations: &mcp.ToolAnnotations{Title: "Click on an element", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: uidSchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		args := wrapUidToSelector(req.Params.Arguments)
		r, err := callActiveAntibot(logger, "clickOn", args)
		return r, nil, err
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "fill",
		Title:       "Fill an input field",
		Description: "Type text into an input or textarea. Use the element id from take_snapshot output.",
		Annotations: &mcp.ToolAnnotations{Title: "Fill an input field", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: uidTextSchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		args := wrapUidToFillArgs(req.Params.Arguments)
		r, err := callActiveAntibot(logger, "fill", args)
		return r, nil, err
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "type_text",
		Title:       "Type text with human timing",
		Description: "Type text character-by-character into a previously focused input.",
		Annotations: &mcp.ToolAnnotations{Title: "Type text with human timing", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "Text to type"},
			},
			"required": []string{"text"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		r, err := callActiveAntibot(logger, "typeText", req.Params.Arguments)
		return r, nil, err
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "hover",
		Title:       "Hover over an element",
		Description: "Hover over an element from the page snapshot.",
		Annotations: &mcp.ToolAnnotations{Title: "Hover over an element", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: uidSchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		args := wrapUidToSelector(req.Params.Arguments)
		r, err := callActiveAntibot(logger, "hover", args)
		return r, nil, err
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "press_key",
		Title:       "Press a keyboard key",
		Description: "Send a key press: Enter, Tab, Escape, ArrowDown, Backspace, etc.",
		Annotations: &mcp.ToolAnnotations{Title: "Press a keyboard key", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{"type": "string", "description": "Key to press: Enter, Tab, Escape, ArrowDown, etc."},
			},
			"required": []string{"key"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		r, err := callActiveAntibot(logger, "pressKey", req.Params.Arguments)
		return r, nil, err
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "scroll",
		Title:       "Scroll the page",
		Description: "Scroll an element into view by uid, or scroll the page by pixel delta. Use selector type \"bottom\" to scroll to the page bottom.",
		Annotations: &mcp.ToolAnnotations{Title: "Scroll the page", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uid":    map[string]any{"type": "string", "description": "Element id to scroll into view (optional)"},
				"deltaX": map[string]any{"type": "number", "description": "Horizontal scroll pixels (optional)"},
				"deltaY": map[string]any{"type": "number", "description": "Vertical scroll pixels (optional, e.g. 500 to scroll down)"},
			},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		var params struct {
			UID    string  `json:"uid"`
			DeltaX float64 `json:"deltaX"`
			DeltaY float64 `json:"deltaY"`
		}
		json.Unmarshal(req.Params.Arguments, &params)
		cdpArgs := map[string]any{}
		if params.UID != "" {
			cdpArgs["selector"] = map[string]any{"type": "axNodeId", "query": params.UID}
		} else if params.DeltaX == 0 && params.DeltaY == 0 {
			// No uid and no delta — scroll to bottom
			cdpArgs["selector"] = map[string]any{"type": "bottom", "query": ""}
		}
		if params.DeltaX != 0 || params.DeltaY != 0 {
			cdpArgs["delta"] = map[string]any{"x": params.DeltaX, "y": params.DeltaY}
		}
		translated, _ := json.Marshal(cdpArgs)
		r, err := callActiveAntibot(logger, "scroll", translated)
		if err != nil {
			return r, nil, err
		}

		// Also execute JS scrollBy as fallback — some pages have custom scroll containers
		// that Antibot.scroll (which uses native wheel events) can't scroll
		session, _ := browser.FindSession("")
		if session != nil && (params.DeltaX != 0 || params.DeltaY != 0) {
			session.SendCDP("Runtime.evaluate", map[string]any{
				"expression":    fmt.Sprintf("window.scrollBy(%v, %v)", params.DeltaX, params.DeltaY),
				"returnByValue": true,
			})
		}

		return r, nil, err
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "select_option",
		Title:       "Select a dropdown option",
		Description: "Select an option from a <select> dropdown by uid and value.",
		Annotations: &mcp.ToolAnnotations{Title: "Select a dropdown option", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uid":   map[string]any{"type": "string", "description": "Element id of the select element"},
				"value": map[string]any{"type": "string", "description": "Option value or text to select"},
			},
			"required": []string{"uid", "value"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		args := wrapUidToSelector(req.Params.Arguments)
		r, err := callActiveAntibot(logger, "selectOption", args)
		return r, nil, err
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "drag",
		Title:       "Drag and drop",
		Description: "Drag an element to another element, both identified by uid.",
		Annotations: &mcp.ToolAnnotations{Title: "Drag and drop", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"from_uid": map[string]any{"type": "string", "description": "Element id to drag"},
				"to_uid":   map[string]any{"type": "string", "description": "Element id to drop onto"},
			},
			"required": []string{"from_uid", "to_uid"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		// Convert from_uid/to_uid to from/to selectors
		var args struct {
			FromUID string `json:"from_uid"`
			ToUID   string `json:"to_uid"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		translated, _ := json.Marshal(map[string]any{
			"from": map[string]any{"type": "axNodeId", "query": args.FromUID},
			"to":   map[string]any{"type": "axNodeId", "query": args.ToUID},
		})
		r, err := callActiveAntibot(logger, "dragAndDrop", translated)
		return r, nil, err
	})

	// ── Inspection tools (CDP) ─────────────────────────────────────────────

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "get_page_url",
		Title:       "Get current page URL",
		Description: "Returns the current URL and page title of the browser.",
		Annotations: &mcp.ToolAnnotations{Title: "Get current page URL", DestructiveHint: &falseBool, ReadOnlyHint: true},
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("get_page_url: no active browser session"), nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("URL: %s\nTitle: %s", session.Page.URL, session.Page.Title)}},
		}, nil, nil
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "take_screenshot",
		Title:       "Take a screenshot",
		Description: "Capture a PNG screenshot of the current browser page.",
		Annotations: &mcp.ToolAnnotations{Title: "Take a screenshot", DestructiveHint: &falseBool, ReadOnlyHint: true},
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("take_screenshot: no active browser session"), nil, nil
		}
		result, err := session.SendCDP("Page.captureScreenshot", map[string]any{"format": "png"})
		if err != nil {
			return ToolErrf("take_screenshot: %v", err), nil, nil
		}
		var ss struct{ Data string `json:"data"` }
		json.Unmarshal(result, &ss)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{Data: []byte(ss.Data), MIMEType: "image/png"}},
		}, nil, nil
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "take_snapshot",
		Title:       "Get page content snapshot",
		Description: "Get the accessibility tree of the current page. Returns structured text with element IDs for interaction.",
		Annotations: &mcp.ToolAnnotations{Title: "Get page content snapshot", DestructiveHint: &falseBool, ReadOnlyHint: true},
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("take_snapshot: no active browser session"), nil, nil
		}
		session.Page.Refresh(session)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: session.Page.Snapshot()}},
		}, nil, nil
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "evaluate_script",
		Title:       "Run JavaScript",
		Description: "Execute JavaScript in the browser page. Only for data extraction or reading state — do NOT use for clicking, filling, or interaction (use click, fill, type_text instead).",
		Annotations: &mcp.ToolAnnotations{Title: "Run JavaScript", DestructiveHint: &falseBool},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]any{"type": "string", "description": "JavaScript expression to evaluate"},
			},
			"required": []string{"expression"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("evaluate_script: no active browser session"), nil, nil
		}
		var args struct {
			Expression string `json:"expression"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		if args.Expression == "" {
			return ToolErrf("evaluate_script: expression is required"), nil, nil
		}
		result, err := session.SendCDP("Runtime.evaluate", map[string]any{
			"expression":    args.Expression,
			"returnByValue": true,
		})
		if err != nil {
			return ToolErrf("evaluate_script: %v", err), nil, nil
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
			return ToolErrf("evaluate_script error: %s", evalResult.ExceptionDetails.Text), nil, nil
		}
		b, _ := json.MarshalIndent(evalResult.Result.Value, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})

	// ── WebMCP meta-tools ──────────────────────────────────────────────────

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "list_webmcp_tools",
		Title:       "List page-registered MCP tools",
		Description: "Discover WebMCP tools exposed by the current page via navigator.modelContext.registerTool(). Returns tool names, descriptions, and input schemas.",
		Annotations: &mcp.ToolAnnotations{Title: "List page-registered MCP tools", DestructiveHint: &falseBool, ReadOnlyHint: true},
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("list_webmcp_tools: no active browser session"), nil, nil
		}

		// Read tools stored on the page state (populated by toolsAdded events during cloud_browser_open)
		pageTools := session.Page.GetWebMCPTools()

		if len(pageTools) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No WebMCP tools registered on this page."}},
			}, nil, nil
		}

		b, _ := json.MarshalIndent(pageTools, "", "  ")
		logger.Printf("[list_webmcp_tools] Found %d page tools", len(pageTools))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Page-registered WebMCP tools:\n%s\n\nUse call_webmcp_tool with the tool name to execute.", string(b))}},
		}, nil, nil
	})

	tools.AddToolToToolset(ts, &mcp.Tool{
		Name:        "call_webmcp_tool",
		Title:       "Execute a page-registered MCP tool",
		Description: "Call a WebMCP tool discovered via list_webmcp_tools. The tool runs in the page's JavaScript context via CDP WebMCP.invokeTool.",
		Annotations: &mcp.ToolAnnotations{Title: "Execute a page-registered MCP tool", DestructiveHint: &falseBool},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool_name": map[string]any{"type": "string", "description": "Name of the WebMCP tool to call (from list_webmcp_tools)"},
				"input":     map[string]any{"type": "string", "description": "JSON-stringified parameters to pass to the tool. Omit for tools with no parameters."},
			},
			"required": []string{"tool_name"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("call_webmcp_tool: no active browser session"), nil, nil
		}
		var args struct {
			ToolName string `json:"tool_name"`
			Input    string `json:"input"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		if args.ToolName == "" {
			return ToolErrf("call_webmcp_tool: tool_name is required"), nil, nil
		}

		var inputArgs json.RawMessage
		if args.Input != "" {
			inputArgs = json.RawMessage(args.Input)
		} else {
			inputArgs = json.RawMessage(`{}`)
		}

		// Use the proper CDP WebMCP.invokeTool flow
		r, err := browser.InvokeTool(logger, session, args.ToolName, inputArgs)
		return r, nil, err
	})

	return ts
}

// callActiveAntibot finds the active session and calls an Antibot CDP tool.
func callActiveAntibot(logger browser.Logger, toolName string, arguments json.RawMessage) (*mcp.CallToolResult, error) {
	session, err := browser.FindSession("")
	if err != nil {
		return ToolErrf("%s: no active browser session. Call cloud_browser_open first.", toolName), nil
	}
	return browser.CallTool(logger, session, toolName, arguments)
}

// wrapUidToSelector converts a simple {"uid": "183"} to {"selector": {"type": "axNodeId", "query": "183"}}
// while preserving any extra fields (deltaX, deltaY, etc.).
func wrapUidToSelector(args json.RawMessage) json.RawMessage {
	var parsed map[string]any
	if json.Unmarshal(args, &parsed) != nil {
		return args
	}
	if uid, ok := parsed["uid"].(string); ok {
		parsed["selector"] = map[string]any{"type": "axNodeId", "query": uid}
		delete(parsed, "uid")
	}
	out, _ := json.Marshal(parsed)
	return out
}

// wrapUidToFillArgs converts {"uid": "183", "value": "hello"} to {"selector": {...}, "text": "hello"}
func wrapUidToFillArgs(args json.RawMessage) json.RawMessage {
	var parsed map[string]any
	if json.Unmarshal(args, &parsed) != nil {
		return args
	}
	if uid, ok := parsed["uid"].(string); ok {
		parsed["selector"] = map[string]any{"type": "axNodeId", "query": uid}
		delete(parsed, "uid")
	}
	if val, ok := parsed["value"]; ok {
		parsed["text"] = val
		delete(parsed, "value")
	}
	out, _ := json.Marshal(parsed)
	return out
}
