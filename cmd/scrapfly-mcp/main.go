package main

import (
	"flag"
	"log"
	"os"

	"github.com/scrapfly/scrapfly-mcp/pkg/provider"
	scrapflyprovider "github.com/scrapfly/scrapfly-mcp/pkg/provider/scrapfly"
	"github.com/scrapfly/scrapfly-mcp/pkg/server"
)

var (
	httpAddr = flag.String("http", "", "if set, use streamable HTTP at this address, instead of stdin/stdout")
	apiKey   = flag.String("apikey", "", "if set, use this API key, instead of the one in the environment variable")
)

func main() {
	flag.Parse()

	apikey := *apiKey
	if apikey == "" {
		apikey = os.Getenv("SCRAPFLY_API_KEY")
	}
	if apikey == "" {
		log.Fatal("apikey must be set. Either as an argument or as an environment variable.")
	}

	scrapflyToolProvider := scrapflyprovider.NewScrapflyToolProvider(scrapflyprovider.MakeDefaultScrapflyClient(apikey),
		scrapflyprovider.GetDefaultScrapflyClient,
		nil)

	toolProvider := provider.NewToolProvider("scrapfly", scrapflyToolProvider)

	server := server.NewScrapflyMCPServer(toolProvider)
	if *httpAddr != "" {
		server.WithHttpAddr(*httpAddr)
		server.ServeStreamable()
	} else {
		server.ServeStdio()
	}
}
