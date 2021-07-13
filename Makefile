.PHONY: help
help: ## Show this text.
	# https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: help
test: ## Run test
	go test -v -coverprofile=profile.cov ./...

.PHONY: help
generate: ## Generate
	go generate ./...

.PHONY: help
check-diff: ## Check whether there are changes
	@./scripts/check-diff.sh
