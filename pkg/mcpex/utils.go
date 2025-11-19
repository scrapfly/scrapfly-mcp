package mcpex

import "github.com/modelcontextprotocol/go-sdk/mcp"

func NewPlaceholderMCPCallToolResult() *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "STRUCTURED_CONTENT_PLACEHOLDER"}},
	}
}
