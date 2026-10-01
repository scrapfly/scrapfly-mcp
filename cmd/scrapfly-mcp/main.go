package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/authenticableClient"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider"
	scrapflyprovider "github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/server"
)

var (
	httpAddr      = flag.String("http", "", "if set, use streamable HTTP at this address (include port number, eg 127.0.0.1:1423), instead of stdin/stdout")
	apiKey        = flag.String("apikey", "", "if set, use this API key, instead of the one in the environment variable")
	apiHost       = flag.String("host", "", "if set, override the Scrapfly API host. Falls back to SCRAPFLY_API_HOST env var, then to the SDK default https://api.scrapfly.io.")
	browserHost   = flag.String("browser-host", "", "if set, override the Scrapfly Cloud Browser host. Falls back to SCRAPFLY_BROWSER_HOST env var, then derives from -host by replacing the leading 'api.' with 'browser.', then to the SDK default https://browser.scrapfly.io.")
	verifySSLFlag = flag.Bool("verify-ssl", true, "verify TLS certificates on outbound calls. Set false ONLY when targeting a host serving a self-signed certificate. Falls back to SCRAPFLY_VERIFY_SSL env var (`0`/`false` to disable).")
	allowRemote   = flag.Bool("allow-remote", false, "server-key HTTP mode only: allow binding a non-loopback interface. That mode has no per-caller authentication — every request uses the baked-in API key — so bind loopback unless you front it with your own auth. Falls back to SCRAPFLY_ALLOW_REMOTE env var.")
)

// isLoopbackHost reports whether h is a loopback address or "localhost".
func isLoopbackHost(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// normalizeServerKeyBind keeps server-key HTTP mode off the network. That mode
// authenticates no caller: every request uses the baked-in API key, so a
// routable bind would hand the key to anyone who can reach the port. A missing
// host defaults to loopback; a non-loopback host is refused unless allowRemote.
func normalizeServerKeyBind(addr string, allowRemote bool) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// No colon (e.g. "8080") — treat the whole value as a port on loopback.
		return "127.0.0.1:" + strings.TrimPrefix(addr, ":"), nil
	}
	if host == "" {
		return "127.0.0.1:" + port, nil
	}
	if !allowRemote && !isLoopbackHost(host) {
		return "", fmt.Errorf("refusing to bind server-key HTTP mode to %s: this mode has no authentication and would expose the API key to anyone who can reach the port; bind 127.0.0.1, or pass -allow-remote to override", addr)
	}
	return addr, nil
}

// deriveBrowserHostFromAPI returns the Cloud Browser host implied by an
// API host override, on the convention that the two share the same root
// domain with different sub-domains (api.example.com → browser.example.com).
// Returns "" when the substitution cannot be made unambiguously, so the
// caller can fall back to the SDK default.
func deriveBrowserHostFromAPI(apiHost string) string {
	scheme, rest, ok := strings.Cut(apiHost, "://")
	if !ok {
		return ""
	}
	if !strings.HasPrefix(rest, "api.") {
		return ""
	}
	return scheme + "://browser." + strings.TrimPrefix(rest, "api.")
}

