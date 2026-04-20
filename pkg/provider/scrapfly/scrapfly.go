package scrapflyprovider

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/constants"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/resources"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/schemas"
	"github.com/scrapfly/scrapfly-mcp/pkg/tools"
)

type ScrapflyClientGetter func(p *ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error)

type ScrapflyToolProvider struct {
	Client       *scrapfly.Client
	ClientGetter ScrapflyClientGetter
	MCPServer    *mcp.Server // set during RegisterAll(), used for dynamic tool registration (cloud browser)
	logger       *log.Logger
}

// if logger is nil, it will use the default logger with opinionated prefix and settings
func NewScrapflyToolProvider(client *scrapfly.Client, clientGetter ScrapflyClientGetter, logger *log.Logger) *ScrapflyToolProvider {
	if logger == nil {
		logger = log.New(os.Stderr, "[ScrapflyToolProvider] ", log.Lmicroseconds|log.Lmsgprefix|log.LstdFlags)
	}
	return &ScrapflyToolProvider{
		Client:       client,
		ClientGetter: clientGetter,
		logger:       logger,
	}
}

func MakeDefaultScrapflyClient(apiKey string) *scrapfly.Client {
	client, err := scrapfly.New(apiKey)
	if err != nil {
		log.Printf("Failed to create scrapfly client: %v", err)
	}
	return client
}
func GetDefaultScrapflyClient(p *ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error) {
	// Fall back to the pre-configured client
	if p.Client == nil {
		return nil, fmt.Errorf("client not found")
	}
	return p.Client, nil
}

func (p *ScrapflyToolProvider) SetMCPServer(server *mcp.Server) {
	p.MCPServer = server
}

func (p *ScrapflyToolProvider) ToolSet() tools.HandledToolSet {
	return standardTools(p)
}

func (p *ScrapflyToolProvider) PromptSet() tools.HandledPromptSet {
	return standardPrompts(p)
}

func (p *ScrapflyToolProvider) ResourceSet() tools.HandledResourceSet {
	return standardResources(p)
}

var falseBool = false
var trueBool = true
var standardPermissionsMeta = mcp.Meta{
	"scrapfly/permissions/sufficient": []string{},
	"scrapfly/permissions/required":   []string{"tools:default"},
}

