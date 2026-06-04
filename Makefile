BINARY := membot
BIN_DIR := bin

help: ## List of commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) ./cmd/membot

install: ## Install the binary
	go install ./cmd/membot

codegen: ## Generate the code
	go tool sqlc generate

lint: ## Run the linter
	go tool golangci-lint run

lint-fix: ## Fix the linter
	go tool golangci-lint run --fix

clean: ## Clean the binary
	rm -rf $(BIN_DIR)

.PHONY: help build install codegen lint lint-fix clean