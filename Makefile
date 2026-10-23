######################
# Help
######################
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2}'

######################
# Testing
######################
test: ## Run tests
	go test -v ./...

vet: ## Run go vet
	go vet ./...

######################
# Security
######################
govulncheck: ## Run Go vulnerability check
	@echo "🔎 Running govulncheck..."
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

######################
# Dependencies
######################
mod-tidy: ## Tidy go modules
	go mod tidy
