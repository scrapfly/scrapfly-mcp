package scrapflyprovider

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	scrapfly "github.com/scrapfly/go-scrapfly"
)

func ToolErrf(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
		IsError: true,
	}
}

type toolErrorPayload struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	Hint          string `json:"hint,omitempty"`
	RetryAfterMs  int    `json:"retry_after_ms,omitempty"`
	ResultContent string `json:"result_content,omitempty"`
}

func ToolErr(code, message, hint string, retryAfterMs int, resultContent string) *mcp.CallToolResult {
	payload := toolErrorPayload{
		Code:          code,
		Message:       message,
		Hint:          hint,
		RetryAfterMs:  retryAfterMs,
		ResultContent: resultContent,
	}
	b, _ := json.Marshal(payload)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		IsError: true,
	}
}

func ToolErrFromError(tool string, err error) *mcp.CallToolResult {
	var apiErr *scrapfly.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.Code
		if code == "" {
			switch apiErr.HTTPStatusCode {
			case 401:
				code = "UNAUTHORIZED"
			case 429:
				code = "RATE_LIMITED"
			default:
				code = fmt.Sprintf("UPSTREAM_%d", apiErr.HTTPStatusCode)
			}
		}
		hint := apiErr.Hint
		msg := fmt.Sprintf("%s: %s", tool, apiErr.Message)
		resultContent := ""
		if apiErr.APIResponse != nil {
			resultContent = apiErr.APIResponse.Result.Content
		}
		return ToolErr(code, msg, hint, apiErr.RetryAfterMs, resultContent)
	}
	return ToolErr("GENERIC_ERROR", fmt.Sprintf("%s: %v", tool, err), "Check inputs and try again.", 0, "")
}
