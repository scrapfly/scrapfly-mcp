package browser

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/accessibility"
)

// WebMCPToolInfo describes a page-registered WebMCP tool discovered via toolsAdded events.
type WebMCPToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	FrameID     string          `json:"frameId"`
}

// PageState tracks the current browser page state, refreshed after navigations and interactions.
type PageState struct {
	mu          sync.Mutex
	URL         string
	Title       string
	AXTree      string // compact AX tree text for LLM consumption
	FrameID     string
	WebMCPTools []WebMCPToolInfo // page-registered tools from WebMCP.toolsAdded
}

// axValueString extracts a string from an accessibility.Value.
// The Value field is jsontext.Value (raw JSON bytes), so we unquote it.
func axValueString(v *accessibility.Value) string {
	if v == nil || len(v.Value) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal([]byte(v.Value), &s) == nil {
		return s
	}
	// Fallback: return the raw JSON trimmed of quotes
	return strings.Trim(string(v.Value), `"`)
}

// axValueBool extracts a bool from an accessibility.Value.
func axValueBool(v *accessibility.Value) bool {
	if v == nil || len(v.Value) == 0 {
		return false
	}
	var b bool
	if json.Unmarshal([]byte(v.Value), &b) == nil {
		return b
	}
	return false
}

// AddWebMCPTools appends discovered tools, skipping duplicates and browser-frame tools.
func (p *PageState) AddWebMCPTools(tools []WebMCPToolInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range tools {
		if t.FrameID == "browser" {
			continue
		}
		found := false
		for _, existing := range p.WebMCPTools {
			if existing.Name == t.Name {
				found = true
				break
			}
		}
		if !found {
			p.WebMCPTools = append(p.WebMCPTools, t)
		}
	}
}

// RemoveWebMCPTools removes tools by name.
func (p *PageState) RemoveWebMCPTools(names []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	filtered := p.WebMCPTools[:0]
	for _, t := range p.WebMCPTools {
		remove := false
		for _, name := range names {
			if t.Name == name {
				remove = true
				break
			}
		}
		if !remove {
			filtered = append(filtered, t)
		}
	}
	p.WebMCPTools = filtered
}

// GetWebMCPTools returns the current list of page-registered tools.
func (p *PageState) GetWebMCPTools() []WebMCPToolInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]WebMCPToolInfo, len(p.WebMCPTools))
	copy(result, p.WebMCPTools)
	return result
}

// ClearWebMCPTools removes all stored tools (called on navigation).
func (p *PageState) ClearWebMCPTools() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.WebMCPTools = nil
}

