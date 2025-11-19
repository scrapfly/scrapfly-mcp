package tools

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HandledResource struct {
	Resource *mcp.Resource
	Handler  mcp.ResourceHandler
}

func NewHandledResource(t *mcp.Resource, h mcp.ResourceHandler) (*HandledResource, error) {
	return &HandledResource{Resource: t, Handler: h}, nil
}

type HandledResourceSet map[string]*HandledResource

func NewHandledResourceSet() HandledResourceSet {
	return make(HandledResourceSet)
}

func AddResourceToResourceSet(s HandledResourceSet, t *mcp.Resource, h mcp.ResourceHandler) error {
	hr, err := NewHandledResource(t, h)
	if err != nil {
		return fmt.Errorf("AddResourceToResourceSet: resource %q: %v", t.Name, err)
	}
	s[t.Name] = hr
	return nil
}

func (s HandledResourceSet) RegisterResources(server *mcp.Server) []string {
	resourceNames := make([]string, 0, len(s))
	for _, ht := range s {
		resourceNames = append(resourceNames, ht.Resource.Name)
		server.AddResource(ht.Resource, ht.Handler)
	}
	return resourceNames
}
