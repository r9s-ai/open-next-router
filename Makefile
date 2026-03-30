.PHONY: run build build-all test version release-dry release-snapshot clean help bench-dsl bench-dslconfig bench-dsl-pprof prek prek-install

# Get version from root release tags only (ignore submodule tags like onr-core/v*)
VERSION ?= $(shell git describe --tags --match 'v*' --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
ENV_FILE ?= ./.env
GO_BUILD_CACHE ?= /tmp/go-build-cache
BENCH_COUNT ?= 1
BENCHTIME ?=
BENCHMEM ?= -benchmem
DSL_BENCH_PKGS := ./onr-core/pkg/jsonutil ./onr-core/pkg/dslmeta ./onr-core/pkg/usageestimate ./onr-core/pkg/dslconfig
DSL_BENCH_PATTERN ?= Benchmark(GetFloatByPathWithMatch|GetFirstIntByPaths|MetaRequestRoot|EstimateChatCompletions|ExtractUsage_|ExtractFinishReason_|ProviderUsageSelect_|ProviderFinishReasonSelect_|StreamMetricsAggregator_)
DSL_PPROF_BENCH ?= BenchmarkExtractUsage_CustomFacts_Exported
DSL_PPROF_OUT ?= /tmp/dslconfig.cpu.out

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
	@if [ ! -f ./onr.yaml ]; then \
		echo "onr.yaml not found, copying from config/onr.example.yaml"; \
		cp ./config/onr.example.yaml ./onr.yaml; \
	fi
	@if [ ! -f ./keys.yaml ]; then \
		echo "keys.yaml not found, copying from config/keys.example.yaml"; \
		cp ./config/keys.example.yaml ./keys.yaml; \
	fi
	@if [ ! -f ./models.yaml ]; then \
		echo "models.yaml not found, copying from config/models.example.yaml"; \
		cp ./config/models.example.yaml ./models.yaml; \
	fi
	@set -a; \
		if [ -f "$(ENV_FILE)" ]; then . "$(ENV_FILE)"; fi; \
	set +a; \
	GIN_MODE=release go run -ldflags "$(LDFLAGS)" ./cmd/onr --config ./onr.yaml

build: ## Build the main binary
	go build -ldflags "$(LDFLAGS)" -o bin/onr ./cmd/onr
	go build -ldflags "$(LDFLAGS)" -o bin/onr-admin ./cmd/onr-admin

build-all: ## Build for all platforms using GoReleaser
	goreleaser build --snapshot --clean

test: ## Run tests
	go test -v ./...
	cd onr-core && go test -v ./...

bench-dsl: ## Run grouped DSL/perf benchmarks with benchmem
	env GOCACHE=$(GO_BUILD_CACHE) go test $(DSL_BENCH_PKGS) -run '^$$' -bench '$(DSL_BENCH_PATTERN)' $(BENCHMEM) -count=$(BENCH_COUNT) $(if $(BENCHTIME),-benchtime=$(BENCHTIME),)

bench-dslconfig: ## Run dslconfig-only benchmarks with benchmem
	env GOCACHE=$(GO_BUILD_CACHE) go test ./onr-core/pkg/dslconfig -run '^$$' -bench 'Benchmark(ExtractUsage_|ExtractFinishReason_|ProviderUsageSelect_|ProviderFinishReasonSelect_|StreamMetricsAggregator_)' $(BENCHMEM) -count=$(BENCH_COUNT) $(if $(BENCHTIME),-benchtime=$(BENCHTIME),)

bench-dsl-pprof: ## Capture a CPU profile for a DSL benchmark and print pprof top
	env GOCACHE=$(GO_BUILD_CACHE) go test ./onr-core/pkg/dslconfig -run '^$$' -bench '$(DSL_PPROF_BENCH)' -count=1 $(if $(BENCHTIME),-benchtime=$(BENCHTIME),) -cpuprofile $(DSL_PPROF_OUT)
	$$(go env GOPATH)/bin/pprof -top $(DSL_PPROF_OUT)

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

prek-install: ## 安装/更新 prek
	python3 -m pip install --upgrade prek
	prek install -t pre-commit -t commit-msg

prek: ## 执行 prek 全量检查
	prek run --all-files

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/goreleaser/goreleaser@latest
	@echo "Done! GoReleaser installed."
