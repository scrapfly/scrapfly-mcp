package mcpskills

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type wireSkillResource struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type wireSkill struct {
	URI         string              `json:"uri"`
	Frontmatter map[string]any      `json:"frontmatter"`
	Resources   []wireSkillResource `json:"resources"`
}

type wireListParams struct {
	mcp.ParamsBase
	Cursor string `json:"cursor,omitempty"`
}

type wireListResult struct {
	mcp.ResultBase
	ResultType string `json:"resultType"`
	mcp.Cacheable
	Skills     []wireSkill `json:"skills"`
	NextCursor string      `json:"nextCursor,omitempty"`
}

type wireGetParams struct {
	mcp.ParamsBase
	URI string `json:"uri"`
}

type wireGetResult struct {
	mcp.ResultBase
	ResultType string `json:"resultType"`
	mcp.Cacheable
	Skill wireSkill `json:"skill"`
}

type wireDirectoryParams struct {
	mcp.ParamsBase
	URI    string `json:"uri"`
	Cursor string `json:"cursor,omitempty"`
}

type wireDirectoryResult struct {
	mcp.ResultBase
	ResultType string `json:"resultType"`
	mcp.Cacheable
	Resources  []*mcp.Resource `json:"resources"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

func newWireClient(t *testing.T) *mcp.Client {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "client-test", Version: "1"}, nil)
	if err := mcp.AddSendingCustomMethod[*wireListParams, *wireListResult](client, "skills/list"); err != nil {
		t.Fatal(err)
	}
	if err := mcp.AddSendingCustomMethod[*wireGetParams, *wireGetResult](client, "skills/get"); err != nil {
		t.Fatal(err)
	}
	if err := mcp.AddSendingCustomMethod[*wireDirectoryParams, *wireDirectoryResult](client, "resources/directory/read"); err != nil {
		t.Fatal(err)
	}
	return client
}

func connectWire(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	client := newWireClient(t)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	clientSession, err := client.Connect(t.Context(), clientTransport, &mcp.ClientSessionOptions{ProtocolVersion: "2026-07-28"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func newProtocolServer(t *testing.T, source fstest.MapFS) *mcp.Server {
	t.Helper()
	server := mcp.NewServer(
		&mcp.Implementation{Name: "skills-test", Version: "1"},
		&mcp.ServerOptions{Capabilities: Capabilities()},
	)
	if err := register(server, source); err != nil {
		t.Fatal(err)
	}
	return server
}

func TestSkillsProtocol(t *testing.T) {
	clientSession := connectWire(t, newProtocolServer(t, protocolFixture()))

	initialize := clientSession.InitializeResult()
	if initialize.ProtocolVersion != "2026-07-28" {
		t.Fatalf("protocol = %q", initialize.ProtocolVersion)
	}
	extension, ok := initialize.Capabilities.Extensions["io.modelcontextprotocol/skills"].(map[string]any)
	if !ok || extension["directoryRead"] != true {
		t.Fatalf("skills capability = %#v", initialize.Capabilities.Extensions["io.modelcontextprotocol/skills"])
	}
	if initialize.Capabilities.Resources == nil {
		t.Fatal("resources capability is missing")
	}

	list, err := mcp.CallCustomMethod[*wireListParams, *wireListResult](t.Context(), clientSession, "skills/list", &wireListParams{})
	if err != nil {
		t.Fatal(err)
	}
	assertComplete(t, list.ResultType, list.TTLMs, list.CacheScope)
	if got, want := skillURIs(list.Skills), []string{"skill://example/SKILL.md", "skill://example/nested/SKILL.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("skills/list URIs = %v, want %v", got, want)
	}

	get, err := mcp.CallCustomMethod[*wireGetParams, *wireGetResult](t.Context(), clientSession, "skills/get", &wireGetParams{URI: "skill://example/SKILL.md"})
	if err != nil {
		t.Fatal(err)
	}
	assertComplete(t, get.ResultType, get.TTLMs, get.CacheScope)
	if !reflect.DeepEqual(list.Skills[0], get.Skill) {
		t.Errorf("skills/list entry differs from skills/get: %#v != %#v", list.Skills[0], get.Skill)
	}
	if get.Skill.Frontmatter["license"] != "MIT" {
		t.Errorf("frontmatter lost license: %#v", get.Skill.Frontmatter)
	}
	assertManifestURIs(t, get.Skill.Resources, []string{
		"skill://example/SKILL.md",
		"skill://example/nested/SKILL.md",
		"skill://example/nested/data.bin",
		"skill://example/references/guide.md",
	})

	read, err := clientSession.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: get.Skill.URI})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Contents) != 1 || read.Contents[0].MIMEType != "text/markdown" {
		t.Fatalf("resource contents = %#v", read.Contents)
	}
	if read.Contents[0].Text != string(protocolFixture()["skills/example/SKILL.md"].Data) {
		t.Error("SKILL.md content changed in transit")
	}
	assertCache(t, read.TTLMs, read.CacheScope)
	wire, err := json.Marshal(read)
	if err != nil {
		t.Fatal(err)
	}
	var readWire map[string]any
	if err := json.Unmarshal(wire, &readWire); err != nil {
		t.Fatal(err)
	}
	if readWire["resultType"] != "complete" {
		t.Errorf("resources/read resultType = %#v", readWire["resultType"])
	}

	resources, err := clientSession.ListResources(t.Context(), &mcp.ListResourcesParams{})
	if err != nil {
		t.Fatal(err)
	}
	var skillResource *mcp.Resource
	for _, resource := range resources.Resources {
		if resource.URI == get.Skill.URI {
			skillResource = resource
			break
		}
	}
	if skillResource == nil {
		t.Fatal("SKILL.md is absent from resources/list")
	}
	if skillResource.Name != "example" || skillResource.Description != "Example skill" || skillResource.Size != int64(len(protocolFixture()["skills/example/SKILL.md"].Data)) {
		t.Errorf("SKILL.md metadata = %#v", skillResource)
	}

	binary, err := clientSession.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: "skill://example/nested/data.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.Contents[0].Blob; len(got) != 3 || got[0] != 0xff {
		t.Errorf("binary resource = %v", got)
	}

	directory, err := mcp.CallCustomMethod[*wireDirectoryParams, *wireDirectoryResult](t.Context(), clientSession, "resources/directory/read", &wireDirectoryParams{URI: "skill://example"})
	if err != nil {
		t.Fatal(err)
	}
	assertComplete(t, directory.ResultType, directory.TTLMs, directory.CacheScope)
	if directory.NextCursor != "" || len(directory.Resources) != 4 {
		t.Fatalf("root directory = %#v", directory)
	}
	children := make(map[string]*mcp.Resource, len(directory.Resources))
	for _, resource := range directory.Resources {
		children[resource.URI] = resource
	}
	if len(children) != 4 {
		t.Fatalf("directory contains duplicate or recursive children: %#v", children)
	}
	if child := children["skill://example/SKILL.md"]; child == nil || child.Name != "example" || child.Description != "Example skill" || child.MIMEType != "text/markdown" || child.Size != skillResource.Size {
		t.Errorf("SKILL.md directory metadata = %#v", child)
	}
	if child := children["skill://example/references"]; child == nil || child.Name != "references" || child.MIMEType != directoryMIME {
		t.Errorf("references directory metadata = %#v", child)
	}
	if child := children["skill://example/nested"]; child == nil || child.Name != "nested" || child.MIMEType != directoryMIME {
		t.Errorf("nested directory metadata = %#v", child)
	}
	if child := children["skill://example/empty"]; child == nil || child.Name != "empty" || child.MIMEType != directoryMIME {
		t.Errorf("empty directory metadata = %#v", child)
	}

	guide, err := mcp.CallCustomMethod[*wireDirectoryParams, *wireDirectoryResult](t.Context(), clientSession, "resources/directory/read", &wireDirectoryParams{URI: "skill://example/references"})
	if err != nil {
		t.Fatal(err)
	}
	if len(guide.Resources) != 1 || guide.Resources[0].URI != "skill://example/references/guide.md" || guide.Resources[0].Name != "guide.md" {
		t.Errorf("reference directory = %#v", guide.Resources)
	}

	empty, err := mcp.CallCustomMethod[*wireDirectoryParams, *wireDirectoryResult](t.Context(), clientSession, "resources/directory/read", &wireDirectoryParams{URI: "skill://example/empty"})
	if err != nil {
		t.Fatal(err)
	}
	assertComplete(t, empty.ResultType, empty.TTLMs, empty.CacheScope)
	if empty.Resources == nil || len(empty.Resources) != 0 {
		t.Errorf("empty directory resources = %#v, want []", empty.Resources)
	}
	emptyWire, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	var emptyObject map[string]any
	if err := json.Unmarshal(emptyWire, &emptyObject); err != nil {
		t.Fatal(err)
	}
	if resources, ok := emptyObject["resources"].([]any); !ok || len(resources) != 0 {
		t.Errorf("empty directory wire resources = %#v, want []", emptyObject["resources"])
	}

	assertInvalidParams(t, func() error {
		_, err := mcp.CallCustomMethod[*wireGetParams, *wireGetResult](t.Context(), clientSession, "skills/get", &wireGetParams{URI: "skill://missing/SKILL.md"})
		return err
	})
	assertInvalidParams(t, func() error {
		_, err := mcp.CallCustomMethod[*wireDirectoryParams, *wireDirectoryResult](t.Context(), clientSession, "resources/directory/read", &wireDirectoryParams{URI: get.Skill.URI})
		return err
	})
}

func TestSpecialFilenamesRoundTripThroughResources(t *testing.T) {
	source := protocolFixture()
	source["skills/example/references/100% #?.md"] = &fstest.MapFile{Data: []byte("special")}
	clientSession := connectWire(t, newProtocolServer(t, source))
	const uri = "skill://example/references/100%25%20%23%3F.md"

	result, err := clientSession.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Contents) != 1 || result.Contents[0].Text != "special" {
		t.Errorf("special file contents = %#v", result.Contents)
	}
}

func TestSkillsMethodsPaginate(t *testing.T) {
	source := fstest.MapFS{}
	for i := 0; i < 101; i++ {
		name := fmt.Sprintf("skill-%03d", i)
		source["skills/"+name+"/SKILL.md"] = &fstest.MapFile{Data: []byte(fmt.Sprintf("---\nname: %s\ndescription: Example\n---\n", name))}
	}
	for i := 0; i < 100; i++ {
		source[fmt.Sprintf("skills/skill-000/%03d.md", i)] = &fstest.MapFile{Data: []byte("x")}
	}
	clientSession := connectWire(t, newProtocolServer(t, source))

	first, err := mcp.CallCustomMethod[*wireListParams, *wireListResult](t.Context(), clientSession, "skills/list", &wireListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Skills) != 100 || first.NextCursor != "100" {
		t.Fatalf("first skills page = %d, cursor %q", len(first.Skills), first.NextCursor)
	}
	second, err := mcp.CallCustomMethod[*wireListParams, *wireListResult](t.Context(), clientSession, "skills/list", &wireListParams{Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Skills) != 1 || second.NextCursor != "" {
		t.Fatalf("second skills page = %d, cursor %q", len(second.Skills), second.NextCursor)
	}
	allSkills := append(append([]wireSkill{}, first.Skills...), second.Skills...)
	for i, skill := range allSkills {
		want := fmt.Sprintf("skill://skill-%03d/SKILL.md", i)
		if skill.URI != want {
			t.Errorf("skill %d URI = %q, want %q", i, skill.URI, want)
		}
	}

	firstDirectory, err := mcp.CallCustomMethod[*wireDirectoryParams, *wireDirectoryResult](t.Context(), clientSession, "resources/directory/read", &wireDirectoryParams{URI: "skill://skill-000"})
	if err != nil {
		t.Fatal(err)
	}
	if len(firstDirectory.Resources) != 100 || firstDirectory.NextCursor != "100" {
		t.Fatalf("first directory page = %d, cursor %q", len(firstDirectory.Resources), firstDirectory.NextCursor)
	}
	secondDirectory, err := mcp.CallCustomMethod[*wireDirectoryParams, *wireDirectoryResult](t.Context(), clientSession, "resources/directory/read", &wireDirectoryParams{URI: "skill://skill-000", Cursor: firstDirectory.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	allResources := append(append([]*mcp.Resource{}, firstDirectory.Resources...), secondDirectory.Resources...)
	if len(allResources) != 101 || secondDirectory.NextCursor != "" {
		t.Fatalf("directory pages contain %d resources, cursor %q", len(allResources), secondDirectory.NextCursor)
	}
	for i := 0; i < 100; i++ {
		want := fmt.Sprintf("skill://skill-000/%03d.md", i)
		if allResources[i].URI != want {
			t.Errorf("directory resource %d = %q, want %q", i, allResources[i].URI, want)
		}
	}
	if allResources[100].URI != "skill://skill-000/SKILL.md" {
		t.Errorf("last directory resource = %q", allResources[100].URI)
	}

	assertInvalidParams(t, func() error {
		_, err := mcp.CallCustomMethod[*wireListParams, *wireListResult](t.Context(), clientSession, "skills/list", &wireListParams{Cursor: "bad"})
		return err
	})
	assertInvalidParams(t, func() error {
		_, err := mcp.CallCustomMethod[*wireDirectoryParams, *wireDirectoryResult](t.Context(), clientSession, "resources/directory/read", &wireDirectoryParams{URI: "skill://skill-000", Cursor: "bad"})
		return err
	})
}

func skillURIs(skills []wireSkill) []string {
	uris := make([]string, len(skills))
	for i, skill := range skills {
		uris[i] = skill.URI
	}
	return uris
}

func assertManifestURIs(t *testing.T, resources []wireSkillResource, want []string) {
	t.Helper()
	got := make([]string, len(resources))
	seen := make(map[string]bool, len(resources))
	for i, resource := range resources {
		got[i] = resource.URI
		if seen[resource.URI] {
			t.Errorf("duplicate manifest URI %q", resource.URI)
		}
		seen[resource.URI] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("manifest URIs = %v, want %v", got, want)
	}
}

func assertComplete(t *testing.T, resultType string, ttlMs int, cacheScope string) {
	t.Helper()
	if resultType != "complete" {
		t.Errorf("resultType = %q, want complete", resultType)
	}
	assertCache(t, ttlMs, cacheScope)
}

func assertCache(t *testing.T, ttlMs int, cacheScope string) {
	t.Helper()
	if ttlMs != 300_000 || cacheScope != "public" {
		t.Errorf("cache = %d, %q, want 300000, public", ttlMs, cacheScope)
	}
}

func assertInvalidParams(t *testing.T, call func() error) {
	t.Helper()
	err := call()
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Fatalf("error = %v, want JSON-RPC invalid params", err)
	}
}
