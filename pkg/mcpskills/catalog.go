package mcpskills

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
)

const (
	sourceRoot       = "skills"
	skillFilename    = "SKILL.md"
	directoryMIME    = "inode/directory"
	maxSkillFiles    = 512
	maxSkillBytes    = 16 << 20
	maxNameLength    = 64
	maxDescription   = 1024
	maxCompatibility = 500
)

var (
	//go:embed skills
	embeddedSkills embed.FS
	skillName      = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

type skillFile struct {
	resource mcp.Resource
	digest   string
	text     string
	blob     []byte
	isText   bool
}

type catalog struct {
	skills      []Skill
	byURI       map[string]Skill
	files       map[string]*skillFile
	directories map[string][]*mcp.Resource
}

func Register(server *mcp.Server) error {
	return register(server, embeddedSkills)
}

func register(server *mcp.Server, source fs.FS) error {
	catalog, err := load(source)
	if err != nil {
		return fmt.Errorf("load MCP skills: %w", err)
	}
	for _, file := range catalog.files {
		server.AddResource(file.metadata(), file.read)
	}
	if err := mcp.AddReceivingCustomMethod(server, MethodList, catalog.list); err != nil {
		return fmt.Errorf("register %s: %w", MethodList, err)
	}
	if err := mcp.AddReceivingCustomMethod(server, MethodGet, catalog.get); err != nil {
		return fmt.Errorf("register %s: %w", MethodGet, err)
	}
	if err := mcp.AddReceivingCustomMethod(server, MethodDirectoryRead, catalog.readDirectory); err != nil {
		return fmt.Errorf("register %s: %w", MethodDirectoryRead, err)
	}
	return nil
}

func load(source fs.FS) (*catalog, error) {
	files, directories, skillRoots, err := walkSource(source)
	if err != nil {
		return nil, err
	}
	if len(skillRoots) == 0 {
		return nil, fmt.Errorf("no %s found", skillFilename)
	}

	c := &catalog{
		byURI:       make(map[string]Skill, len(skillRoots)),
		files:       make(map[string]*skillFile, len(files)),
		directories: make(map[string][]*mcp.Resource),
	}
	frontmatters := make(map[string]map[string]any, len(skillRoots))
	for _, root := range skillRoots {
		content, err := fs.ReadFile(source, path.Join(root, skillFilename))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", root, err)
		}
		frontmatter, err := parseFrontmatter(content)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path.Join(root, skillFilename), err)
		}
		if err := validateFrontmatter(root, frontmatter); err != nil {
			return nil, fmt.Errorf("%s: %w", path.Join(root, skillFilename), err)
		}
		frontmatters[root] = frontmatter
	}

	directoryURIs := make(map[string]string)
	for _, root := range skillRoots {
		for _, directory := range directories {
			if within(root, directory) {
				uri := skillURI(root)
				if directory != root {
					uri = skillURI(directory)
				}
				directoryURIs[directory] = uri
				c.directories[uri] = []*mcp.Resource{}
			}
		}
	}

	for _, filePath := range files {
		root, ok := owningSkill(skillRoots, filePath)
		if !ok {
			return nil, fmt.Errorf("file %s is outside every skill", filePath)
		}
		content, err := fs.ReadFile(source, filePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filePath, err)
		}
		if _, exists := c.files[filePath]; !exists {
			c.files[filePath] = newSkillFile(filePath, content, frontmatters[root])
		}
	}

	for _, root := range skillRoots {
		entry, err := skillEntry(root, files, c.files, frontmatters[root])
		if err != nil {
			return nil, err
		}
		c.skills = append(c.skills, entry)
		c.byURI[entry.URI] = entry
	}
	for directory, uri := range directoryURIs {
		parent := path.Dir(directory)
		if parentURI, ok := directoryURIs[parent]; ok {
			c.directories[parentURI] = append(c.directories[parentURI], &mcp.Resource{
				URI:      uri,
				Name:     path.Base(directory),
				MIMEType: directoryMIME,
			})
		}
	}
	for filePath, file := range c.files {
		parentURI, ok := directoryURIs[path.Dir(filePath)]
		if !ok {
			return nil, fmt.Errorf("file %s has no served parent directory", filePath)
		}
		c.directories[parentURI] = append(c.directories[parentURI], file.metadata())
	}
	for uri := range c.directories {
		slices.SortFunc(c.directories[uri], func(a, b *mcp.Resource) int {
			return strings.Compare(a.URI, b.URI)
		})
	}
	slices.SortFunc(c.skills, func(a, b Skill) int { return strings.Compare(a.URI, b.URI) })
	return c, nil
}