func standardTools(provider *ScrapflyToolProvider) tools.HandledToolSet {
	HandledTools := tools.NewHandledToolset()
	tools.AddToolToToolset(HandledTools, &mcp.Tool{ // alias
		Name:        "info_account",
		Title:       "Scrapfly Account Informations",
		Description: "Return the caller's Scrapfly account state: subscription plan, API-credit usage, concurrency limits, quotas. Call when the user asks about their account / billing / quota / remaining credits / plan. Not for scraping content.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Account Informations",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
			ReadOnlyHint:    true,
		},
		Meta: standardPermissionsMeta,
	}, provider.InfoAccount)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "web_scrape",
		Title:       "Scrapfly Advanced Scraping Tool",
		Description: "One-shot fetch of a URL with full control (headers, JS rendering, country, proxy pool, anti-scraping options). Stateless — returns the response body and metadata, no persistent session. This is the right tool whenever the task is \"get the content/bytes at this URL\": downloading a file, fetching an HTML page, calling a JSON endpoint, grabbing a sitemap. Only switch to `cloud_browser_open` when the task requires multi-step interaction with a page (clicking, form filling, navigating between pages, logging in). Use `scraping_instruction_enhanced` first if you're uncertain which options to set. Prefer `web_get_page` for the common quick-fetch path.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Advanced Scraping Tool",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
			ReadOnlyHint:    true,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[ScrapeToolInput](),
		Meta:        standardPermissionsMeta,
	}, ScrapingHandlerFor[ScrapeToolInput](provider))
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "web_get_page",
		Title:       "Scrapfly Quick Page Fetch Tool",
		Description: "One-shot fetch of a URL with sane defaults. Stateless. Right choice for simple \"get me the page / the JSON / the file at X\" asks — including plain file downloads where the URL already points at the asset. Falls back to `web_scrape` when you need to tune headers/JS-rendering/proxy; to `cloud_browser_open` when the task needs interaction with the page.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Quick Page Fetch Tool",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &trueBool,
			ReadOnlyHint:    true,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[GetPageToolInput](),
		Meta:        standardPermissionsMeta,
	}, ScrapingHandlerFor[GetPageToolInput](provider))
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "info_api_key",
		Title:       "Scrapfly Account API Key",
		Description: "Return the Users' ScrapFly API key",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Account API Key",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &falseBool,
			ReadOnlyHint:    true,
		},
		Meta: standardPermissionsMeta,
	}, provider.InfoApiKey)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "scraping_instruction_enhanced",
		Title:       "Scrapfly Scraping tools instructions // enhanced prompt",
		Description: "Return a concise cheat-sheet of Scrapfly's scraping options (ASP, render_js, country, proxy pool, session) and when to use each. Call this before your first `web_scrape` / `web_get_page` on an unfamiliar target so the chosen options are right the first time.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Scraping tools instructions // enhanced prompt",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &falseBool,
			ReadOnlyHint:    true,
		},
		Meta: standardPermissionsMeta,
	}, provider.InstructionPrompt)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "screenshot",
		Title:       "Scrapfly Screenshot Tool",
		Description: "One-shot PNG screenshot of a URL. Stateless — loads the page, renders, returns the image. Inputs: `url` (required, must be http:// or https://), `capture` (optional — either 'fullpage' or a CSS selector such as `img[alt='Logo']`). There is NO `selector` parameter — use `capture` for element-cropped shots.\n\nWhen to use this vs `cloud_browser_open` + `cloud_browser_screenshot`:\n  • Use `screenshot` only for truly stateless captures (\"give me a screenshot of <url>\") where the user does not need to see a live browser panel and will not follow up with more actions on the same page.\n  • Prefer `cloud_browser_open` + `cloud_browser_screenshot` whenever the user asks to \"go to\", \"open\", or \"visit\" a site, or mentions seeing a logo/element/feature on the page — even if the request looks like a single-screenshot task. That path opens the live browser panel the user watches, and the screenshot is element-cropped via CDP with proper scroll-into-view handling. The LLM-facing conversation UI is built around seeing the browser session.\n  • If a session is already active, never fall back to this one-shot tool; continue inside the session with `cloud_browser_screenshot` / `take_screenshot`.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Screenshot Tool",
			DestructiveHint: &falseBool,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
			ReadOnlyHint:    true,
		},
		InputSchema: schemas.MustRefineScreenshotToolInputSchema[ScreenshotToolInput](),
		Meta:        standardPermissionsMeta,
	}, provider.Screenshot)

	// Cloud Browser tools
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "check_if_blocked",
		Title:       "Antibot Block Detector",
		Description: "Given a scrape result (URL + status + headers + body), decide if it is actually a block page. Runs the Scrapfly classification API which detects the major anti-bot products (Cloudflare, DataDome, PerimeterX, Akamai, Kasada, Imperva, AWS WAF, F5 Shape, and more). Returns `is_blocked` and, when known, the matching `antibot` vendor. Costs 1 API credit per call. Run after a `web_scrape` / `web_get_page` whenever the response looks suspicious — a 200 with empty/tiny body, unexpected challenge markup — before feeding the content back to the user. If blocked, retry via `browser_unblock`.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Antibot Block Detector",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &falseBool,
			ReadOnlyHint:    true,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[CheckIfBlockedInput](),
		Meta:        standardPermissionsMeta,
	}, provider.CheckIfBlocked)

	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_open",
		Title:       "Scrapfly Cloud Browser — Open Session",
		Description: "Start a stateful real-browser session at a URL. Use this when the task requires *interaction* with a page — the user said \"open\", \"go to\", \"navigate to\", \"log in\", or the work needs clicking, form filling, or multi-step navigation. Returns the initial accessibility snapshot plus the page's registered WebMCP tools.\n\n" +
			"Do NOT use this for a plain \"download <url>\" or \"fetch <url>\" where the URL already points at the asset and no interaction is needed — use `web_get_page` (simple) or `web_scrape` (with options) instead. Those are cheaper and faster.\n\n" +
			"Once a session is open, the FULL set of CDP-backed interaction tools is available and operates on this session implicitly (no session_id in the calls):\n" +
			"  • Reading: `take_snapshot` (accessibility tree + uids), `take_screenshot` (PNG), `get_page_url`, `evaluate_script` (read-only JS).\n" +
			"  • Input: `click`, `fill`, `type_text`, `hover`, `press_key`, `scroll`, `drag`, `select_option`.\n" +
			"  • Page-author API: `list_webmcp_tools`, `call_webmcp_tool` — prefer these when the page exposes a matching tool; they are the author's declared programmatic API and survive DOM refactors.\n" +
			"  • Navigation in the same session: `cloud_browser_navigate`. Session management: `cloud_browser_sessions`, `cloud_browser_close` (only on explicit user request), `cloud_browser_downloads`, `cloud_browser_performance`.\n\n" +
			"If the opened page shows a challenge/captcha, close and retry with `browser_unblock`.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser — Open Session",
			DestructiveHint: &falseBool,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
			ReadOnlyHint:    false,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[CloudBrowserOpenInput](),
		Meta:        standardPermissionsMeta,
	}, provider.CloudBrowserOpen)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_close",
		Title:       "Scrapfly Cloud Browser — Close Session",
		Description: "Close the cloud-browser session and release its resources. Call this ONLY when the user explicitly asks to end / close / stop / \"we're done\". When a task finishes normally, leave the session open — the user has a dedicated UI control for closing and may keep going in a follow-up turn.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser — Close Session",
			DestructiveHint: &trueBool,
			IdempotentHint:  true,
			OpenWorldHint:   &falseBool,
			ReadOnlyHint:    false,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[CloudBrowserCloseInput](),
		Meta:        standardPermissionsMeta,
	}, provider.CloudBrowserClose)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_screenshot",
		Title:       "Scrapfly Cloud Browser — Screenshot",
		Description: "PNG of the active cloud-browser session. Optional `selector` is a CSS selector (e.g. `img[alt='Scrapfly Logo']`, `#header`, `.logo img`) — NOT a uid from `take_snapshot`. Without `selector`, captures the full viewport (or full page if `full_page: true`). For element-level shots where you only have a uid, prefer `take_screenshot` after `scroll`-ing the element into view; `take_screenshot` is the newer flat-API equivalent and is preferred in general.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser — Screenshot",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &falseBool,
			ReadOnlyHint:    true,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[CloudBrowserScreenshotInput](),
		Meta:        standardPermissionsMeta,
	}, provider.CloudBrowserScreenshot)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_eval",
		Title:       "Scrapfly Cloud Browser — Evaluate JavaScript",
		Description: "Run JavaScript in the active cloud-browser session and return its value. Equivalent to `evaluate_script`; prefer `evaluate_script` in new code. Reads only — do NOT use for clicking/filling.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser — Evaluate JavaScript",
			DestructiveHint: &falseBool,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
			ReadOnlyHint:    false,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[CloudBrowserEvalInput](),
		Meta:        standardPermissionsMeta,
	}, provider.CloudBrowserEval)
	// cloud_browser_snapshot removed — snapshot is automatically included in
	// cloud_browser_open, cloud_browser_navigate, and after fill/clickOn responses.
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_performance",
		Title:       "Scrapfly Cloud Browser — PageSpeed Lab Run",
		Description: "PageSpeed Insights-style lab run: cold-cache reload with mobile throttling (Moto G4 + slow 4G + 4× CPU) by default, or desktop wired. Returns Core Web Vitals (LCP, FCP, CLS, TTFB, INP), Speed Index, Total Blocking Time, Time To Interactive, resource waterfall with render-blocking detection, diagnostics (DOM nodes, main-thread ms, total byte weight), Lighthouse-style performance score (0-100), and Good/Needs-Improvement/Poor ratings per PSI thresholds. Use after cloud_browser_open. Inputs: preset ('mobile'|'desktop'), timeout_ms (max 30000).",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser — Performance Metrics",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &falseBool,
			ReadOnlyHint:    true,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[CloudBrowserPerformanceInput](),
		Meta:        standardPermissionsMeta,
	}, provider.CloudBrowserPerformance)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_sessions",
		Title:       "Scrapfly Cloud Browser — List Sessions",
		Description: "List active cloud-browser sessions for this user with their URLs, registered WebMCP tools, and expiry. Useful when you lost track of what's open, or to decide between reusing an existing session and opening a fresh one.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser — List Sessions",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &falseBool,
			ReadOnlyHint:    true,
		},
		Meta: standardPermissionsMeta,
	}, provider.CloudBrowserSessions)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_downloads",
		Title:       "Scrapfly Cloud Browser — Downloads",
		Description: "Inspect files that have been downloaded during the current browser session (e.g. after clicking a link that triggered a download, or submitting a form that returned an attachment). Without `filename`, returns metadata for every captured download. With `filename`, returns the file's bytes base64-encoded. Only relevant when a download was triggered inside the session — for fetching a file whose URL you already know, use `web_get_page` / `web_scrape` instead.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser — Downloads",
			DestructiveHint: &falseBool,
			IdempotentHint:  true,
			OpenWorldHint:   &falseBool,
			ReadOnlyHint:    true,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[CloudBrowserDownloadsInput](),
		Meta:        standardPermissionsMeta,
	}, provider.CloudBrowserDownloads)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "browser_unblock",
		Title:       "Open Browser with Anti-Bot Bypass",
		Description: "Same as `cloud_browser_open` but runs the URL through Scrapfly's anti-bot bypass first (Cloudflare, DataDome, PerimeterX, Akamai, etc.). Use when a plain `cloud_browser_open` landed on a challenge/captcha page, or when you already know the target is anti-bot protected. On success you get a stateful browser session with the same full action set available (`take_snapshot`, `click`, `fill`, `call_webmcp_tool`, …) — pick the right follow-up exactly as you would after `cloud_browser_open`.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Open Browser with Anti-Bot Bypass",
			DestructiveHint: &falseBool,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
			ReadOnlyHint:    false,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[BrowserUnblockInput](),
		Meta:        standardPermissionsMeta,
	}, provider.BrowserUnblock)
	tools.AddToolToToolset(HandledTools, &mcp.Tool{
		Name:        "cloud_browser_navigate",
		Title:       "Scrapfly Cloud Browser — Navigate",
		Description: "Navigate the active cloud-browser session to a new URL. Re-uses the same browser (cookies, storage, session state preserved) and refreshes the WebMCP tool list because page-registered tools are scoped to the current document. Returns a fresh snapshot. Use this instead of opening a new session when you want to stay signed in or keep context across URLs.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Scrapfly Cloud Browser — Navigate",
			DestructiveHint: &falseBool,
			IdempotentHint:  false,
			OpenWorldHint:   &trueBool,
			ReadOnlyHint:    false,
		},
		InputSchema: schemas.MustRefineScrapingToolInputSchema[CloudBrowserNavigateInput](),
		Meta:        standardPermissionsMeta,
	}, provider.CloudBrowserNavigate)

	// Static browser interaction tools (click, fill, hover, etc.) + WebMCP meta-tools
	for name, ht := range browserInteractionTools(provider) {
		HandledTools[name] = ht
	}

	return HandledTools
}

