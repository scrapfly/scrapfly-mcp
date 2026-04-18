build: ## Build the scrapfly-mcp binary
	go build -o bin/scrapfly-mcp ./cmd/scrapfly-mcp

generate-antibot-schemas: ## Regenerate antibot tool schemas from browser_protocol.json
	python3 pkg/provider/scrapfly/gen_antibot_schemas.py browser_protocol.json > pkg/provider/scrapfly/antibot_schemas_gen.go
	@echo "Generated pkg/provider/scrapfly/antibot_schemas_gen.go"

release: ## make release VERSION=1.2.3
	npm version $(VERSION)
	git push --follow-tags
