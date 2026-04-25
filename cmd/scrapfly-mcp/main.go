package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/authenticableClient"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider"
	scrapflyprovider "github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/server"
)

var (
	httpAddr = flag.String("http", "", "if set, use streamable HTTP at this address (include port number, eg 127.0.0.1:1423), instead of stdin/stdout")
	apiKey   = flag.String("apikey", "", "if set, use this API key, instead of the one in the environment variable")
	apiHost  = flag.String("host", "", "if set, override the Scrapfly API host (e.g. https://api.scrapfly.local for local dev cluster). Falls back to SCRAPFLY_API_HOST env var, then to the SDK default https://api.scrapfly.io.")
	browserHost = flag.String("browser-host", "", "if set, override the Scrapfly Cloud Browser host (e.g. https://browser.scrapfly.local). Falls back to SCRAPFLY_BROWSER_HOST env var, then derives from -host by replacing the leading 'api.' with 'browser.', then to the SDK default https://browser.scrapfly.io.")
	verifySSLFlag = flag.Bool("verify-ssl", true, "verify TLS certificates on outbound calls. Set false ONLY when targeting a self-signed dev host (api.scrapfly.local). Falls back to SCRAPFLY_VERIFY_SSL env var (`0`/`false` to disable).")
)

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
	// We only need to disable for self-signed dev clusters
	// (api.scrapfly.local).
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