// Refresh re-fetches page metadata and AX tree from the browser.
func (p *PageState) Refresh(session *Session) {
	p.mu.Lock()
	defer p.mu.Unlock()
	log.Printf("[Page] Refreshing page state...")

	// Get metadata
	metaResult, _ := session.SendCDP("Runtime.evaluate", map[string]any{
		"expression":    `JSON.stringify({title: document.title, url: location.href})`,
		"returnByValue": true,
	})
	var meta struct {
		Result struct{ Value string `json:"value"` } `json:"result"`
	}
	json.Unmarshal(metaResult, &meta)
	var pageMeta struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	json.Unmarshal([]byte(meta.Result.Value), &pageMeta)
	p.URL = pageMeta.URL
	p.Title = pageMeta.Title

	// Get frame ID
	frameResult, _ := session.SendCDP("Page.getFrameTree", nil)
	if frameResult != nil {
		var ft struct {
			FrameTree struct {
				Frame struct{ Id string `json:"id"` } `json:"frame"`
			} `json:"frameTree"`
		}
		json.Unmarshal(frameResult, &ft)
		p.FrameID = ft.FrameTree.Frame.Id
	}

	// Get AX tree
	axParams := map[string]any{"depth": 10}
	if p.FrameID != "" {
		axParams["frameId"] = p.FrameID
	}
	axResult, err := session.SendCDP("Accessibility.getFullAXTree", axParams)
	if err != nil {
		// Fallback to text
		textResult, _ := session.SendCDP("Runtime.evaluate", map[string]any{
			"expression":    `document.body?.innerText?.substring(0, 50000) || ''`,
			"returnByValue": true,
		})
		var textEval struct {
			Result struct{ Value string `json:"value"` } `json:"result"`
		}
		json.Unmarshal(textResult, &textEval)
		p.AXTree = textEval.Result.Value
		return
	}

	var axTree struct {
		Nodes []*accessibility.Node `json:"nodes"`
	}
	json.Unmarshal(axResult, &axTree)

	skipRoles := map[string]bool{
		"none": true, "generic": true, "InlineTextBox": true,
		"LineBreak": true, "StaticText": true, "paragraph": true,
		"LayoutTable": true, "LayoutTableRow": true, "LayoutTableCell": true,
	}

	// Collect every backendDOMNodeId we'll annotate with compound-
	// component metadata (input/select/textarea attributes that the
	// AX tree alone doesn't surface — type, placeholder, min/max/step,
	// readonly, select.options). See compoundMeta + collectCompoundMeta
	// below.
	candidateBackendIDs := make([]int64, 0)
	candidateRoles := make(map[int64]string)
	for _, node := range axTree.Nodes {
		role := axValueString(node.Role)
		if role == "textbox" || role == "combobox" || role == "checkbox" || role == "radio" || role == "spinbutton" || role == "slider" || role == "searchbox" {
			if id := int64(node.BackendDOMNodeID); id > 0 {
				candidateBackendIDs = append(candidateBackendIDs, id)
				candidateRoles[id] = role
			}
		}
	}
	compoundByID := p.collectCompoundMeta(session, candidateBackendIDs)

	var sb strings.Builder
	for _, node := range axTree.Nodes {
		if node.Ignored {
			continue
		}
		role := axValueString(node.Role)
		if skipRoles[role] {
			continue
		}
		name := axValueString(node.Name)
		value := axValueString(node.Value)
		if name == "" && value == "" && role != "textbox" && role != "button" && role != "link" && role != "checkbox" && role != "radio" && role != "combobox" {
			continue
		}
		line := fmt.Sprintf("id=%s %s", node.NodeID, role)
		if name != "" {
			line += fmt.Sprintf(` "%s"`, name)
		}
		if value != "" {
			line += fmt.Sprintf(` value="%s"`, value)
		}
		for _, prop := range node.Properties {
			switch string(prop.Name) {
			case "focused", "required", "disabled", "checked":
				if axValueBool(prop.Value) {
					line += " " + string(prop.Name)
				}
			}
		}
		// Compound-component enrichment — adds the attributes the
		// model actually needs to plan the next action (what kind of
		// input is this, what values are accepted, what options exist
		// on a <select>, …). All optional; we only emit fields that
		// were actually present on the underlying element.
		if meta, ok := compoundByID[int64(node.BackendDOMNodeID)]; ok {
			if meta.Type != "" {
				line += fmt.Sprintf(` type="%s"`, meta.Type)
			}
			if meta.Placeholder != "" {
				line += fmt.Sprintf(` placeholder="%s"`, escapeQuotes(meta.Placeholder))
			}
			if meta.Min != "" {
				line += fmt.Sprintf(` min="%s"`, meta.Min)
			}
			if meta.Max != "" {
				line += fmt.Sprintf(` max="%s"`, meta.Max)
			}
			if meta.Step != "" {
				line += fmt.Sprintf(` step="%s"`, meta.Step)
			}
			if meta.Pattern != "" {
				line += fmt.Sprintf(` pattern="%s"`, escapeQuotes(meta.Pattern))
			}
			if meta.Readonly {
				line += " readonly"
			}
			if meta.Multiple {
				line += " multiple"
			}
			if len(meta.Options) > 0 {
				line += fmt.Sprintf(` options="%s"`, strings.Join(meta.Options, "|"))
			}
			if meta.Files > 0 {
				line += fmt.Sprintf(` files=%d`, meta.Files)
				if len(meta.FilesNames) > 0 {
					line += fmt.Sprintf(` filenames="%s"`, strings.Join(meta.FilesNames, "|"))
				}
			}
		}
		sb.WriteString(line + "\n")
	}
	p.AXTree = sb.String()
	log.Printf("[Page] Refresh done: url=%s title=%s axNodes=%d compoundEnriched=%d",
		p.URL, p.Title, len(axTree.Nodes), len(compoundByID))
}

// compoundMeta carries the per-element form-control attributes that
// the AX tree alone doesn't surface. Populated by collectCompoundMeta
// via per-element Runtime.callFunctionOn calls and rendered into the
// snapshot text alongside the AX role/name.
type compoundMeta struct {
	Type        string   `json:"type"`
	Placeholder string   `json:"placeholder"`
	Min         string   `json:"min"`
	Max         string   `json:"max"`
	Step        string   `json:"step"`
	Pattern     string   `json:"pattern"`
	Readonly    bool     `json:"readonly"`
	Multiple    bool     `json:"multiple"`
	Files       int      `json:"files"`
	Options     []string `json:"options"`
	FilesNames  []string `json:"filesNames"`
}

