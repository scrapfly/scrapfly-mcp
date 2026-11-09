package scrapflyprovider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/browser"
)

const pngHeader = "\x89PNG\r\n\x1a\n"

// cdpSession stores a session whose browser answers every command with data.
func cdpSession(t *testing.T, id, owner, data string) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var command struct {
				ID int64 `json:"id"`
			}
			if err := json.Unmarshal(raw, &command); err != nil {
				return
			}
			answer := fmt.Sprintf(`{"id":%d,"result":{"data":%q}}`, command.ID, data)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(answer)); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	session := &browser.Session{SessionID: id, Owner: owner, CdpConn: conn, ExpiresAt: time.Now().Add(time.Hour)}
	session.StartReader()
	browser.Store.Store(id, session)
	t.Cleanup(session.Close)
}

func keyedProvider(t *testing.T) *ScrapflyToolProvider {
	t.Helper()
	client, err := scrapfly.NewWithHost("scp-test-key", "https://web-scraping.dev", false)
	if err != nil {
		t.Fatal(err)
	}
	getter := func(p *ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error) {
		return client, nil
	}
	return NewScrapflyToolProvider(client, getter, log.New(io.Discard, "", 0))
}

func assertPNG(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	if result.IsError {
		t.Fatalf("failed: %+v", result.Content)
	}
	for _, part := range result.Content {
		if image, ok := part.(*mcp.ImageContent); ok {
			if !strings.HasPrefix(string(image.Data), pngHeader) {
				t.Fatalf("image data starts with %q", image.Data[:min(len(image.Data), 16)])
			}
			return
		}
	}
	t.Fatalf("no image in %+v", result.Content)
}

func assertNoImage(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	if !result.IsError {
		t.Fatal("not a tool error")
	}
	for _, part := range result.Content {
		if _, ok := part.(*mcp.ImageContent); ok {
			t.Fatalf("an image went out: %+v", result.Content)
		}
	}
}

func staticScreenshot(t *testing.T, provider *ScrapflyToolProvider) *mcp.CallToolResult {
	t.Helper()
	tool := browserInteractionTools(provider)["take_screenshot"]
	if tool == nil {
		t.Fatal("no take_screenshot tool")
	}
	result, err := tool.Handler(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)}})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCloudBrowserScreenshotReturnsTheImageBytes(t *testing.T) {
	provider := keyedProvider(t)
	cdpSession(t, t.Name(), "", base64.StdEncoding.EncodeToString([]byte(pngHeader+"image")))

	result, _, err := provider.CloudBrowserScreenshot(context.Background(), nil, CloudBrowserScreenshotInput{SessionID: t.Name()})
	if err != nil {
		t.Fatal(err)
	}
	assertPNG(t, result)
}

func TestCloudBrowserScreenshotRefusesDataItCannotDecode(t *testing.T) {
	provider := keyedProvider(t)
	cdpSession(t, t.Name(), "", "not base64!")

	result, _, err := provider.CloudBrowserScreenshot(context.Background(), nil, CloudBrowserScreenshotInput{SessionID: t.Name()})
	if err != nil {
		t.Fatal(err)
	}
	assertNoImage(t, result)
}

func TestStaticTakeScreenshotReturnsTheImageBytes(t *testing.T) {
	cdpSession(t, t.Name(), browser.OwnerKey("scp-test-key"), base64.StdEncoding.EncodeToString([]byte(pngHeader+"image")))

	assertPNG(t, staticScreenshot(t, keyedProvider(t)))
}

func TestStaticTakeScreenshotRefusesDataItCannotDecode(t *testing.T) {
	cdpSession(t, t.Name(), browser.OwnerKey("scp-test-key"), "not base64!")

	assertNoImage(t, staticScreenshot(t, keyedProvider(t)))
}

// sessionScreenshot calls the take_screenshot tool registered for a session.
func sessionScreenshot(t *testing.T) *mcp.CallToolResult {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	var names []string
	browser.RegisterCDPTools(log.New(io.Discard, "", 0), server, t.Name(), "abc", &names, nil)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "1"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "browser_abc_take_screenshot"})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSessionTakeScreenshotToolReturnsTheImageBytes(t *testing.T) {
	cdpSession(t, t.Name(), "", base64.StdEncoding.EncodeToString([]byte(pngHeader+"image")))

	assertPNG(t, sessionScreenshot(t))
}

func TestSessionTakeScreenshotToolRefusesDataItCannotDecode(t *testing.T) {
	cdpSession(t, t.Name(), "", "not base64!")

	assertNoImage(t, sessionScreenshot(t))
}
