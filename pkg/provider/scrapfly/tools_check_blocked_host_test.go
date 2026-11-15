package scrapflyprovider

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/scrapfly/go-scrapfly"
)

// check_if_blocked calls /classify outside the SDK client: it must reach the host the binary was started with,
// not the default API host.
func TestCheckIfBlockedCallsTheConfiguredAPIHost(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify" {
			t.Errorf("path = %q, want /classify", r.URL.Path)
		}
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"blocked": false})
	}))
	defer server.Close()
	t.Setenv(classifyAPIHostEnv, "")

	client, err := scrapfly.New("test-key")
	if err != nil {
		t.Fatalf("scrapfly.New: %v", err)
	}
	p := NewScrapflyToolProvider(client, GetDefaultScrapflyClient, log.New(io.Discard, "", 0))
	p.APIHost = server.URL

	_, _, err = p.CheckIfBlocked(context.Background(), newCallToolRequest(), CheckIfBlockedInput{
		URL:        "https://web-scraping.dev/product/1",
		StatusCode: 200,
	})
	if err != nil {
		t.Fatalf("CheckIfBlocked: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("configured host got %d requests, want 1", hits.Load())
	}
}
