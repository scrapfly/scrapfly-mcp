build: ## Build the scrapfly-mcp binary
	go build -o bin/scrapfly-mcp ./cmd/scrapfly-mcp

release: ## make release VERSION=1.2.3
	npm version $(VERSION)
	git push --follow-tags
