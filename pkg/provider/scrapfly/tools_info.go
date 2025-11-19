package scrapflyprovider

import (
	"context"

	"github.com/scrapfly/scrapfly-mcp/internal/sanitizer"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
)

// --------------------------------
// This is post-fix for langchain n8n mcp prevalidation when no params are required
// if ever n8n mcp_client node or langchain has regression with
// * error :"Received tool input did not match expected schema"
// * and this error does not show up in the mcp logs
// then
//
// * comment the typeAlias
// type DummyInput = any
//
// * uncomment the DummyInput struct
type DummyInput struct {
	Dummy string `json:"dummy,omitempty" jsonschema:"Dummy input (for langchain compatibility)"`
}

//
// this will be immediately functional and you'll save hours or days of headache that is not on the mcp server side.
//
// [Keep this comment in the codebase for future reference because this regression has been seen multiple times]
//--------------------------------

func (p *ScrapflyToolProvider) InfoAccount(
	ctx context.Context,
	req *mcp.CallToolRequest,
	_ DummyInput,
) (*mcp.CallToolResult, *scrapfly.AccountData, error) {
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("info_account", err), nil, err
	}
	p.logger.Println("Executing tool: info_account for client: ", client.APIKey())
	accountData, err := client.Account()
	if err != nil {
		return ToolErrFromError("account", err), nil, err
	}
	sanitizer.BasicSanitizeNils(accountData)
	return nil, accountData, err
}

type ApiKeyOutput struct {
	ApiKey string `json:"api_key"`
}

func (p *ScrapflyToolProvider) InfoApiKey(
	ctx context.Context,
	req *mcp.CallToolRequest,
	_ DummyInput,
) (*mcp.CallToolResult, *ApiKeyOutput, error) {
	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return ToolErrFromError("info_api_key", err), nil, err
	}
	p.logger.Println("Executing tool: info_api_key for client: ", client.APIKey())
	apiKey := client.APIKey()
	return nil, &ApiKeyOutput{ApiKey: apiKey}, nil
}
