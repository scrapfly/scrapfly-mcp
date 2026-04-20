# Scrapfly MCP — release/dev Makefile.
# Target names mirror sdk/python/Makefile and sdk/rust/Makefile for
# muscle-memory parity.

VERSION ?=
NEXT_VERSION ?=

.PHONY: init install dev build bump generate-antibot-schemas generate-docs release test help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}'

init: ## Install toolchain prerequisites
	go version
	npm --version

install: ## Fetch Go dependencies
	go mod download
	go mod tidy

build: ## Build the scrapfly-mcp binary
	go build -o bin/scrapfly-mcp ./cmd/scrapfly-mcp

dev: build ## Build for local development

test: ## Run Go tests
	go test ./...

generate-antibot-schemas: ## Regenerate antibot tool schemas from browser_protocol.json
	python3 pkg/provider/scrapfly/gen_antibot_schemas.py browser_protocol.json > pkg/provider/scrapfly/antibot_schemas_gen.go
	@echo "Generated pkg/provider/scrapfly/antibot_schemas_gen.go"

generate-docs: ## Generate Go documentation
	go doc -all ./... > docs.txt || true

bump: ## make bump VERSION=x.y.z — update package.json version, commit, push
	@if [ -z "$(VERSION)" ]; then echo "Usage: make bump VERSION=x.y.z"; exit 2; fi
	@# Idempotent: skip `npm version` if package.json already at target — it
	@# errors "Version not changed" otherwise and aborts `make release`.
	@current=$$(node -p "require('./package.json').version"); \
	if [ "$$current" = "$(VERSION)" ]; then \
		echo "bump: package.json already at $(VERSION), skipping npm version"; \
	else \
		npm version $(VERSION) --no-git-tag-version; \
		git add package.json; \
		git commit -m "bump version to $(VERSION)"; \
		git push; \
	fi

release: ## make release VERSION=x.y.z NEXT_VERSION=x.y.z+1 — tag + publish to npm
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release VERSION=x.y.z NEXT_VERSION=x.y.z+1"; exit 2; fi
	@if [ -z "$(NEXT_VERSION)" ]; then echo "Usage: make release VERSION=x.y.z NEXT_VERSION=x.y.z+1"; exit 2; fi
	git branch | grep \* | cut -d ' ' -f2 | grep main || exit 1
	git pull origin main
	$(MAKE) test
	$(MAKE) build
	@# Idempotent: skip `npm version` if package.json already at target (the
	@# previous `make bump` may have already advanced it). Plain `npm version`
	@# errors "Version not changed" and aborts the whole release otherwise.
	@current=$$(node -p "require('./package.json').version"); \
	if [ "$$current" = "$(VERSION)" ]; then \
		echo "release: package.json already at $(VERSION), skipping npm version"; \
	else \
		npm version $(VERSION) --no-git-tag-version; \
		git add package.json; \
		git commit -m "Release $(VERSION)"; \
		git push origin main; \
	fi
	git tag -a v$(VERSION) -m "Version $(VERSION)"
	@# Push ONLY the new tag. `--tags` sweeps up every stale local tag (see
	@# cli Makefile note) and can push non-FF on moving tags like `latest`.
	git push origin v$(VERSION)
	$(MAKE) bump VERSION=$(NEXT_VERSION)
