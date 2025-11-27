SHELL := /bin/bash

COVER_PROFILE ?= coverage.out
TOOLS_MOD ?= go.tool.mod

GO ?= go
GOTEST := $(GO) test
GOTIDY := $(GO) mod tidy && $(GO) mod tidy -modfile=$(TOOLS_MOD)
GOMOD_DOWNLOAD := $(GO) mod download && $(GO) mod download -modfile=$(TOOLS_MOD)
GOMOD_VERIFY := $(GO) mod verify && $(GO) mod verify -modfile=$(TOOLS_MOD)
GOLANGCI := $(GO) tool -modfile=$(TOOLS_MOD) golangci-lint
GODOC := $(GO) tool -modfile=$(TOOLS_MOD) godoc -http=:6060

.PHONY: help
help: ## Show available make targets
	@echo "Available targets:"
	@grep -hE '^[%%a-zA-Z0-9_/.\-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS=":.*## "}; {printf "  %-20s %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## Sync dependency manifests
	@$(GOTIDY)

.PHONY: deps
deps: ## Download and verify module dependencies
	@$(GOMOD_DOWNLOAD)
	@$(GOMOD_VERIFY)

.PHONY: fmt
fmt: ## Format Go source files
	@$(GOLANGCI) run --config=.golangci.yml --fix --issues-exit-code=0 ./... >/dev/null

.PHONY: lint
lint: ## Run golangci-lint with repository configuration
	@$(GOLANGCI) run --config=.golangci.yml ./...

.PHONY: test
test: ## Execute unit tests
	@$(GOTEST) -cover ./...

.PHONY: cover
cover: ## Run tests with coverage reporting
	@$(GOTEST) -coverprofile=$(COVER_PROFILE) ./...
	@$(GO) tool cover -func=$(COVER_PROFILE)

.PHONY: doc
doc: ## Run godoc server on port 6060
	@$(GODOC)
