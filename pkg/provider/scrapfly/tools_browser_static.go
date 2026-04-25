package scrapflyprovider

// Static browser interaction tools — registered once at startup with flat names.
// Each tool looks up the active browser session via browser.FindSession("").
// Follows the Chrome DevTools MCP pattern (flat names, no session prefix).

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "click",
		Title:       "Click on an element",
		Description: "Click an element in the active cloud browser session. Requires a `uid` obtained from `take_snapshot` output — uids are stable for that snapshot only. Typical flow: take_snapshot → locate element by label/text → click(uid). After the click, the page may navigate or reveal new elements; take another snapshot before your next action if the DOM likely changed.",
		Annotations: &mcp.ToolAnnotations{Title: "Click on an element", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: uidSchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		args := wrapUidToSelector(req.Params.Arguments)
		r, err := callActiveAntibot(logger, "clickOn", args)
		return r, nil, err
	})

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "fill",
		Title:       "Fill an input field",
		Description: "Set the value of an input/textarea in one shot (faster than `type_text`). Prefer this for forms, search boxes, addresses. Requires `uid` from a recent `take_snapshot`. Use `type_text` instead only when the page has a live autocomplete that needs per-keystroke events.",
		Annotations: &mcp.ToolAnnotations{Title: "Fill an input field", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: uidTextSchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		args := wrapUidToFillArgs(req.Params.Arguments)
		r, err := callActiveAntibot(logger, "fill", args)
		return r, nil, err
	})

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "type_text",
		Title:       "Type text with human timing",
		Description: "Type text character-by-character into the currently focused input (use after clicking/focusing). Slower than `fill` but fires per-keystroke events — needed for live autocompletes, search-as-you-type, or inputs that reject setValue-style writes.",
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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "hover",
		Title:       "Hover over an element",
		Description: "Move the mouse over an element (by `uid` from `take_snapshot`). Use when a page reveals menus, tooltips, or controls only on hover — otherwise `click` is what you want.",
		Annotations: &mcp.ToolAnnotations{Title: "Hover over an element", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: uidSchema,
		Meta:        standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		args := wrapUidToSelector(req.Params.Arguments)
		r, err := callActiveAntibot(logger, "hover", args)
		return r, nil, err
	})

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "press_key",
		Title:       "Press a keyboard key",
		Description: "Send a single key to the focused element. Accepts names like Enter, Tab, Escape, ArrowDown, Backspace. Typical use: submit a form (Enter), navigate a listbox (ArrowDown), close a modal (Escape).",
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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "scroll",
		Title:       "Scroll the page",
		Description: "Scroll either (a) a specific element into view by `uid`, or (b) the page by a pixel delta, or (c) to bottom via selector type \"bottom\". Use (a) before clicking an off-screen element; use (c) to trigger infinite-scroll pagination; retake a snapshot afterwards since new content typically appears.",
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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "select_option",
		Title:       "Select a dropdown option",
		Description: "Pick an option from a native HTML `<select>` by the dropdown's `uid` and the option's `value` attribute (not its visible label). For custom dropdowns built out of divs, use `click` instead.",
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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "drag",
		Title:       "Drag and drop",
		Description: "Drag one element onto another (both by `uid`). Use for HTML5 drag-and-drop interfaces — reordering lists, uploading via drop zones, Trello-style boards. Rare outside those cases.",
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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "get_page_url",
		Title:       "Get current page URL",
		Description: "Return the browser's current URL and page title. Cheap; use it to confirm a navigation landed where you expected, or to capture the final URL after redirects before reporting back to the user.",
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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "take_screenshot",
		Title:       "Take a screenshot",
		Description: "Capture a PNG of the current cloud-browser page. Use when the user asks for a visual, or when the page's information (charts, diagrams, styled layout) isn't well-represented by the accessibility tree. For structural/text understanding, `take_snapshot` is cheaper and more actionable.",
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
		// TextContent sidecar: ADK's mcptoolset drops image-only tool
		// responses and errors with "no text content in tool response".
		// The text is what the LLM sees; the image goes to UI clients.
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Screenshot captured (PNG, %d bytes base64).", len(ss.Data))},
				&mcp.ImageContent{Data: []byte(ss.Data), MIMEType: "image/png"},
			},
		}, nil, nil
	})

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "take_snapshot",
		Title:       "Get page content snapshot",
		Description: "Return the current page's accessibility tree as structured text with element uids. This is your primary way to see and act on the page — read it to find links, buttons, inputs, headings, then use their uids with `click`, `fill`, `hover`, etc. Cheap — call it freely whenever you're unsure what's on the page or after an action that likely changed the DOM. Do NOT guess uids.",
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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "evaluate_script",
		Title:       "Run JavaScript",
		Description: "Evaluate a JS expression in the page and return its value. Use for *reading*: extract a computed value, read localStorage, inspect window state, pull structured data the snapshot doesn't expose. Do NOT use for interaction — `click`/`fill`/`type_text` are the supported paths and survive anti-bot checks. Script runs as `(() => <your expr>)()` in the main world.",
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
	//
	// Note: list_webmcp_tools and call_webmcp_tool are ALSO mounted by
	// addWebMCPMetaTools into the static tool surface so the model can
	// reach them across the dynamic mount/unmount boundary (ADK-Python
	// caches tools/list and doesn't refetch on
	// notifications/tools/list_changed today). Both tools error
	// gracefully when no browser is open. We keep them in this builder
	// too so they get re-mounted with the rest of the interaction set
	// — server.AddTool replaces existing tools with the same name, no
	// duplicate-registration risk.

	addWebMCPMetaTools(ts, logger)

	// ── browser-use parity tools ───────────────────────────────────────────
	// Convenience tools that close the gap with browser-use's action set.
	// All gated by browser.FindSession("") so they error cleanly when no
	// session is open — same contract as the rest of this file.

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "scroll_to_text",
		Title:       "Scroll until text is visible",
		Description: "Scroll the active page until the first element whose textContent contains the given substring is in the viewport. Case-insensitive substring match. No-op if the text is already visible. Cheap to call before any click on content that may be below the fold (long articles, lazy-loaded lists, paginated tables). Returns the matched element's tag and a 80-char text excerpt, or an error if not found.",
		Annotations: &mcp.ToolAnnotations{Title: "Scroll until text is visible", DestructiveHint: &falseBool},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{"type": "string", "description": "Substring to search for in element textContent"},
			},
			"required": []string{"text"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("scroll_to_text: no active browser session"), nil, nil
		}
		var args struct {
			Text string `json:"text"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		if args.Text == "" {
			return ToolErrf("scroll_to_text: text is required"), nil, nil
		}
		// Walk visible elements, find first that contains the text,
		// scroll into view + return tag/excerpt. Single CDP roundtrip.
		js := fmt.Sprintf(`(() => {
  const needle = %q.toLowerCase();
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_ELEMENT);
  let node;
  while ((node = walker.nextNode())) {
    const txt = (node.innerText || node.textContent || '').toLowerCase();
    if (txt.includes(needle)) {
      node.scrollIntoView({block:'center', behavior:'instant'});
      const own = (node.innerText || node.textContent || '').trim().slice(0, 80);
      return JSON.stringify({tag: node.tagName.toLowerCase(), excerpt: own});
    }
  }
  return JSON.stringify({error: 'not found'});
})()`, args.Text)
		evalResult, err := session.SendCDP("Runtime.evaluate", map[string]any{
			"expression": js, "returnByValue": true,
		})
		if err != nil {
			return ToolErrFromError("scroll_to_text", err), nil, nil
		}
		var rv struct {
			Result struct{ Value string `json:"value"` } `json:"result"`
		}
		json.Unmarshal(evalResult, &rv)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: rv.Result.Value}},
		}, nil, nil
	})

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "dropdown_options",
		Title:       "List options of a <select> element",
		Description: "Return all <option> entries (value, label, selected) of a native HTML <select> element by uid. Use before select_option to discover available choices when the snapshot's `options=\"a|b|c\"` enrichment is truncated or missing.",
		Annotations: &mcp.ToolAnnotations{Title: "List dropdown options", DestructiveHint: &falseBool, ReadOnlyHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uid": map[string]any{"type": "string", "description": "axNodeId from take_snapshot, must point at a <select> element"},
			},
			"required": []string{"uid"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		var args struct {
			UID string `json:"uid"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		if args.UID == "" {
			return ToolErrf("dropdown_options: uid is required"), nil, nil
		}
		// We can't address an element by axNodeId from JS-land. The
		// PageState.Refresh enrichment (previous commit) ALREADY ships
		// `options="..."` inline on every <select> in the snapshot;
		// this tool's job is just to remind the model where to look
		// when it forgot to read the snapshot or the enrichment was
		// truncated. Cheap, idempotent, no CDP cost.
		hint := `For native <select> dropdowns, the snapshot already includes options="..." inline ` +
			`alongside the combobox node. If the snapshot enrichment was truncated or missing, ` +
			`call evaluate_script with this expression (after taking a fresh snapshot to find a ` +
			`stable selector for the element):` + "\n\n" +
			`  Array.from(document.querySelector('select#YOUR_ID').options).map(o => ` +
			`({value: o.value, label: o.text, selected: o.selected}))`
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: hint}},
		}, nil, nil
	})

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "wait",
		Title:       "Pause execution",
		Description: "Sleep for N seconds before the next action. Use sparingly — most pages can be interacted with as soon as the snapshot returns. Useful for waiting on animations to settle, on lazy-loaded carousels, or on deliberate rate-limiting between repeated form submissions. Capped at 10s server-side; longer sleeps will be clamped.",
		Annotations: &mcp.ToolAnnotations{Title: "Wait", DestructiveHint: &falseBool, ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"seconds": map[string]any{"type": "number", "description": "Seconds to wait. Clamped to [0.1, 10]."},
			},
			"required": []string{"seconds"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		var args struct {
			Seconds float64 `json:"seconds"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		if args.Seconds < 0.1 {
			args.Seconds = 0.1
		}
		if args.Seconds > 10 {
			args.Seconds = 10
		}
		select {
		case <-time.After(time.Duration(args.Seconds * float64(time.Second))):
		case <-ctx.Done():
			return ToolErrf("wait: cancelled"), nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("waited %.2fs", args.Seconds)}},
		}, nil, nil
	})

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "find_elements",
		Title:       "CSS-selector element finder",
		Description: "Return up to 20 elements matching a CSS selector with their tag, text excerpt (80 chars), and key attributes. Use when take_snapshot doesn't expose what you need — typically for non-interactive content selection (`<article>`, `.product-card`, `[data-id]`). Returns an empty list rather than an error when nothing matches.",
		Annotations: &mcp.ToolAnnotations{Title: "Find elements by CSS", DestructiveHint: &falseBool, ReadOnlyHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"selector":    map[string]any{"type": "string", "description": "CSS selector"},
				"max_results": map[string]any{"type": "integer", "description": "Cap the result count. Default 20, max 50."},
			},
			"required": []string{"selector"},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("find_elements: no active browser session"), nil, nil
		}
		var args struct {
			Selector   string `json:"selector"`
			MaxResults int    `json:"max_results"`
		}
		json.Unmarshal(req.Params.Arguments, &args)
		if args.Selector == "" {
			return ToolErrf("find_elements: selector is required"), nil, nil
		}
		if args.MaxResults <= 0 {
			args.MaxResults = 20
		}
		if args.MaxResults > 50 {
			args.MaxResults = 50
		}
		js := fmt.Sprintf(`(() => {
  const els = Array.from(document.querySelectorAll(%q)).slice(0, %d);
  return JSON.stringify(els.map(e => {
    const attrs = {};
    for (const a of ['id','class','href','src','alt','title','data-id','data-test','aria-label']) {
      const v = e.getAttribute(a);
      if (v) attrs[a] = v.length > 80 ? v.slice(0,80)+'…' : v;
    }
    const text = (e.innerText || e.textContent || '').trim().slice(0, 80);
    return {tag: e.tagName.toLowerCase(), text, attrs};
  }));
})()`, args.Selector, args.MaxResults)
		evalResult, err := session.SendCDP("Runtime.evaluate", map[string]any{
			"expression": js, "returnByValue": true,
		})
		if err != nil {
			return ToolErrFromError("find_elements", err), nil, nil
		}
		var rv struct {
			Result struct{ Value string `json:"value"` } `json:"result"`
		}
		json.Unmarshal(evalResult, &rv)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: rv.Result.Value}},
		}, nil, nil
	})

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "go_back",
		Title:       "Browser back button",
		Description: "Navigate one step back in the browser's history (equivalent to clicking the browser's back arrow). Returns an error if there's nothing to go back to. After this call, take_snapshot to see the previous page's elements — DOM uids from prior snapshots are stale.",
		Annotations: &mcp.ToolAnnotations{Title: "Go back", DestructiveHint: &falseBool, OpenWorldHint: &trueBool},
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Meta: standardPermissionsMeta,
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, any, error) {
		session, err := browser.FindSession("")
		if err != nil {
			return ToolErrf("go_back: no active browser session"), nil, nil
		}
		// CDP recipe: read history, navigate to entry currentIndex-1.
		histResult, err := session.SendCDP("Page.getNavigationHistory", nil)
		if err != nil {
			return ToolErrFromError("go_back", err), nil, nil
		}
		var hist struct {
			CurrentIndex int `json:"currentIndex"`
			Entries      []struct {
				ID  int    `json:"id"`
				URL string `json:"url"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(histResult, &hist); err != nil {
			return ToolErrFromError("go_back", err), nil, nil
		}
		if hist.CurrentIndex <= 0 {
			return ToolErrf("go_back: no previous entry in history"), nil, nil
		}
		prev := hist.Entries[hist.CurrentIndex-1]
		_, err = session.SendCDP("Page.navigateToHistoryEntry", map[string]any{
			"entryId": prev.ID,
		})
		if err != nil {
			return ToolErrFromError("go_back", err), nil, nil
		}
		// Refresh page state so the next snapshot is current.
		session.Page.Refresh(session)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Navigated back to %s\n\n%s", prev.URL, session.Page.Snapshot())}},
		}, nil, nil
	})

	return ts
}

// addWebMCPMetaTools registers list_webmcp_tools and call_webmcp_tool
// into the given toolset. These are exposed BOTH statically (so the
// model can reach them across the cloud_browser_open / _close
// boundary even when the MCP client doesn't refetch tools/list — a
// known gap in adk-python) AND dynamically as part of the
// interactionTools set (so they appear alongside their siblings).
// `server.AddTool` replaces by name, so double-registration is safe.
//
// Both handlers no-op gracefully when no browser session is open.
func addWebMCPMetaTools(ts tools.HandledToolSet, logger *log.Logger) {
	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "list_webmcp_tools",
		Title:       "List page-registered MCP tools",
		Description: "List the WebMCP tools the current page has registered via `navigator.modelContext.registerTool()`. Returns name, description, and input schema for each. When a page exposes these, they are an author-provided programmatic API — prefer calling one via `call_webmcp_tool` over DOM scraping or UI clicks. The `cloud_browser_open` and `cloud_browser_navigate` responses already surface this list, so you rarely need to call this directly.",
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

	tools.MustAddToolToToolset(ts, &mcp.Tool{
		Name:        "call_webmcp_tool",
		Title:       "Execute a page-registered MCP tool",
		Description: "Invoke a WebMCP tool that the current page registered with `navigator.modelContext.registerTool()`. Preferred over clicking/scraping when the page exposes a matching tool — it's the author's declared API for that action and avoids DOM fragility. Input must satisfy the schema returned by `list_webmcp_tools` (also on the open/navigate response). Tool runs in the page's main world.",
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
