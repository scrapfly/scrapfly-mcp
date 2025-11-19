package provider

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/scrapfly-mcp/pkg/tools"
)

type toolProvider interface {
	ToolSet() tools.HandledToolSet
	PromptSet() tools.HandledPromptSet
	ResourceSet() tools.HandledResourceSet
}

type ToolProvider struct {
	name string
	toolProvider
}

func NewToolProvider(name string, p toolProvider) ToolProvider {
	return ToolProvider{name: name, toolProvider: p}
}

func (p *ToolProvider) Name() string {
	return p.name
}

func (p *ToolProvider) RegisterAll(server *mcp.Server) (toolNames []string, promptNames []string, resourceNames []string) {
	toolNames = p.RegisterTools(server)
	promptNames = p.RegisterPrompts(server)
	resourceNames = p.RegisterResources(server)
	return toolNames, promptNames, resourceNames
}

func (p *ToolProvider) RegisterTools(server *mcp.Server) []string {
	return p.ToolSet().RegisterTools(server)
}

func (p *ToolProvider) RegisterPrompts(server *mcp.Server) []string {
	return p.PromptSet().RegisterPrompts(server)
}

func (p *ToolProvider) RegisterResources(server *mcp.Server) []string {
	return p.ResourceSet().RegisterResources(server)
}
