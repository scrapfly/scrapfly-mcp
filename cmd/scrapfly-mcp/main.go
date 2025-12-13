package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/scrapfly/go-scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/authenticableClient"
	"github.com/scrapfly/scrapfly-mcp/pkg/provider"
	scrapflyprovider "github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/server"
)

var (
	httpAddr = flag.String("http", "", "if set, use streamable HTTP at this address (include port number, eg 127.0.0.1:1423), instead of stdin/stdout")
	apiKey   = flag.String("apikey", "", "if set, use this API key, instead of the one in the environment variable")
)

func main() {
	flag.Parse()

	apikey := *apiKey
	if apikey == "" {
		apikey = os.Getenv("SCRAPFLY_API_KEY")
	}

	if apikey == "" && *httpAddr == "" {
		log.Fatal("Either apikey (as an argument or as an environment variable) or httpdAddr must must be set.")
	}

	clientGetter := func(p *scrapflyprovider.ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error) {
		return scrapflyprovider.MakeDefaultScrapflyClient(apikey), nil
	}

	if apikey == "" && *httpAddr != "" {
		clientGetter = func(p *scrapflyprovider.ScrapflyToolProvider, ctx context.Context) (*scrapfly.Client, error) {
			return authenticableClient.GetStreamableScrapflyClient(p, ctx)
		}
	}

	scrapflyToolProvider := scrapflyprovider.NewScrapflyToolProvider(scrapflyprovider.MakeDefaultScrapflyClient(apikey),
		clientGetter,
		nil)

	toolProvider := provider.NewToolProvider("scrapfly", scrapflyToolProvider)

	server := server.NewScrapflyMCPServer(toolProvider)
	if *httpAddr != "" { // httpAddr is actually string parsed WITH port number. port only imply 0.0.0.0 eg :1123
		server.WithHttpAddr(*httpAddr)
		if apikey == "" {
			server.WithStreamableServerFunction(authenticableClient.CorsAndAuthenticatedStreamableServerFunction)
		}
		server.ServeStreamable()
	} else {
		server.ServeStdio()
	}
}