func walkSource(source fs.FS) ([]string, []string, []string, error) {
	files := []string{}
	directories := []string{}
	skillRoots := []string{}
	err := fs.WalkDir(source, sourceRoot, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == sourceRoot {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", filePath)
		}
		if entry.IsDir() {
			directories = append(directories, filePath)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect %s: %w", filePath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", filePath)
		}
		files = append(files, filePath)
		if path.Base(filePath) == skillFilename {
			skillRoots = append(skillRoots, path.Dir(filePath))
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("walk %s: %w", sourceRoot, err)
	}
	slices.Sort(files)
	slices.Sort(directories)
	slices.Sort(skillRoots)
	return files, directories, skillRoots, nil
}

func parseFrontmatter(content []byte) (map[string]any, error) {
	lines := bytes.Split(content, []byte("\n"))
	if len(lines) < 3 || !bytes.Equal(bytes.TrimSuffix(lines[0], []byte("\r")), []byte("---")) {
		return nil, fmt.Errorf("missing YAML frontmatter")
	}
	closing := -1
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(bytes.TrimSuffix(lines[i], []byte("\r")), []byte("---")) {
			closing = i
			break
		}
	}
	if closing < 0 {
		return nil, fmt.Errorf("unterminated YAML frontmatter")
	}
	frontmatter := map[string]any{}
	if err := yaml.Unmarshal(bytes.Join(lines[1:closing], []byte("\n")), &frontmatter); err != nil {
		return nil, fmt.Errorf("parse YAML frontmatter: %w", err)
	}
	return frontmatter, nil
}

func validateFrontmatter(root string, frontmatter map[string]any) error {
	name, ok := frontmatter["name"].(string)
	if !ok || len(name) == 0 || len(name) > maxNameLength || !skillName.MatchString(name) {
		return fmt.Errorf("invalid skill name")
	}
	if path.Base(root) != name {
		return fmt.Errorf("skill name %q does not match directory %q", name, path.Base(root))
	}
	description, ok := frontmatter["description"].(string)
	if !ok || utf8.RuneCountInString(description) == 0 || utf8.RuneCountInString(description) > maxDescription {
		return fmt.Errorf("invalid skill description")
	}
	if value, exists := frontmatter["compatibility"]; exists {
		compatibility, ok := value.(string)
		if !ok || utf8.RuneCountInString(compatibility) == 0 || utf8.RuneCountInString(compatibility) > maxCompatibility {
			return fmt.Errorf("invalid skill compatibility")
		}
	}
	if value, exists := frontmatter["metadata"]; exists {
		metadata, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("skill metadata must be a mapping")
		}
		for key, item := range metadata {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("skill metadata %q must be a string", key)
			}
		}
	}
	return nil
}

func newSkillFile(filePath string, content []byte, frontmatter map[string]any) *skillFile {
	mimeType := mime.TypeByExtension(path.Ext(filePath))
	if path.Ext(filePath) == ".md" {
		mimeType = "text/markdown"
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(content)
	}
	resource := mcp.Resource{
		URI:      skillURI(filePath),
		Name:     path.Base(filePath),
		MIMEType: mimeType,
		Size:     int64(len(content)),
	}
	if path.Base(filePath) == skillFilename && frontmatter != nil {
		resource.Name, _ = frontmatter["name"].(string)
		resource.Description, _ = frontmatter["description"].(string)
	}
	file := &skillFile{
		resource: resource,
		digest:   fmt.Sprintf("sha256:%x", sha256.Sum256(content)),
		isText:   utf8.Valid(content),
	}
	if file.isText {
		file.text = string(content)
	} else {
		file.blob = slices.Clone(content)
	}
	return file
}

