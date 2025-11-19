package scrapflyprovider

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/resources"
)

type InstructionPromptOutput struct {
	InstructionPrompt string `json:"instruction_prompt"`
}

func (p *ScrapflyToolProvider) InstructionPrompt(
	ctx context.Context,
	req *mcp.CallToolRequest, _ DummyInput) (*mcp.CallToolResult, *InstructionPromptOutput, error) {

	return nil, &InstructionPromptOutput{InstructionPrompt: resources.InstructionPromptString},
		nil
}

func RecommendedSystemPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Description: "Recommended system prompt for your no code scraper agent",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: resources.InstructionPromptString},
			},
		},
	}, nil
}

func CompositePrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Description: "Composite prompt standard exemple for your no code scraper agent, combine system prompt and user prompt",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: resources.InstructionPromptString + req.Params.Arguments["user_prompt"]},
			},
		},
	}, nil
}
