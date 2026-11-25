package mcpskills

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ExtensionID         = "io.modelcontextprotocol/skills"
	MethodList          = "skills/list"
	MethodGet           = "skills/get"
	MethodDirectoryRead = "resources/directory/read"
	cacheTTLMs          = 300_000
	pageSize            = 100
)

type SkillResource struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type Skill struct {
	URI         string          `json:"uri"`
	Frontmatter map[string]any  `json:"frontmatter"`
	Resources   []SkillResource `json:"resources"`
}

type ListParams struct {
	mcp.ParamsBase
	Cursor string `json:"cursor,omitempty"`
}

type ListResult struct {
	mcp.ResultBase
	ResultType string `json:"resultType"`
	mcp.Cacheable
	Skills     []Skill `json:"skills"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

type GetParams struct {
	mcp.ParamsBase
	URI string `json:"uri"`
}

type GetResult struct {
	mcp.ResultBase
	ResultType string `json:"resultType"`
	mcp.Cacheable
	Skill Skill `json:"skill"`
}

type DirectoryParams struct {
	mcp.ParamsBase
	URI    string `json:"uri"`
	Cursor string `json:"cursor,omitempty"`
}

type DirectoryResult struct {
	mcp.ResultBase
	ResultType string `json:"resultType"`
	mcp.Cacheable
	Resources  []*mcp.Resource `json:"resources"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

func Capabilities() *mcp.ServerCapabilities {
	capabilities := &mcp.ServerCapabilities{
		Logging: &mcp.LoggingCapabilities{},
	}
	capabilities.AddExtension(ExtensionID, map[string]any{"directoryRead": true})
	return capabilities
}

func cacheable() mcp.Cacheable {
	return mcp.Cacheable{TTLMs: cacheTTLMs, CacheScope: "public"}
}
