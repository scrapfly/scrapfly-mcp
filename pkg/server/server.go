package server

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/scrapfly/scrapfly-mcp/pkg/provider"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ContextKey is a type for context keys used in this package
type ContextKey string

// APIKeyContextKey is the context key for storing the API key from query parameters
const APIKeyContextKey ContextKey = "apiKey"

const ServerVersion = "1.1.0"

func init() {
	log.SetFlags(log.Lmicroseconds | log.Lmsgprefix | log.LstdFlags)
}

type StreamableServerFunction func(handler http.Handler, httpAddr *string)
type StdioServerFunction func(server *mcp.Server, t *mcp.LoggingTransport)

func DefaultStreamableServerFunction(handler http.Handler, httpAddr *string) {
	log.Fatal(http.ListenAndServe(*httpAddr, handler))
}

func DefaultStdioServerFunction(server *mcp.Server, t *mcp.LoggingTransport) {
	if err := server.Run(context.Background(), t); err != nil {
		log.Printf("Server failed: %v", err)
	}
}

type ScrapflyMCPServer struct {
	server                   *mcp.Server
	toolProviders            []provider.ToolProvider
	streamableServerFunction StreamableServerFunction
	stdioServerFunction      StdioServerFunction
	streamableHTTPOptions    *mcp.StreamableHTTPOptions
	httpAddr                 string
	started                  bool
	loggingMiddleware        func() func(next mcp.MethodHandler) mcp.MethodHandler
}

func NewScrapflyMCPServer(toolProviders ...provider.ToolProvider) *ScrapflyMCPServer {
	return &ScrapflyMCPServer{
		server:                   newServer(toolProviders...),
		toolProviders:            toolProviders,
		streamableServerFunction: DefaultStreamableServerFunction,
		stdioServerFunction:      DefaultStdioServerFunction,
		httpAddr:                 "0.0.0.0:1123",
		loggingMiddleware:        DefaultStreamableTransportLoggingMiddleware(50),
	}
}

func (s *ScrapflyMCPServer) Server() *mcp.Server {
	return s.server
}

func (s *ScrapflyMCPServer) WithStreamableHTTPOptions(streamableHTTPOptions *mcp.StreamableHTTPOptions) *ScrapflyMCPServer {
	if !s.canChangeConfig("change streamable http options") {
		return s
	}
	s.streamableHTTPOptions = streamableHTTPOptions
	return s
}

func (s *ScrapflyMCPServer) WithStreamableServerFunction(streamableServerFunction StreamableServerFunction) *ScrapflyMCPServer {
	if !s.canChangeConfig("change streamable server function") {
		return s
	}
	s.streamableServerFunction = streamableServerFunction
	return s
}

func (s *ScrapflyMCPServer) WithStdioServerFunction(stdioServerFunction StdioServerFunction) *ScrapflyMCPServer {
	if !s.canChangeConfig("change stdio server function") {
		return s
	}
	s.stdioServerFunction = stdioServerFunction
	return s
}

func (s *ScrapflyMCPServer) WithHttpAddr(httpAddr string) *ScrapflyMCPServer {
	if !s.canChangeConfig("change http address") {
		return s
	}
	s.httpAddr = httpAddr
	return s
}

func (s *ScrapflyMCPServer) WithLoggingMiddleware(loggingMiddleware func() func(next mcp.MethodHandler) mcp.MethodHandler) *ScrapflyMCPServer {
	if !s.canChangeConfig("change logging middleware") {
		return s
	}
	s.loggingMiddleware = loggingMiddleware
	return s
}

func (s *ScrapflyMCPServer) canChangeConfig(action string) bool {
	if s.started {
		log.Printf("[SCRAPFLY-MCP] Server already started, cannot %s", action)
		return false
	}
	return true
}

func (s *ScrapflyMCPServer) ServeStdio() {
	if !s.canChangeConfig("serve stdio") {
		return
	}
	s.started = true
	log.Printf("[SCRAPFLY-MCP] Starting stdio server on %s\n", s.httpAddr)
	s.stdioServerFunction(s.server, &mcp.LoggingTransport{Transport: &mcp.StdioTransport{}, Writer: os.Stderr})
}

func (s *ScrapflyMCPServer) ServeStreamable() {
	if !s.canChangeConfig("serve streamable") {
		return
	}
	s.started = true
	log.Printf("[SCRAPFLY-MCP] Starting streamable server on %s\n", s.httpAddr)
	s.server.AddReceivingMiddleware(s.loggingMiddleware())
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.server
	}, s.streamableHTTPOptions)

	// Wrap the handler to extract apiKey from query parameters and inject into context
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.URL.Query().Get("apiKey")
		if apiKey != "" {
			ctx := context.WithValue(r.Context(), APIKeyContextKey, apiKey)
			r = r.WithContext(ctx)
		}
		mcpHandler.ServeHTTP(w, r)
	})

	s.streamableServerFunction(wrappedHandler, &s.httpAddr)
}

func newServer(toolProviders ...provider.ToolProvider) *mcp.Server {
	log.Printf("[SCRAPFLY-MCP] Bootstraping MCP server...\n")
	log.Printf("[SCRAPFLY-MCP] Server version: %s\n", ServerVersion)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "scrapfly-tools",
		Title:   "Scrapfly MCP Server",
		Version: ServerVersion,
	},
		&mcp.ServerOptions{
			Instructions: "always ensure assistant has read the scraping_instruction_enhanced tool before using any scraping",
		})

	if len(toolProviders) > 0 {
		for _, toolProvider := range toolProviders {
			log.Printf("[%s] Registering provider...\n", toolProvider.Name())
			providerToolNames, providerPromptNames, providerResourceNames := toolProvider.RegisterAll(server)
			log.Printf("[%s] Registered tools: %v\n", toolProvider.Name(), providerToolNames)
			log.Printf("[%s] Registered prompts: %v\n", toolProvider.Name(), providerPromptNames)
			log.Printf("[%s] Registered resources: %v\n", toolProvider.Name(), providerResourceNames)
		}
	}

	return server
}
