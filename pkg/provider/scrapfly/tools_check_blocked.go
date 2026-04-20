package scrapflyprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// classifyAPIHostEnv lets operators point the MCP tool at a custom
// classification endpoint (staging, enterprise, on-prem). When unset
// the tool hits the public Scrapfly API, matching the SDK default.
const classifyAPIHostEnv = "SCRAPFLY_API_HOST"
const classifyDefaultHost = "https://api.scrapfly.io"

// defaultStatusCodeForClassify is used when the caller doesn't supply
// one. 200 is what most LLM agents produce when probing a page they
// just scraped, and the classifier still runs body+header analysis on
// it, so this is a safe default rather than a 400.
const defaultStatusCodeForClassify = 200

type CheckIfBlockedInput struct {
	URL             string            `json:"url" jsonschema:"URL the scraped response came from. Included in the classification request so the server can apply domain-aware detection."`
	Content         string            `json:"content" jsonschema:"Page content (HTML/text) from a scrape result. Use raw or clean_html format for best detection accuracy."`
	StatusCode      int               `json:"status_code,omitempty" jsonschema:"HTTP status code from the scrape result (e.g. 403, 429, 503). Defaults to 200. Improves detection accuracy."`
	ResponseHeaders map[string]string `json:"response_headers,omitempty" jsonschema:"Response headers from the scrape result. Enables header-based antibot detection."`
}

type CheckIfBlockedOutput struct {
	IsBlocked      bool   `json:"is_blocked"`
	Antibot        string `json:"antibot,omitempty"`
	Confidence     string `json:"confidence"`
	Details        string `json:"details"`
	Recommendation string `json:"recommendation"`
	Cost           int    `json:"cost"`
}

type classifyAPIRequest struct {
	URL        string            `json:"url"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
}

// classifyAPIResult mirrors the live /classify response shape:
// flat {blocked, antibot: string|null, cost: int}. Kept intentionally
// minimal so that additions on the server side surface transparently
// if we decide to propagate them later.
type classifyAPIResult struct {
	Blocked bool   `json:"blocked"`
	Antibot string `json:"antibot"`
	Cost    int    `json:"cost"`
}

// classifyAPIError is the canonical Scrapfly error envelope
// (code, message, http_code, retryable, reason, error_id, links)
// that the API returns on every 4xx/5xx from /classify. Decoding it
// lets the tool surface Retryable on 503s and a useful error_id
// for support tickets instead of a raw byte dump.
type classifyAPIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	HTTPCode  int    `json:"http_code"`
	Retryable bool   `json:"retryable"`
	Reason    string `json:"reason"`
	ErrorID   string `json:"error_id"`
}

// CheckIfBlocked forwards the response metadata to the Scrapfly /classify
// endpoint. The server-side classifier is authoritative and evolves
// continuously; this tool stays thin on purpose so new anti-bot
// detections become available to MCP users without redeploying.
func (p *ScrapflyToolProvider) CheckIfBlocked(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input CheckIfBlockedInput,
) (*mcp.CallToolResult, *CheckIfBlockedOutput, error) {
	p.logger.Println("Executing tool: check_if_blocked (server-side classifier)")

	if input.URL == "" {
		return nil, nil, fmt.Errorf("url is required for classification")
	}
	if input.StatusCode == 0 {
		input.StatusCode = defaultStatusCodeForClassify
	}

	client, err := p.ClientGetter(p, ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("scrapfly client: %w", err)
	}

	host := os.Getenv(classifyAPIHostEnv)
	if host == "" {
		host = classifyDefaultHost
	}

	payload, err := json.Marshal(classifyAPIRequest{
		URL:        input.URL,
		StatusCode: input.StatusCode,
		Headers:    input.ResponseHeaders,
		Body:       input.Content,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal classify: %w", err)
	}

	endpoint, err := url.Parse(host + "/classify")
	if err != nil {
		return nil, nil, fmt.Errorf("parse classify host: %w", err)
	}
	q := endpoint.Query()
	q.Set("key", client.APIKey())
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("classify request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("classify read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		var e classifyAPIError
		if json.Unmarshal(body, &e) == nil && e.Code != "" {
			retryHint := ""
			if e.Retryable {
				retryHint = " (retryable)"
			}
			return nil, nil, fmt.Errorf("classify %s [HTTP %d%s]: %s (error_id=%s)",
				e.Code, e.HTTPCode, retryHint, e.Message, e.ErrorID)
		}
		return nil, nil, fmt.Errorf("classify returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result classifyAPIResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, fmt.Errorf("classify decode: %w", err)
	}

	output := &CheckIfBlockedOutput{
		IsBlocked: result.Blocked,
		Antibot:   result.Antibot,
		Cost:      result.Cost,
	}

	switch {
	case result.Blocked && result.Antibot != "":
		output.Confidence = "high"
		output.Details = fmt.Sprintf("%s shield matched the response signature.", result.Antibot)
		output.Recommendation = "Enable asp=true with render_js=true; if still blocked, switch to residential proxy pool."
	case result.Blocked:
		output.Confidence = "medium"
		output.Details = "Response flagged as blocked without a named shield match."
		output.Recommendation = "Enable asp=true; try render_js=true and/or a different proxy pool."
	default:
		output.Confidence = "high"
		output.Details = "No antibot blocking detected — the page content looks legitimate."
		output.Recommendation = "No action needed."
	}

	return nil, output, nil
}
