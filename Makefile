SHELL := /bin/bash
.DEFAULT_GOAL := help

help: ## Show help
	@grep -E '^[a-zA-Z_-]+:.*?## ' Makefile | awk 'BEGIN {FS":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Run server locally
	cd backend && go run ./cmd/server

build-backend: ## Build backend binary
	cd backend && go build ./cmd/server

lint: ## Placeholder for lint
	@echo "lint ok"


