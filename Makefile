.PHONY: help
help: ## Show this text.
	# https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: test
test: ## Run test
	go test -v -coverprofile=profile.cov ./...

.PHONY: generate
generate: ## Generate
	go generate ./...

.PHONY: check-diff
check-diff: ## Check whether there are changes
	@./scripts/check-diff.sh
