// Package browser provides Cloud Browser session management, CDP communication,
// page state tracking, and WebMCP tool discovery/proxy.
package browser

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Session tracks a live Cloud Browser session with an active CDP WebSocket.
type Session struct {
	SessionID        string
	MCPEndpoint      string
	WSURL            string
	ToolNames        []string        // namespaced tool names registered on the MCP server
	ExpiresAt        time.Time       // browser timeout
	CdpConn          *websocket.Conn // live CDP WebSocket connection
	CdpMu            sync.Mutex      // protects CdpConn writes
	CdpID            atomic.Int64    // CDP message ID counter
	CdpPageSessionID string          // flattened session ID for page-level CDP commands

	// CDP multiplexer state (managed by StartReader)
	pending       map[int64]*pendingRequest
	pendingMu     sync.Mutex
	eventHandlers map[string][]EventHandler
	handlersMu    sync.RWMutex
	readerDone    chan struct{}

	// CancelCleanup cancels the auto-cleanup goroutine when the session is
	// closed manually (before timeout expiry).
	CancelCleanup func()

	// Page state — maintained across tool calls.
	Page PageState
}

// Store is a per-provider in-memory store of active browser sessions.
// Thread-safe via sync.Map. Keyed by session_id.
var Store sync.Map

// FindSession looks up a browser session by ID. If sessionID is empty,
// returns the first active session found (non-deterministic if multiple exist).
func FindSession(sessionID string) (*Session, error) {
	if sessionID != "" {
		val, ok := Store.Load(sessionID)
		if !ok {
			return nil, fmt.Errorf("session %s not found", sessionID)
		}
		return val.(*Session), nil
	}
	// Fallback: return the most recent active session with a live connection.
	// Clean up dead sessions as we go.
	var session *Session
	var deadKeys []any
	Store.Range(func(key, value any) bool {
		s := value.(*Session)
		if s.CdpConn == nil {
			deadKeys = append(deadKeys, key)
			return true // skip dead sessions
		}
		// Check if connection is alive by checking if the reader is still running
		select {
		case <-s.readerDone:
			// Reader has stopped — connection is dead
			deadKeys = append(deadKeys, key)
			return true
		default:
			// Reader still running — connection is alive
			session = s
			return false
		}
	})
	// Remove dead sessions
	for _, k := range deadKeys {
		Store.Delete(k)
	}
	if session == nil {
		return nil, fmt.Errorf("no active browser session")
	}
	return session, nil
}

// Close closes the CDP WebSocket, removes the session from the store,
// and cancels the auto-cleanup goroutine.
func (s *Session) Close() {
	if s.CancelCleanup != nil {
		s.CancelCleanup()
	}
	Store.Delete(s.SessionID)
	if s.CdpConn != nil {
		s.CdpConn.Close()
	}
}

// cdpResponse and pendingRequest types are in cdp.go