func standardPrompts(_ *ScrapflyToolProvider) tools.HandledPromptSet {
	if constants.DisableProviderPrompts {
		return tools.NewHandledPromptSet()
	}
	HandledPrompts := tools.NewHandledPromptSet()
	tools.AddPromptToPromptSet(HandledPrompts, PromptsList[0], RecommendedSystemPrompt)
	tools.AddPromptToPromptSet(HandledPrompts, PromptsList[1], CompositePrompt)
	return HandledPrompts
}

var PromptsList = []*mcp.Prompt{
	{
		Name: "system_prompt",
		//Title:       "Scrapfly Scraping tools Recommended System Prompt",
		Description: "System prompt standard exemple for your no code scraper agent",
	},
	{
		Name: "composite_prompt",
		//Title:       "Scrapfly Scraping tools Composite Prompt builder",
		Description: "Composite prompt standard exemple for your no code scraper agent, combine system prompt and user prompt",
		Arguments:   []*mcp.PromptArgument{{Name: "user_prompt", Description: "User prompt"}},
	},
}

var ResourcesList = []*mcp.Resource{
	{
		Name:        "web_scraping_openapi_specification",
		MIMEType:    "text/plain",
		URI:         "embedded:web_scraping_api",
		Description: "Scraping API specification for Scrapfly as last resort reference",
		Annotations: &mcp.Annotations{
			Audience: []mcp.Role{"user"},
			Priority: 0.0,
		},
	},
	{
		Name:        "scraping_instruction_enhanced",
		MIMEType:    "text/plain",
		URI:         "embedded:scraping_instruction_enhanced",
		Description: "Scraping instruction / enhanced prompt for scraping tools",
		Annotations: &mcp.Annotations{
			Audience: []mcp.Role{"assistant"},
			Priority: 1.0,
		},
	},
}

func standardResources(_ *ScrapflyToolProvider) tools.HandledResourceSet {
	HandledResources := tools.NewHandledResourceSet()
	if constants.DisableProviderResources {
		return HandledResources
	}
	tools.AddResourceToResourceSet(HandledResources, ResourcesList[0], resources.EmbeddedResourceHandler)
	tools.AddResourceToResourceSet(HandledResources, ResourcesList[1], resources.EmbeddedResourceHandler)
	return HandledResources
}
