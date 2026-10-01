// Package browser provides Cloud Browser session management, CDP communication,
// page state tracking, and WebMCP tool discovery/proxy.
package browser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// NoOwner is a sentinel owner tag that matches no stored session. A caller
// whose API key cannot be resolved gets this, so a lookup fails closed
// instead of falling through to another caller's session.
const NoOwner = "\x00no-owner"

// OwnerKey derives a stable, non-reversible tag from an API key. Sessions are
// tagged with it on open so one key's sessions stay invisible to another in
// multi-caller HTTP mode; single-key modes resolve to one tag, making the
// scoping a no-op. Empty key -> empty tag ("match any", for the local
// single-tenant playground routes that have no caller key).
func OwnerKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:8])
}

// Session tracks a live Cloud Browser session with an active CDP WebSocket.
type Session struct {
	SessionID        string
	Owner            string // OwnerKey() of the API key that opened it; "" == unowned/legacy
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

// FindSession looks up a browser session by ID, scoped to owner. If sessionID
// is empty it returns an active session belonging to owner (non-deterministic
// if that owner has several). owner == "" matches any session, for the local
// single-tenant playground routes; a non-empty owner never resolves a session
// tagged with a different owner, so callers in multi-caller HTTP mode cannot
// see or drive each other's sessions.
func FindSession(owner, sessionID string) (*Session, error) {
	if sessionID != "" {
		val, ok := Store.Load(sessionID)
		if !ok {
			return nil, fmt.Errorf("session %s not found", sessionID)
		}
		s := val.(*Session)
		if owner != "" && s.Owner != "" && s.Owner != owner {
			// Hide another owner's session behind the same not-found error so
			// the ID space can't be probed for liveness.
			return nil, fmt.Errorf("session %s not found", sessionID)
		}
		return s, nil
	}
	// Fallback: return an active session for this owner with a live connection.
	// Clean up dead sessions as we go.
	var session *Session
	var deadKeys []any
	Store.Range(func(key, value any) bool {
		s := value.(*Session)
		if s.CdpConn == nil {
			deadKeys = append(deadKeys, key)
			return true // skip dead sessions
		}
		if owner != "" && s.Owner != owner {
			return true // not this caller's session
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
