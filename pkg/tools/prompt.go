package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HandledPrompt struct {
	Prompt  *mcp.Prompt
	Handler mcp.PromptHandler
}
type HandledPromptSet map[string]*HandledPrompt

func NewHandledPromptSet() HandledPromptSet {
	return make(HandledPromptSet)
}

func NewHandledPrompt(p *mcp.Prompt, h mcp.PromptHandler) (*HandledPrompt, error) {
	return &HandledPrompt{Prompt: p, Handler: h}, nil
}

func AddPromptToPromptSet(s HandledPromptSet, p *mcp.Prompt, h mcp.PromptHandler) error {
	hp, err := NewHandledPrompt(p, h)
	if err != nil {
		return fmt.Errorf("AddPromptToPromptSet: prompt %q: %v", p.Name, err)
	}
	s[p.Name] = hp
	return nil
}

func (s HandledPromptSet) RegisterPrompts(server *mcp.Server) []string {
	promptNames := make([]string, 0, len(s))
	for _, hp := range s {
		promptNames = append(promptNames, hp.Prompt.Name)
		server.AddPrompt(hp.Prompt, hp.Handler)
	}
	return promptNames
}
