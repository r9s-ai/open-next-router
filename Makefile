.PHONY: run build build-all test version release-dry release-snapshot clean help

# Get version from git tags
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build flags
LDFLAGS := -s -w \
	-X github.com/r9s-ai/open-next-router/internal/version.Version=$(VERSION) \
	-X github.com/r9s-ai/open-next-router/internal/version.Commit=$(COMMIT) \
	-X github.com/r9s-ai/open-next-router/internal/version.BuildDate=$(BUILD_DATE)

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

run: ## Run the application locally
	go run -ldflags "$(LDFLAGS)" ./cmd/onr --config ./onr.yaml

build: ## Build the main binary
	go build -ldflags "$(LDFLAGS)" -o bin/onr ./cmd/onr
	go build -ldflags "$(LDFLAGS)" -o bin/onr-admin ./cmd/onr-admin

build-all: ## Build for all platforms using GoReleaser
	goreleaser build --snapshot --clean

test: ## Run tests
	go test -v ./...
	cd onr-core && go test -v ./...

version: ## Show version information
	@echo "Version:    $(VERSION)"
	@echo "Commit:     $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

encrypt: ## Encrypt text (usage: make encrypt TEXT="your-text")
	go run ./cmd/onr-admin crypto encrypt --text "$(TEXT)"

release-dry: ## Dry run of GoReleaser (test release without publishing)
	goreleaser release --snapshot --clean --skip=publish

release-snapshot: ## Build snapshot release (for testing)
	goreleaser build --snapshot --clean

clean: ## Clean build artifacts
	rm -rf bin/ dist/
	go clean

fmt: ## Format code
	go fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/goreleaser/goreleaser@latest
	@echo "Done! GoReleaser installed."
