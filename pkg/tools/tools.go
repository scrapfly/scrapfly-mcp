package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scrapfly/scrapfly-mcp/pkg/mcpex"
)

type HandledTool struct {
	Tool    *mcp.Tool
	Handler mcp.ToolHandler
}

func NewHandledTool[In, Out any](t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) (*HandledTool, error) {
	tt, hh, err := mcpex.ToToolHandler(t, h)
	if err != nil {
		return nil, fmt.Errorf("NewHandledTool: tool %q: %v", t.Name, err)
	}
	return &HandledTool{Tool: tt, Handler: hh}, nil
}

type HandledToolSet map[string]*HandledTool

func NewHandledToolset() HandledToolSet {
	return make(HandledToolSet)
}

func AddToolToToolset[In, Out any](s HandledToolSet, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) error {
	ht, err := NewHandledTool(t, h)
	if err != nil {
		return fmt.Errorf("AddTool: tool %q: %v", t.Name, err)
	}
	s[t.Name] = ht
	return nil
}

// MustAddToolToToolset is a fail-fast wrapper around AddToolToToolset.
// Use it during init when a registration failure should be a hard
// error (i.e., always). Silently dropping tools at startup is the
// historical pattern that hid the empty-enum schema bug for months;
// don't reintroduce it.
func MustAddToolToToolset[In, Out any](s HandledToolSet, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	if err := AddToolToToolset(s, t, h); err != nil {
		panic(fmt.Sprintf("MustAddToolToToolset: %v", err))
	}
}

func (s HandledToolSet) RegisterTools(server *mcp.Server) []string {
	toolNames := make([]string, 0, len(s))
	for _, ht := range s {
		toolNames = append(toolNames, ht.Tool.Name)
		server.AddTool(ht.Tool, ht.Handler)
	}
	return toolNames
}

// Names returns every tool name in the set. Useful for the dynamic
// registration path: callers pass `set.Names()...` to
// `server.RemoveTools(...)`.
func (s HandledToolSet) Names() []string {
	names := make([]string, 0, len(s))
	for name := range s {
		names = append(names, name)
	}
	return names
}
