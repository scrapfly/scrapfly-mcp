package scrapflyprovider

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
)

func screenshotProvider(t *testing.T, status int, contentType, body string) *ScrapflyToolProvider {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	client, err := scrapfly.NewWithHost("scp-test-key", server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	getter := func(p *ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error) {
		return client, nil
	}
	return NewScrapflyToolProvider(client, getter, log.New(io.Discard, "", 0))
}

func TestScreenshotWithAWebhookReturnsItsJob(t *testing.T) {
	provider := screenshotProvider(t, http.StatusCreated, "application/json; charset=utf-8", `{"webhook_enqueued":true,"job_uuid":"job-1"}`)

	result, _, err := provider.Screenshot(context.Background(), nil, ScreenshotToolInput{URL: "https://web-scraping.dev/product/1", Webhook: "my-webhook"})
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if result.IsError {
		t.Fatalf("a queued screenshot is an error: %+v", result.Content)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content: %d parts, want the job alone", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content is %T, want text", result.Content[0])
	}
	if !strings.Contains(text.Text, "job-1") || !strings.Contains(text.Text, "my-webhook") {
		t.Fatalf("the job is not named: %q", text.Text)
	}
}

func TestScreenshotReturnsItsImage(t *testing.T) {
	provider := screenshotProvider(t, http.StatusOK, "image/png", "\x89PNG\r\n\x1a\nimage")

	result, _, err := provider.Screenshot(context.Background(), nil, ScreenshotToolInput{URL: "https://web-scraping.dev/product/1"})
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	var image *mcp.ImageContent
	for _, part := range result.Content {
		if found, ok := part.(*mcp.ImageContent); ok {
			image = found
		}
	}
	if image == nil || image.MIMEType != "image/png" || !strings.HasPrefix(string(image.Data), "\x89PNG") {
		t.Fatalf("no png image in %+v", result.Content)
	}
}