// collectCompoundMeta runs a single Runtime.evaluate over the page
// to collect input/select compound metadata for the given
// backendDOMNodeIds. Returns a map keyed by backendDOMNodeId.
//
// The script is a single function call that walks the document for
// every form-control element, identifies it via window.__cdp_findByBE
// (a temp helper), and returns a JSON-friendly structure. Falls back
// to per-element queries if the batched call fails — never errors;
// a missing entry just means the model loses one enrichment field.
func (p *PageState) collectCompoundMeta(session *Session, backendIDs []int64) map[int64]compoundMeta {
	out := make(map[int64]compoundMeta)
	if len(backendIDs) == 0 {
		return out
	}
	// Use DOM.resolveNode + Runtime.callFunctionOn per id. The
	// alternative (single Runtime.evaluate) can't address elements by
	// backendDOMNodeId — it has to walk the DOM, which is fragile
	// against shadow roots. Per-call is cheap (each is <2ms) and there
	// are typically <20 form controls per page.
	for _, bid := range backendIDs {
		resolveResult, err := session.SendCDP("DOM.resolveNode", map[string]any{
			"backendNodeId": bid,
		})
		if err != nil {
			continue
		}
		var resolved struct {
			Object struct {
				ObjectID string `json:"objectId"`
			} `json:"object"`
		}
		if err := json.Unmarshal(resolveResult, &resolved); err != nil || resolved.Object.ObjectID == "" {
			continue
		}
		callResult, err := session.SendCDP("Runtime.callFunctionOn", map[string]any{
			"objectId":      resolved.Object.ObjectID,
			"functionDeclaration": _compoundMetaJSFn,
			"returnByValue": true,
			"silent":        true,
		})
		if err != nil {
			continue
		}
		var rv struct {
			Result struct {
				Value compoundMeta `json:"value"`
			} `json:"result"`
		}
		if err := json.Unmarshal(callResult, &rv); err != nil {
			continue
		}
		out[bid] = rv.Result.Value
	}
	return out
}

// _compoundMetaJSFn is the JS expression Runtime.callFunctionOn runs
// on each form-control element. Pure read; no side effects. Returns
// a stable shape (matches the Go compoundMeta struct).
const _compoundMetaJSFn = `function(){
  const e = this;
  const tag = (e.tagName || '').toLowerCase();
  const out = { type:'', placeholder:'', min:'', max:'', step:'', pattern:'',
                readonly:false, multiple:false, files:0, options:[], filesNames:[] };
  if (tag === 'select') {
    out.type = 'select';
    out.multiple = !!e.multiple;
    const opts = [];
    for (const o of e.options) {
      const v = (o.value !== '' ? o.value : o.text) || '';
      if (v) opts.push(v.length > 40 ? v.slice(0,40)+'…' : v);
      if (opts.length >= 50) { opts.push('…'); break; }
    }
    out.options = opts;
  } else if (tag === 'input' || tag === 'textarea') {
    out.type = (e.getAttribute('type') || tag).toLowerCase();
    out.placeholder = e.getAttribute('placeholder') || '';
    out.min = e.getAttribute('min') || '';
    out.max = e.getAttribute('max') || '';
    out.step = e.getAttribute('step') || '';
    out.pattern = e.getAttribute('pattern') || '';
    out.readonly = !!e.readOnly;
    if (out.type === 'file') {
      out.multiple = !!e.multiple;
      try {
        const files = e.files || [];
        out.files = files.length;
        for (let i = 0; i < Math.min(files.length, 5); i++) {
          out.filesNames.push(files[i].name);
        }
      } catch(_) {}
    }
  }
  return out;
}`

// escapeQuotes makes a string safe for embedding in `key="..."` text
// inside the snapshot. Stripped quotes prevent the model from
// confusing attribute boundaries; we don't need a perfect escape.
func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// DownloadMeta holds metadata for a downloaded file.
type DownloadMeta struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// Snapshot returns the full snapshot text for LLM consumption.
func (p *PageState) Snapshot() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Page: %s\nURL: %s\n\n", p.Title, p.URL))
	sb.WriteString("Page elements — to interact, use selector {\"type\": \"axNodeId\", \"query\": \"<id>\"}:\n\n")
	sb.WriteString(p.AXTree)
	return sb.String()
}

// ── Antibot CDP actions ─────────────────────────────────────────────────────
// These methods call Antibot.* CDP commands directly on the browser session.
// All return (success bool, errorMessage string).

// Selector is the Antibot element selector.
type Selector struct {
	Type  string `json:"type"`  // css, xpath, axNodeId, coord, role, bottom
	Query string `json:"query"`
}

