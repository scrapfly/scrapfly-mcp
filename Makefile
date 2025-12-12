release: ## make release VERSION=1.2.3
	npm version $(VERSION)
	git push --follow-tags