package mcpex

import (
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/scrapfly-mcp/internal/patcher"
)

func setError(r *mcp.CallToolResult, err error) {
	r.Content = []mcp.Content{&mcp.TextContent{Text: err.Error()}}
	r.IsError = true
	if e := setCallToolResultErr(r, err); e != nil {
		log.Printf("WARNING: Failed to set call tool result err at protocol level: %v", e)
	}
}

func setCallToolResultErr(ptr interface{}, err error) error {
	return patcher.SetUnexportedField(ptr, "err", err)
}