func skillEntry(root string, files []string, loaded map[string]*skillFile, frontmatter map[string]any) (Skill, error) {
	entry := Skill{
		URI:         skillURI(path.Join(root, skillFilename)),
		Frontmatter: frontmatter,
		Resources:   []SkillResource{},
	}
	var total int64
	for _, filePath := range files {
		if !within(root, filePath) {
			continue
		}
		file := loaded[filePath]
		entry.Resources = append(entry.Resources, SkillResource{
			URI:    file.resource.URI,
			Digest: file.digest,
			Size:   file.resource.Size,
		})
		total += file.resource.Size
	}
	if len(entry.Resources) > maxSkillFiles {
		return Skill{}, fmt.Errorf("%s has %d files, maximum is %d", root, len(entry.Resources), maxSkillFiles)
	}
	if total > maxSkillBytes {
		return Skill{}, fmt.Errorf("%s has %d bytes, maximum is %d", root, total, maxSkillBytes)
	}
	return entry, nil
}

func owningSkill(roots []string, filePath string) (string, bool) {
	for i := len(roots) - 1; i >= 0; i-- {
		if within(roots[i], filePath) {
			return roots[i], true
		}
	}
	return "", false
}

func within(root, candidate string) bool {
	return candidate == root || strings.HasPrefix(candidate, root+"/")
}

func skillURI(filePath string) string {
	segments := strings.Split(strings.TrimPrefix(filePath, sourceRoot+"/"), "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return "skill://" + strings.Join(segments, "/")
}

func (f *skillFile) metadata() *mcp.Resource {
	resource := f.resource
	return &resource
}

func (f *skillFile) read(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	contents := &mcp.ResourceContents{
		URI:      request.Params.URI,
		MIMEType: f.resource.MIMEType,
	}
	if f.isText {
		contents.Text = f.text
	} else {
		contents.Blob = f.blob
	}
	return &mcp.ReadResourceResult{
		Cacheable: cacheable(),
		Contents:  []*mcp.ResourceContents{contents},
	}, nil
}

func (c *catalog) list(_ context.Context, _ *mcp.ServerSession, params *ListParams) (*ListResult, error) {
	start, end, next, err := page(paramsCursor(params), len(c.skills))
	if err != nil {
		return nil, err
	}
	return &ListResult{
		ResultType: "complete",
		Cacheable:  cacheable(),
		Skills:     slices.Clone(c.skills[start:end]),
		NextCursor: next,
	}, nil
}

func (c *catalog) get(_ context.Context, _ *mcp.ServerSession, params *GetParams) (*GetResult, error) {
	if params == nil {
		return nil, invalidParams("missing skill URI")
	}
	skill, ok := c.byURI[params.URI]
	if !ok {
		return nil, invalidParams("unknown skill %q", params.URI)
	}
	return &GetResult{
		ResultType: "complete",
		Cacheable:  cacheable(),
		Skill:      skill,
	}, nil
}

func (c *catalog) readDirectory(_ context.Context, _ *mcp.ServerSession, params *DirectoryParams) (*DirectoryResult, error) {
	if params == nil {
		return nil, invalidParams("missing directory URI")
	}
	resources, ok := c.directories[params.URI]
	if !ok {
		return nil, invalidParams("unknown skill directory %q", params.URI)
	}
	start, end, next, err := page(params.Cursor, len(resources))
	if err != nil {
		return nil, err
	}
	children := slices.Clone(resources[start:end])
	return &DirectoryResult{
		ResultType: "complete",
		Cacheable:  cacheable(),
		Resources:  children,
		NextCursor: next,
	}, nil
}

func paramsCursor(params *ListParams) string {
	if params == nil {
		return ""
	}
	return params.Cursor
}

func page(cursor string, length int) (int, int, string, error) {
	start := 0
	if cursor != "" {
		parsed, err := strconv.Atoi(cursor)
		if err != nil || parsed < 0 || parsed > length || strconv.Itoa(parsed) != cursor {
			return 0, 0, "", invalidParams("invalid pagination cursor")
		}
		start = parsed
	}
	end := min(start+pageSize, length)
	next := ""
	if end < length {
		next = strconv.Itoa(end)
	}
	return start, end, next, nil
}

func invalidParams(format string, args ...any) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: fmt.Sprintf(format, args...)}
}
