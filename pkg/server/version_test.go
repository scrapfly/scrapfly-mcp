package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// repoFile reads a file from the repo root (two levels up from pkg/server).
func repoFile(t *testing.T, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("..", "..", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return buf
}

// package.json is the release source of truth (`make bump` / `make release`
// drive it). server.json is what the MCP registry serves, and directories
// such as Glama sync their index — and their tool-quality scores — from that
// registry entry. When the two drifted, the registry stayed pinned on 1.0.0
// while npm shipped 1.3.x, so every directory scored a tool surface roughly a
// year old. ServerVersion is the third copy, reported in `initialize`.
//
// `make sync-version` rewrites the latter two from package.json; this test is
// what makes forgetting to run it loud.
func TestVersionsAreInSync(t *testing.T) {
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(repoFile(t, "package.json"), &pkg); err != nil {
		t.Fatalf("parse package.json: %v", err)
	}
	if pkg.Version == "" {
		t.Fatal("package.json has no version")
	}

	var manifest struct {
		Version  string `json:"version"`
		Packages []struct {
			Identifier string `json:"identifier"`
			Version    string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(repoFile(t, "server.json"), &manifest); err != nil {
		t.Fatalf("parse server.json: %v", err)
	}

	if manifest.Version != pkg.Version {
		t.Errorf("server.json version %q != package.json %q — run `make sync-version`", manifest.Version, pkg.Version)
	}
	for _, p := range manifest.Packages {
		if p.Version != pkg.Version {
			t.Errorf("server.json package %q version %q != package.json %q — run `make sync-version`", p.Identifier, p.Version, pkg.Version)
		}
	}
	if ServerVersion != pkg.Version {
		t.Errorf("ServerVersion %q != package.json %q — run `make sync-version`", ServerVersion, pkg.Version)
	}
}
