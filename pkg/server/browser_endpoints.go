package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly/browser"
)

// RegisterBrowserEndpoints attaches the /browser/* HTTP routes to the
// provided mux. They back a browser playground UI (screencast, downloads,
// expects, but use the in-process browser.Session store (no gRPC, no
// store: no session broker in front, and no auth layer of their own.
// The trust boundary is therefore this process, which holds at most one
// browser — bind the listener to loopback, never a public interface.
//
// Routes:
//
//	GET /browser/screencast?session_id=...   — SSE: JPEG frames as `event: frame`
//	GET /browser/downloads?session_id=...    — JSON: download manifest
//	GET /browser/download?session_id=&filename=...  — JSON: base64 file payload
//	GET /browser/captchas?session_id=...     — JSON: empty list (captcha records
//	    are not recorded by this server; we always return {"records": []}
//	    so a client that polls the route gets a valid empty answer, not a 404).
//	GET /browser/active                      — JSON: {"session_id": "...", "url": "..."}
//	    or {} if no session — used by the playground UI to reattach to an
//	    in-progress session after a page reload.
//	GET /browser/screenshot?session_id=...   — JSON: {"data": "<base64 PNG>",
//	    "mime": "image/png"}; one-shot capture via CDP Page.captureScreenshot.
//	    Skips the LLM round-trip used by the in-tool cloud_browser_screenshot,
//	    so the playground's "Capture frame" button is instant + free.
//
// `session_id` is optional everywhere — when omitted, the most recent
// active session is used (FindSession's empty-id semantics). That is
// the right behavior for this server, which holds at most
// one concurrent session per MCP process.
func RegisterBrowserEndpoints(mux *http.ServeMux) {
	mux.HandleFunc("/browser/screencast", handleBrowserScreencast)
	mux.HandleFunc("/browser/downloads", handleBrowserDownloads)
	mux.HandleFunc("/browser/download", handleBrowserDownload)
	mux.HandleFunc("/browser/captchas", handleBrowserCaptchas)
	mux.HandleFunc("/browser/active", handleBrowserActive)
	mux.HandleFunc("/browser/screenshot", handleBrowserScreenshot)
}

func writeJSONErr(w http.ResponseWriter, err error, status int) {
	body, _ := json.Marshal(map[string]string{"error": err.Error()})
	http.Error(w, string(body), status)
}

func handleBrowserScreencast(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	session, err := browser.FindSession(r.URL.Query().Get("session_id"))
	if err != nil {
		writeJSONErr(w, err, http.StatusNotFound)
		return
	}

	// CDP screencast frames arrive on a goroutine the Session owns —
	// it keeps producing even after this handler returns (StopScreencast
	// is called below, but it's racy with in-flight frames). Without a
	// guard, a frame that lands while the http.ResponseWriter is being
	// torn down hits a nil bufio.Writer and crashes the whole process.
	//
	// Three protections:
	//   1) Atomic `stopped` flag flipped before StopScreencast — frames
	//      that win the race do nothing.
	//   2) ctx.Err() check — never write after the client disconnects.
	//   3) `defer recover()` — last line of defense; we'd rather drop a
	//      frame than panic the MCP server.
	ctx := r.Context()
	var stopped atomic.Bool
	session.StartScreencast("jpeg", 100, func(frame browser.ScreencastFrame) {
		if stopped.Load() || ctx.Err() != nil {
			return
		}
		defer func() {
			if rec := recover(); rec != nil {
				// Frame dropped — see comment above. Avoids cascading
				// the panic into the goroutine and killing the daemon.
				_ = rec
			}
		}()
		data, _ := json.Marshal(map[string]string{"data": frame.Data})
		fmt.Fprintf(w, "event: frame\ndata: %s\n\n", string(data))
		flusher.Flush()
	})
	<-ctx.Done()
	stopped.Store(true)
	session.StopScreencast()
}

func handleBrowserDownloads(w http.ResponseWriter, r *http.Request) {
	session, err := browser.FindSession(r.URL.Query().Get("session_id"))
	if err != nil {
		writeJSONErr(w, err, http.StatusNotFound)
		return
	}
	downloads, err := session.ListDownloads()
	if err != nil {
		writeJSONErr(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(downloads)
}

func handleBrowserDownload(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, `{"error":"filename is required"}`, http.StatusBadRequest)
		return
	}
	session, err := browser.FindSession(r.URL.Query().Get("session_id"))
	if err != nil {
		writeJSONErr(w, err, http.StatusNotFound)
		return
	}
	data, err := session.GetDownload(filename)
	if err != nil {
		writeJSONErr(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"filename": filename, "data": data, "encoding": "base64"})
}

func handleBrowserCaptchas(w http.ResponseWriter, r *http.Request) {
	// Captcha records require the Antibot CDP domain plumbed through
	// a capture layer this server does not ship, so nothing is ever recorded.
	// Returning an empty list rather than an error keeps a client's
	// "no captcha seen" path working without a 404 cascade.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"records": []any{}})
}

// handleBrowserScreenshot issues a one-shot CDP Page.captureScreenshot on
// the active session and returns the PNG as base64. The playground's
// "Capture frame" button calls this directly — no LLM in the loop, so
// it's instant and doesn't burn agent tokens. Mirrors the in-tool
// `take_screenshot` CDP path exactly.
func handleBrowserScreenshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	session, err := browser.FindSession(r.URL.Query().Get("session_id"))
	if err != nil {
		writeJSONErr(w, err, http.StatusNotFound)
		return
	}
	raw, err := session.SendCDP("Page.captureScreenshot", map[string]any{"format": "png"})
	if err != nil {
		writeJSONErr(w, err, http.StatusBadGateway)
		return
	}
	var ss struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(raw, &ss)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": ss.Data,
		"mime": "image/png",
		"url":  session.Page.URL,
	})
}

// handleBrowserActive returns metadata for the currently-active browser
// session, or an empty object when none. The playground polls this on
// page load so a refresh mid-session reattaches the screencast and
// captured-downloads pane without losing context.
func handleBrowserActive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	session, err := browser.FindSession("")
	if err != nil || session == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"session_id": session.SessionID,
		"url":        session.Page.URL,
		"title":      session.Page.Title,
	})
}
