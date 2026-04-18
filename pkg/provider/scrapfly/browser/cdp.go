package browser

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// cdpResponse is the common CDP response/event shape.
type cdpResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// pendingRequest tracks a CDP command waiting for its response.
type pendingRequest struct {
	ch chan cdpResponse
}

// EventHandler is called for each CDP event. Return true to keep listening.
type EventHandler func(method string, params json.RawMessage) bool

// StartReader starts the background CDP reader goroutine.
// Must be called once after the WebSocket connection is established.
// The reader dispatches responses to waiting SendCDP callers and events
// to registered handlers. This eliminates read races between concurrent callers.
func (s *Session) StartReader() {
	s.pending = make(map[int64]*pendingRequest)
	s.eventHandlers = make(map[string][]EventHandler)
	s.readerDone = make(chan struct{})

	// WebSocket keepalive — send ping every 5s to prevent proxy idle disconnect
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.readerDone:
				return
			case <-ticker.C:
				s.CdpMu.Lock()
				err := s.CdpConn.WriteMessage(websocket.PingMessage, nil)
				s.CdpMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer close(s.readerDone)
		for {
			_, raw, err := s.CdpConn.ReadMessage()
			if err != nil {
				log.Printf("[CDP] Reader stopped: %v", err)
				// Unblock all pending requests
				s.pendingMu.Lock()
				for id, req := range s.pending {
					req.ch <- cdpResponse{Error: &struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					}{Code: -1, Message: fmt.Sprintf("connection closed: %v", err)}}
					delete(s.pending, id)
				}
				s.pendingMu.Unlock()
				return
			}

			var resp cdpResponse
			if err := json.Unmarshal(raw, &resp); err != nil {
				log.Printf("[CDP] unmarshal error: %v", err)
				continue
			}
			// Log every raw message for debugging
			if resp.ID == 0 && resp.Method == "" {
				rawStr := string(raw)
				if len(rawStr) > 300 { rawStr = rawStr[:300] + "..." }
				log.Printf("[CDP RAW] no id/method: %s", rawStr)
			}

			// Response to a pending command
			if resp.ID != 0 {
				s.pendingMu.Lock()
				req, found := s.pending[resp.ID]
				if found {
					delete(s.pending, resp.ID)
				}
				s.pendingMu.Unlock()
				if found {
					if resp.Error != nil {
						log.Printf("[CDP RECV] id=%d ERROR %d: %s", resp.ID, resp.Error.Code, resp.Error.Message)
					} else {
						log.Printf("[CDP RECV] id=%d OK len=%d", resp.ID, len(resp.Result))
					}
					req.ch <- resp
				} else {
					log.Printf("[CDP RECV] id=%d (orphan — no pending handler)", resp.ID)
				}
				continue
			}

			// Event — dispatch to handlers.
			// Handlers run in goroutines to avoid blocking the reader (some call SendCDP).
			// Handlers returning false are flagged for removal after all goroutines complete.
			if resp.Method != "" {
				log.Printf("[CDP EVENT] %s (params=%d bytes)", resp.Method, len(resp.Params))
				method := resp.Method
				params := resp.Params

				s.handlersMu.RLock()
				handlers := make([]EventHandler, len(s.eventHandlers[method]))
				copy(handlers, s.eventHandlers[method])
				wildcards := make([]EventHandler, len(s.eventHandlers["*"]))
				copy(wildcards, s.eventHandlers["*"])
				s.handlersMu.RUnlock()

				// Track which handlers to remove
				type removal struct {
					idx int
				}
				removals := make(chan removal, len(handlers))
				var wg sync.WaitGroup

				for i, h := range handlers {
					wg.Add(1)
					go func(idx int, handler EventHandler) {
						defer wg.Done()
						if !handler(method, params) {
							removals <- removal{idx}
						}
					}(i, h)
				}
				for _, h := range wildcards {
					go h(method, params)
				}

				// Wait for all handlers, then remove dead ones
				go func() {
					wg.Wait()
					close(removals)
					var toRemove []int
					for r := range removals {
						toRemove = append(toRemove, r.idx)
					}
					if len(toRemove) > 0 {
						s.handlersMu.Lock()
						current := s.eventHandlers[method]
						var kept []EventHandler
						for i, h := range current {
							remove := false
							for _, ri := range toRemove {
								if ri == i {
									remove = true
									break
								}
							}
							if !remove {
								kept = append(kept, h)
							}
						}
						s.eventHandlers[method] = kept
						s.handlersMu.Unlock()
					}
				}()
			}
		}
	}()
}

