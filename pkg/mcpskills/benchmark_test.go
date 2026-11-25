package mcpskills

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func BenchmarkResourceRead(b *testing.B) {
	content := bytes.Repeat([]byte("a"), 16*1024)
	file := newSkillFile("skills/example/SKILL.md", content, nil)
	request := &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: file.resource.URI}}
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := file.read(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDirectoryRead(b *testing.B) {
	resources := make([]*mcp.Resource, 100)
	for i := range resources {
		resources[i] = &mcp.Resource{URI: fmt.Sprintf("skill://example/%03d.md", i), Name: "file.md", MIMEType: "text/markdown"}
	}
	catalog := &catalog{directories: map[string][]*mcp.Resource{"skill://example": resources}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := catalog.readDirectory(context.Background(), nil, &DirectoryParams{URI: "skill://example"}); err != nil {
			b.Fatal(err)
		}
	}
}