// AntibotResult is the common response from Antibot commands.
type AntibotResult struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (s *Session) antibotCall(method string, params map[string]any) (*AntibotResult, error) {
	result, err := s.SendCDP(method, params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	var r AntibotResult
	json.Unmarshal(result, &r)
	return &r, nil
}

func selectorParam(sel Selector) map[string]any {
	return map[string]any{"type": sel.Type, "query": sel.Query}
}

// Click clicks on an element with human-like mouse movement.
func (s *Session) Click(sel Selector) (*AntibotResult, error) {
	return s.antibotCall("Antibot.clickOn", map[string]any{
		"selector": selectorParam(sel),
	})
}

// Fill clicks on an element, optionally clears it, then types text.
func (s *Session) Fill(sel Selector, text string, clear bool) (*AntibotResult, error) {
	return s.antibotCall("Antibot.fill", map[string]any{
		"selector": selectorParam(sel),
		"text":     text,
		"clear":    clear,
	})
}

// Type types text at the current cursor position with human-like timing.
func (s *Session) Type(text string) (*AntibotResult, error) {
	return s.antibotCall("Antibot.typeText", map[string]any{
		"text": text,
	})
}

// PressKey sends a single key press (Enter, Tab, Escape, Ctrl+a, etc.).
func (s *Session) PressKey(key string) (*AntibotResult, error) {
	return s.antibotCall("Antibot.pressKey", map[string]any{
		"key": key,
	})
}

// Scroll scrolls an element into view, scrolls to bottom, or scrolls by delta.
func (s *Session) Scroll(sel *Selector, deltaX, deltaY float64) (*AntibotResult, error) {
	params := map[string]any{}
	if sel != nil {
		params["selector"] = selectorParam(*sel)
	}
	if deltaX != 0 || deltaY != 0 {
		params["delta"] = map[string]any{"x": deltaX, "y": deltaY}
	}
	return s.antibotCall("Antibot.scroll", params)
}

// Hover hovers over an element with human-like mouse movement.
func (s *Session) Hover(sel Selector) (*AntibotResult, error) {
	return s.antibotCall("Antibot.hover", map[string]any{
		"selector": selectorParam(sel),
	})
}

// SelectOption selects an option in a <select> or custom dropdown.
func (s *Session) SelectOption(sel Selector, value, text string, index int) (*AntibotResult, error) {
	params := map[string]any{"selector": selectorParam(sel)}
	if value != "" {
		params["value"] = value
	}
	if text != "" {
		params["text"] = text
	}
	if index >= 0 {
		params["index"] = index
	}
	return s.antibotCall("Antibot.selectOption", params)
}

// WaitForElement waits for an element to appear in any frame.
func (s *Session) WaitForElement(sel Selector, timeoutMs int) (*AntibotResult, error) {
	params := map[string]any{"selector": selectorParam(sel)}
	if timeoutMs > 0 {
		params["timeout"] = timeoutMs
	}
	return s.antibotCall("Antibot.waitForElement", params)
}

// IsVisible checks whether an element is visible in the viewport.
func (s *Session) IsVisible(sel Selector) (visible bool, err error) {
	result, err := s.SendCDP("Antibot.isElementVisible", map[string]any{
		"selector": selectorParam(sel),
	})
	if err != nil {
		return false, err
	}
	var r struct {
		Visible bool   `json:"visible"`
		Exists  bool   `json:"exists"`
		Reason  string `json:"reason"`
	}
	json.Unmarshal(result, &r)
	return r.Visible, nil
}

// Navigate navigates the page to a new URL.
func (s *Session) Navigate(url string) error {
	_, err := s.SendCDP("Page.navigate", map[string]any{"url": url})
	return err
}

// Eval evaluates a JavaScript expression and returns the result as a string.
func (s *Session) Eval(expression string) (string, error) {
	result, err := s.SendCDP("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
	})
	if err != nil {
		return "", err
	}
	var evalResult struct {
		Result struct {
			Type  string `json:"type"`
			Value any    `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	json.Unmarshal(result, &evalResult)
	if evalResult.ExceptionDetails != nil {
		return "", fmt.Errorf("JS error: %s", evalResult.ExceptionDetails.Text)
	}
	b, _ := json.Marshal(evalResult.Result.Value)
	return string(b), nil
}

// ── Screencast ──────────────────────────────────────────────────────────────

// ScreencastFrame is a single frame from Page.startScreencast.
type ScreencastFrame struct {
	Data      string            `json:"data"`      // base64-encoded image
	Metadata  map[string]any    `json:"metadata"`
	SessionID int64             `json:"sessionId"`
}

// StartScreencast begins streaming page screenshots via CDP Page.startScreencast.
// The callback is called for each frame. Call StopScreencast to stop.
func (s *Session) StartScreencast(format string, quality int, onFrame func(ScreencastFrame)) {
	if format == "" {
		format = "jpeg"
	}
	if quality == 0 {
		quality = 50
	}

	// Register handler for screencastFrame events
	s.OnEvent("Page.screencastFrame", func(method string, params json.RawMessage) bool {
		var frame ScreencastFrame
		json.Unmarshal(params, &frame)
		onFrame(frame)
		// Acknowledge the frame (fire-and-forget — acks don't return results)
		s.SendCDPFireAndForget("Page.screencastFrameAck", map[string]any{
			"sessionId": frame.SessionID,
		})
		return true
	})

	s.SendCDP("Page.startScreencast", map[string]any{
		"format":        format,
		"quality":       quality,
		"maxWidth":      1920,
		"maxHeight":     1080,
		"everyNthFrame": 1,
	})
}

// StopScreencast stops the page screencast.
func (s *Session) StopScreencast() {
	s.SendCDP("Page.stopScreencast", nil)
}

// ── ScrapiumBrowser downloads ────────────────────────────────────────────────

// HasDownloads returns whether any files have been downloaded in this session.
func (s *Session) HasDownloads() (bool, error) {
	result, err := s.SendCDP("ScrapiumBrowser.hasDownloads", nil)
	if err != nil {
		return false, err
	}
	var r struct{ Result bool `json:"result"` }
	json.Unmarshal(result, &r)
	return r.Result, nil
}

// ListDownloads returns metadata for all downloaded files.
func (s *Session) ListDownloads() ([]DownloadMeta, error) {
	result, err := s.SendCDP("ScrapiumBrowser.getDownloadsMetadatas", nil)
	if err != nil {
		return nil, err
	}
	var r struct{ Metadata map[string]int64 `json:"metadata"` }
	json.Unmarshal(result, &r)
	var downloads []DownloadMeta
	for name, size := range r.Metadata {
		downloads = append(downloads, DownloadMeta{Filename: name, Size: size})
	}
	return downloads, nil
}

// GetDownload retrieves a single downloaded file as base64-encoded data.
func (s *Session) GetDownload(filename string) (string, error) {
	result, err := s.SendCDP("ScrapiumBrowser.getDownload", map[string]any{
		"filename": filename,
	})
	if err != nil {
		return "", err
	}
	var r struct {
		Data string `json:"data"`
	}
	json.Unmarshal(result, &r)
	return r.Data, nil
}

// GetAllDownloads retrieves all downloaded files as a map of filename to base64 data.
// If deleteAfter is true, files are removed from disk after reading.
func (s *Session) GetAllDownloads(deleteAfter bool) (map[string]string, error) {
	result, err := s.SendCDP("ScrapiumBrowser.getDownloads", map[string]any{
		"delete": deleteAfter,
	})
	if err != nil {
		return nil, err
	}
	var r struct{ Files map[string]string `json:"files"` }
	json.Unmarshal(result, &r)
	return r.Files, nil
}

// Screenshot captures the current page as PNG.
func (s *Session) Screenshot(fullPage bool, selector string) ([]byte, error) {
	params := map[string]any{
		"format":           "png",
		"optimizeForSpeed": true,
	}
	if fullPage {
		params["captureBeyondViewport"] = true
	}
	if selector != "" {
		boxJS := fmt.Sprintf(`JSON.stringify((function() {
			var el = document.querySelector(%q);
			if (!el) return null;
			var r = el.getBoundingClientRect();
			return {x: r.x, y: r.y, width: r.width, height: r.height};
		})())`, selector)
		boxResult, _ := s.SendCDP("Runtime.evaluate", map[string]any{
			"expression": boxJS, "returnByValue": true,
		})
		if boxResult != nil {
			var evalRes struct{ Result struct{ Value string `json:"value"` } `json:"result"` }
			json.Unmarshal(boxResult, &evalRes)
			var box struct{ X, Y, Width, Height float64 }
			if json.Unmarshal([]byte(evalRes.Result.Value), &box) == nil && box.Width > 0 {
				params["clip"] = map[string]any{
					"x": box.X, "y": box.Y, "width": box.Width, "height": box.Height, "scale": 1,
				}
			}
		}
	}
	result, err := s.SendCDP("Page.captureScreenshot", params)
	if err != nil {
		return nil, err
	}
	var screenshot struct {
		Data string `json:"data"`
	}
	json.Unmarshal(result, &screenshot)
	return []byte(screenshot.Data), nil
}
