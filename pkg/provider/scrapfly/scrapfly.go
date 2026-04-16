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
		Description: "Get subscription, usage, limits. Use for quotas/billing/concurrency. Avoid for content scraping.",
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
		Description: "Scrape a URL with full control. Use tool scraping_instruction_enhanced before using this tool. Prefer web_get_page for quick fetch",
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
		Description: "Quick page fetch with sane defaults. Use tool scraping_instruction_enhanced before using this tool. Use when you just need the content fast.",
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
		Description: "Return critical instructions for scraping tools",
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
		Description: "Screenshot a URL.",
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
		Name:        "cloud_browser_open",
		Title:       "Scrapfly Cloud Browser — Open Session",
		Description: "Open a remote Chrome browser with MCP-enabled DevTools. Returns a CDP WebSocket URL to connect with Playwright/Puppeteer. Navigate to the target URL after connecting. The browser exposes 18 Antibot tools (clickOn, fill, typeText, scroll, hover, pressKey, selectOption, etc.) via WebMCP. By default uses direct allocation (fast). Set unblock=true ONLY if the site blocks normal access (anti-bot protection).",
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
		Description: "Close a Cloud Browser session and release resources. Removes any dynamically registered WebMCP tools.",
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
		Name:        "cloud_browser_sessions",
		Title:       "Scrapfly Cloud Browser — List Sessions",
		Description: "List all running Cloud Browser sessions for the account.",
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
		Name:        "cloud_browser_navigate",
		Title:       "Scrapfly Cloud Browser — Navigate",
		Description: "Navigate an active Cloud Browser session to a new URL. Re-discovers WebMCP tools on the new page.",
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
