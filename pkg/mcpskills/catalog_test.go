package mcpskills

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEmbeddedSkillsHaveCompleteVerifiableManifests(t *testing.T) {
	catalog, err := load(embeddedSkills)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.skills) < 2 {
		t.Fatalf("got %d embedded skills, want the product catalog", len(catalog.skills))
	}
	digestPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	for _, skill := range catalog.skills {
		t.Run(skill.URI, func(t *testing.T) {
			root := strings.TrimSuffix(strings.TrimPrefix(skill.URI, "skill://"), "/SKILL.md")
			var expected []string
			err := fs.WalkDir(embeddedSkills, sourceRoot, func(filePath string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				relative := strings.TrimPrefix(filePath, sourceRoot+"/")
				if relative == root || strings.HasPrefix(relative, root+"/") {
					expected = append(expected, "skill://"+relative)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(expected)
			var actual []string
			for _, resource := range skill.Resources {
				actual = append(actual, resource.URI)
				if !digestPattern.MatchString(resource.Digest) {
					t.Errorf("invalid digest %q", resource.Digest)
				}
				content, err := fs.ReadFile(embeddedSkills, sourceRoot+"/"+strings.TrimPrefix(resource.URI, "skill://"))
				if err != nil {
					t.Fatal(err)
				}
				if resource.Size != int64(len(content)) {
					t.Errorf("size = %d, want %d", resource.Size, len(content))
				}
				wantDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(content))
				if resource.Digest != wantDigest {
					t.Errorf("digest = %q, want %q", resource.Digest, wantDigest)
				}
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("manifest URIs = %v, want complete set %v", actual, expected)
			}
		})
	}
}

func TestEmbeddedCatalogRegisters(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "embedded-test", Version: "1"}, nil)
	if err := Register(server); err != nil {
		t.Fatal(err)
	}
}

func TestLoadValidatesSkills(t *testing.T) {
	valid := "---\nname: example\ndescription: Example skill\n---\n\nInstructions\n"
	tests := []struct {
		name    string
		source  fstest.MapFS
		wantErr string
	}{
		{
			name:    "no skill",
			source:  fstest.MapFS{"skills/README.md": {Data: []byte("none")}},
			wantErr: "no SKILL.md",
		},
		{
			name:    "missing frontmatter",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte("# Example")}},
			wantErr: "missing YAML frontmatter",
		},
		{
			name:    "unterminated frontmatter",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte("---\nname: example\ndescription: Example\n")}},
			wantErr: "unterminated YAML frontmatter",
		},
		{
			name:    "invalid YAML",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte("---\nname: [\n---\n")}},
			wantErr: "parse YAML frontmatter",
		},
		{
			name:    "name mismatch",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "name: example", "name: other", 1))}},
			wantErr: "does not match directory",
		},
		{
			name:    "invalid name",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "name: example", "name: Example", 1))}},
			wantErr: "invalid skill name",
		},
		{
			name:    "overlong name",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "name: example", "name: "+strings.Repeat("n", 65), 1))}},
			wantErr: "invalid skill name",
		},
		{
			name:    "missing description",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte("---\nname: example\n---\n")}},
			wantErr: "invalid skill description",
		},
		{
			name:    "empty description",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "Example skill", "", 1))}},
			wantErr: "invalid skill description",
		},
		{
			name:    "overlong description",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "Example skill", strings.Repeat("d", 1025), 1))}},
			wantErr: "invalid skill description",
		},
		{
			name:    "empty compatibility",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "Example skill", "Example\ncompatibility: \"\"", 1))}},
			wantErr: "invalid skill compatibility",
		},
		{
			name:    "non-string compatibility",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "Example skill", "Example\ncompatibility: 1", 1))}},
			wantErr: "invalid skill compatibility",
		},
		{
			name:    "overlong compatibility",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "Example skill", "Example\ncompatibility: "+strings.Repeat("c", 501), 1))}},
			wantErr: "invalid skill compatibility",
		},
		{
			name:    "non-mapping metadata",
			source:  fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(strings.Replace(valid, "Example skill", "Example\nmetadata: []", 1))}},
			wantErr: "metadata must be a mapping",
		},
		{
			name: "non-string metadata",
			source: fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(
				"---\nname: example\ndescription: Example\nmetadata:\n  version: 1\n---\n",
			)}},
			wantErr: "must be a string",
		},
		{
			name: "symlink",
			source: fstest.MapFS{
				"skills/example/SKILL.md": {Data: []byte(valid)},
				"skills/example/link":     {Data: []byte("target"), Mode: fs.ModeSymlink},
			},
			wantErr: "is a symlink",
		},
		{
			name: "non-regular file",
			source: fstest.MapFS{
				"skills/example/SKILL.md": {Data: []byte(valid)},
				"skills/example/pipe":     {Data: []byte("pipe"), Mode: fs.ModeNamedPipe},
			},
			wantErr: "is not a regular file",
		},
		{
			name: "file outside skills",
			source: fstest.MapFS{
				"skills/example/SKILL.md": {Data: []byte(valid)},
				"skills/README.md":        {Data: []byte("outside")},
			},
			wantErr: "outside every skill",
		},
		{
			name: "file limit",
			source: func() fstest.MapFS {
				source := fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(valid)}}
				for i := 0; i < 512; i++ {
					source[fmt.Sprintf("skills/example/%03d.txt", i)] = &fstest.MapFile{Data: []byte("x")}
				}
				return source
			}(),
			wantErr: "maximum is 512",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := load(test.source)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want text %q", err, test.wantErr)
			}
		})
	}
}