func main() {
	flag.Parse()

	apikey := *apiKey
	if apikey == "" {
		apikey = os.Getenv("SCRAPFLY_API_KEY")
	}

	// Host override: -host flag wins over SCRAPFLY_API_HOST env wins
	// over the SDK's hard-coded default (https://api.scrapfly.io).
	// This mirrors the apikey precedence above so the precedence story
	// is the same across every config knob.
	apiHostStr := *apiHost
	if apiHostStr == "" {
		apiHostStr = os.Getenv("SCRAPFLY_API_HOST")
	}

	// Browser host: explicit -browser-host > SCRAPFLY_BROWSER_HOST > derived
	// from apiHostStr (api.X → browser.X) > SDK default. Independent knob
	// because in prod the two hosts scale separately, but in the local
	// self-hosted setup they share the same loopback root.
	browserHostStr := *browserHost
	if browserHostStr == "" {
		browserHostStr = os.Getenv("SCRAPFLY_BROWSER_HOST")
	}
	if browserHostStr == "" && apiHostStr != "" {
		browserHostStr = deriveBrowserHostFromAPI(apiHostStr)
	}

	// TLS verification: explicit -verify-ssl=false wins; otherwise
	// SCRAPFLY_VERIFY_SSL=0/false disables verification. Default true.
	// Only needed for a host serving a self-signed certificate.
	verify := *verifySSLFlag
	if v := os.Getenv("SCRAPFLY_VERIFY_SSL"); v != "" {
		verify = !(v == "0" || v == "false" || v == "False")
	}

	// Determine HTTP address: -http flag takes precedence, then PORT env var
	addr := *httpAddr
	if addr == "" {
		if port := os.Getenv("PORT"); port != "" {
			addr = ":" + port
		}
	}

	if apikey == "" && addr == "" {
		log.Fatal("Either apikey (as an argument or as an environment variable) or httpdAddr must must be set.")
	}

	// Server-key HTTP mode (a key is baked in AND we serve HTTP) runs no
	// per-caller auth, so keep it on loopback unless explicitly allowed out.
	serverKeyHTTP := apikey != "" && addr != ""
	if serverKeyHTTP {
		allow := *allowRemote
		// Fail closed: only an explicit truthy value opens the unauthenticated
		// port to the network; anything else (typo, "no", "off") keeps loopback.
		if v := os.Getenv("SCRAPFLY_ALLOW_REMOTE"); v != "" {
			allow = v == "1" || v == "true" || v == "True"
		}
		normalized, err := normalizeServerKeyBind(addr, allow)
		if err != nil {
			log.Fatal(err)
		}
		if normalized != addr {
			log.Printf("[SCRAPFLY-MCP] server-key HTTP mode has no authentication; binding %s (loopback only — pass -allow-remote to override)", normalized)
			addr = normalized
		}
	}

	// makeClient picks the right SDK constructor based on whether a
	// custom host was supplied. NewWithHost is the SDK's documented
	// path for "configure host + verifySSL"; we don't roll our own
	// http.Client / RoundTripper because the SDK already does it
	// correctly inside NewWithHost.
	makeClient := func() *scrapfly.Client {
		var c *scrapfly.Client
		if apiHostStr != "" {
			built, err := scrapfly.NewWithHost(apikey, apiHostStr, verify)
			if err != nil {
				log.Printf("Failed to create scrapfly client with host=%s: %v", apiHostStr, err)
				return nil
			}
			log.Printf("[SCRAPFLY-MCP] Using API host override: %s (verifySSL=%v)", apiHostStr, verify)
			c = built
		} else {
			c = scrapflyprovider.MakeDefaultScrapflyClient(apikey)
		}
		// SDK keeps the Cloud Browser host on a separate knob from the API
		// host because in prod they scale independently. Apply the override
		// here so cloud_browser_open and friends point at the right cluster.
		if browserHostStr != "" && c != nil {
			c.SetCloudBrowserHost(browserHostStr)
			log.Printf("[SCRAPFLY-MCP] Using Cloud Browser host override: %s", browserHostStr)
		}
		return c
	}

	clientGetter := func(p *scrapflyprovider.ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error) {
		return makeClient(), nil
	}

	if apikey == "" && addr != "" {
		clientGetter = func(p *scrapflyprovider.ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error) {
			return authenticableClient.GetStreamableScrapflyClient(p, ctx)
		}
	}

	scrapflyToolProvider := scrapflyprovider.NewScrapflyToolProvider(makeClient(),
		clientGetter,
		nil)
	// Keep TLS verification on for the Cloud Browser CDP dial unless the
	// operator asked to disable it for a self-signed host.
	scrapflyToolProvider.InsecureSkipTLSVerify = !verify
	// info_api_key returns the operator credential; expose it only when the
	// caller already holds that key — stdio (local client) or per-request HTTP
	// auth (caller presents their own) — never in server-key HTTP mode.
	scrapflyToolProvider.ExposeAPIKeyTool = !serverKeyHTTP

	toolProvider := provider.NewToolProvider("scrapfly", scrapflyToolProvider)

	server := server.NewScrapflyMCPServer(toolProvider)

	// Determine HTTP address: -http flag takes precedence, then PORT env var

	if addr != "" { // httpAddr is actually string parsed WITH port number. port only imply 0.0.0.0 eg :1123
		server.WithHttpAddr(addr)
		if apikey == "" {
			server.WithStreamableServerFunction(authenticableClient.CorsAndAuthenticatedStreamableServerFunction)
		}
		server.ServeStreamable()
	} else {
		server.ServeStdio()
	}
}