// OnEvent registers an event handler for a specific CDP event method.
// Use "*" to receive all events. Return false from the handler to unregister.
func (s *Session) OnEvent(method string, handler EventHandler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.eventHandlers[method] = append(s.eventHandlers[method], handler)
}


// SendCDPFireAndForget sends a CDP command without waiting for a response.
// Used for acks and other fire-and-forget messages that don't return results.
func (s *Session) SendCDPFireAndForget(method string, params any) {
	id := s.CdpID.Add(1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if s.CdpPageSessionID != "" {
		msg["sessionId"] = s.CdpPageSessionID
	}
	s.CdpMu.Lock()
	s.CdpConn.WriteJSON(msg)
	s.CdpMu.Unlock()
}

// sendAndWait sends a CDP message and waits for the matching response.
func (s *Session) sendAndWait(msg map[string]any, id int64) (json.RawMessage, error) {
	req := &pendingRequest{ch: make(chan cdpResponse, 1)}

	s.pendingMu.Lock()
	s.pending[id] = req
	s.pendingMu.Unlock()

	msgJSON, _ := json.Marshal(msg)
	log.Printf("[CDP SEND] %s", string(msgJSON))

	s.CdpMu.Lock()
	err := s.CdpConn.WriteJSON(msg)
	s.CdpMu.Unlock()
	if err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("CDP write: %w", err)
	}

	resp := <-req.ch
	if resp.Error != nil {
		log.Printf("[CDP RESP] id=%d ERROR %d: %s", id, resp.Error.Code, resp.Error.Message)
		return nil, fmt.Errorf("CDP error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	log.Printf("[CDP RESP] id=%d OK len=%d", id, len(resp.Result))
	return resp.Result, nil
}

// SendCDP sends a CDP command scoped to the page session and waits for the response.
func (s *Session) SendCDP(method string, params any) (json.RawMessage, error) {
	id := s.CdpID.Add(1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if s.CdpPageSessionID != "" {
		msg["sessionId"] = s.CdpPageSessionID
	}
	return s.sendAndWait(msg, id)
}

// SendCDPBrowser sends a CDP command to the browser process (no sessionId).
func (s *Session) SendCDPBrowser(method string, params any) (json.RawMessage, error) {
	id := s.CdpID.Add(1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	return s.sendAndWait(msg, id)
}

// SendCDPCollectEvents sends a CDP command and collects matching events
// that arrive before the response.
func (s *Session) SendCDPCollectEvents(method string, params any, eventName string) (json.RawMessage, []json.RawMessage, error) {
	id := s.CdpID.Add(1)
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if s.CdpPageSessionID != "" {
		msg["sessionId"] = s.CdpPageSessionID
	}

	// Collect events before the response arrives
	var events []json.RawMessage
	var eventsMu sync.Mutex
	done := make(chan struct{})

	// Register temporary event handler
	s.handlersMu.Lock()
	handlerIdx := len(s.eventHandlers[eventName])
	s.eventHandlers[eventName] = append(s.eventHandlers[eventName], func(m string, p json.RawMessage) bool {
		select {
		case <-done:
			return false // stop after response
		default:
			eventsMu.Lock()
			events = append(events, p)
			log.Printf("[CDP] Collected event: %s (count=%d)", eventName, len(events))
			eventsMu.Unlock()
			return true
		}
	})
	s.handlersMu.Unlock()

	// Send the command
	result, err := s.sendAndWait(msg, id)

	// Stop collecting
	close(done)

	// Remove the temporary handler
	s.handlersMu.Lock()
	handlers := s.eventHandlers[eventName]
	if handlerIdx < len(handlers) {
		s.eventHandlers[eventName] = append(handlers[:handlerIdx], handlers[handlerIdx+1:]...)
	}
	s.handlersMu.Unlock()

	return result, events, err
}
