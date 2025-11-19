package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func DefaultStreamableTransportLoggingMiddleware(maxLength int) func() func(next mcp.MethodHandler) mcp.MethodHandler {
	// 0 means no remarshalling of content (e.g for production)
	if maxLength < 0 {
		maxLength = 50
	}
	logger := log.New(os.Stdout, "[StreamableTransport] ", log.Lmicroseconds|log.Lmsgprefix|log.LstdFlags)
	return func() func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(
				ctx context.Context,
				method string,
				req mcp.Request,
			) (mcp.Result, error) {

				logger.Printf("MCP method started: %s, session_id: %s, has_params: %t", method, req.GetSession().ID(), req.GetParams() != nil)
				// Log more for tool calls.
				if ctr, ok := req.(*mcp.CallToolRequest); ok {
					var JsonArgsString string
					// Unstructured logging needs remarshalling of arguments
					if ctr.Params.Arguments != nil {
						jsonArgs, err := json.Marshal(ctr.Params.Arguments)
						if err != nil {
							JsonArgsString = "Error marshalling args: " + err.Error()
						} else {
							JsonArgsString = string(jsonArgs)
						}
					} else {
						JsonArgsString = "<No args>"
					}
					logger.Printf("Calling tool: %s, args: %s", ctr.Params.Name, JsonArgsString)
				}

				start := time.Now()
				result, err := next(ctx, method, req)

				duration := time.Since(start)
				if err != nil {
					logger.Printf("MCP method failed: %s, session_id: %s, duration_ms: %d, err: %v", method, req.GetSession().ID(), duration.Milliseconds(), err)
				} else {
					logger.Printf("MCP method completed: %s, session_id: %s, duration_ms: %d, has_result: %t", method, req.GetSession().ID(), duration.Milliseconds(), result != nil)
					// Log more for tool results.
					if ctr, ok := result.(*mcp.CallToolResult); ok {
						var content []mcp.Content
						var jsonContentString string
						// dont use StructuredContent without Structured Logging
						if ctr.Content != nil {
							if maxLength > 0 {
								content = ctr.Content
								jsonContent, err := json.Marshal(content)
								if err != nil {
									jsonContentString = "Error marshalling content: " + err.Error()
								} else {
									jsonContentString = truncateBytes(string(jsonContent), maxLength)
									jsonContentString = fmt.Sprintf("%s...<truncated>", jsonContentString)
								}
							} else {
								jsonContentString = "<present but not shown>"
							}
						} else {
							jsonContentString = "<No content>"
						}
						logger.Printf("Tool result: %s, isError: %t, content: %s", method, ctr.IsError, jsonContentString)
					}
				}
				return result, err
			}
		}
	}
}

// Example atlevel logging middleware
func SlogLoggingMiddleware(level slog.Level) func(next mcp.MethodHandler) mcp.MethodHandler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		/*ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Simplify timestamp format for consistent output.
			if a.Key == slog.TimeKey {
				return slog.String("time", "2025-01-01T00:00:00Z")
			}
			return a
		},*/
	}))

	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {

			logger.Info("MCP method started",
				"method", method,
				"session_id", req.GetSession().ID(),
				"has_params", req.GetParams() != nil,
			)
			// Log more for tool calls.
			if ctr, ok := req.(*mcp.CallToolRequest); ok {
				logger.Info("Calling tool",
					"name", ctr.Params.Name,
					"args", ctr.Params.Arguments)
			}

			start := time.Now()
			result, err := next(ctx, method, req)

			duration := time.Since(start)
			if err != nil {
				logger.Error("MCP method failed",
					"method", method,
					"session_id", req.GetSession().ID(),
					"duration_ms", duration.Milliseconds(),
					"err", err,
				)
			} else {
				logger.Info("MCP method completed",
					"method", method,
					"session_id", req.GetSession().ID(),
					"duration_ms", duration.Milliseconds(),
					"has_result", result != nil,
				)
				// Log more for tool results.
				if ctr, ok := result.(*mcp.CallToolResult); ok {
					var content []mcp.Content
					if ctr.StructuredContent == nil {
						if ctr.Content != nil {
							content = ctr.Content
						}
					}
					logger.Info("tool result",
						"isError", ctr.IsError,
						"structuredContent", ctr.StructuredContent,
						"content", content,
					)
				}
			}
			return result, err
		}
	}
}
