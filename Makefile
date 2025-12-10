bump-version:
	@echo "Bumping version to ${VERSION}"
	npm version --no-git-tag-version "${VERSION}"
	git add :/
	git commit -m "Bump version to ${VERSION}"
	git tag "v${VERSION}"
	git push
	git push origin "v${VERSION}"

tag-current-version:
	$(eval CURRENT_VERSION := $(shell node -p "require('./package.json').version"))
	@echo "Tagging current version: ${CURRENT_VERSION}"
	git tag "v${CURRENT_VERSION}"
	git push origin "v${CURRENT_VERSION}"