func TestSkillLimitsAreInclusive(t *testing.T) {
	valid := "---\nname: example\ndescription: Example skill\n---\n"
	t.Run("512 resources accepted", func(t *testing.T) {
		source := fstest.MapFS{"skills/example/SKILL.md": {Data: []byte(valid)}}
		for i := 0; i < 511; i++ {
			source[fmt.Sprintf("skills/example/%03d.txt", i)] = &fstest.MapFile{Data: []byte("x")}
		}
		catalog, err := load(source)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(catalog.skills[0].Resources); got != 512 {
			t.Errorf("got %d resources, want 512", got)
		}
	})
	for _, test := range []struct {
		name    string
		size    int
		wantErr string
	}{
		{name: "16 MiB accepted", size: 16*1024*1024 - len(valid)},
		{name: "16 MiB plus one rejected", size: 16*1024*1024 + 1 - len(valid), wantErr: "maximum is 16777216"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := fstest.MapFS{
				"skills/example/SKILL.md": {Data: []byte(valid)},
				"skills/example/blob.bin": {Data: bytes.Repeat([]byte{0xff}, test.size)},
			}
			_, err := load(source)
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want text %q", err, test.wantErr)
			}
		})
	}
}

func TestFrontmatterBoundariesAccepted(t *testing.T) {
	name := strings.Repeat("n", 64)
	source := fstest.MapFS{
		"skills/" + name + "/SKILL.md": {Data: []byte(fmt.Sprintf(
			"---\nname: %s\ndescription: %s\ncompatibility: %s\n---\n",
			name, strings.Repeat("d", 1024), strings.Repeat("c", 500),
		))},
	}
	if _, err := load(source); err != nil {
		t.Fatal(err)
	}
}

func TestLoadNestedSkillsAndDirectories(t *testing.T) {
	catalog, err := load(protocolFixture())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(catalog.skills), 2; got != want {
		t.Fatalf("got %d skills, want %d", got, want)
	}
	parent := catalog.byURI["skill://example/SKILL.md"]
	if got, want := manifestURIs(parent.Resources), []string{
		"skill://example/SKILL.md",
		"skill://example/nested/SKILL.md",
		"skill://example/nested/data.bin",
		"skill://example/references/guide.md",
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("parent manifest = %v, want %v", got, want)
	}
	nested := catalog.byURI["skill://example/nested/SKILL.md"]
	if got, want := manifestURIs(nested.Resources), []string{
		"skill://example/nested/SKILL.md",
		"skill://example/nested/data.bin",
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("nested manifest = %v, want %v", got, want)
	}
	if got := len(catalog.directories["skill://example/empty"]); got != 0 {
		t.Fatalf("empty directory has %d children", got)
	}
}

func TestPagination(t *testing.T) {
	start, end, next, err := page("", 101)
	if err != nil || start != 0 || end != 100 || next != "100" {
		t.Fatalf("first page = %d, %d, %q, %v", start, end, next, err)
	}
	start, end, next, err = page(next, 101)
	if err != nil || start != 100 || end != 101 || next != "" {
		t.Fatalf("second page = %d, %d, %q, %v", start, end, next, err)
	}
	for _, cursor := range []string{"-1", "01", "102", "bad"} {
		if _, _, _, err := page(cursor, 101); err == nil {
			t.Errorf("cursor %q was accepted", cursor)
		}
	}
}

func TestFrontmatterPassesEveryFieldThrough(t *testing.T) {
	catalog, err := load(protocolFixture())
	if err != nil {
		t.Fatal(err)
	}
	got := catalog.byURI["skill://example/SKILL.md"].Frontmatter
	want := map[string]any{
		"name":          "example",
		"description":   "Example skill",
		"license":       "MIT",
		"compatibility": "Requires an API key",
		"allowed-tools": "Read Bash(go:*)",
		"metadata":      map[string]any{"author": "Scrapfly", "version": "1.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frontmatter = %#v, want %#v", got, want)
	}
}

func manifestURIs(resources []SkillResource) []string {
	uris := make([]string, len(resources))
	for i, resource := range resources {
		uris[i] = resource.URI
	}
	return uris
}

func protocolFixture() fstest.MapFS {
	return fstest.MapFS{
		"skills/example/SKILL.md": {Data: []byte(`---
name: example
description: Example skill
license: MIT
compatibility: Requires an API key
allowed-tools: Read Bash(go:*)
metadata:
  author: Scrapfly
  version: "1.0"
---

Example instructions.
`)},
		"skills/example/references/guide.md": {Data: []byte("# Guide\n")},
		"skills/example/empty":               {Mode: fs.ModeDir},
		"skills/example/nested/SKILL.md": {Data: []byte(`---
name: nested
description: Nested skill
---
`)},
		"skills/example/nested/data.bin": {Data: []byte{0xff, 0x00, 0x01}},
	}
}
