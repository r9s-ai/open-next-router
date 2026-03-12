PKG_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
GO := go
BIN := $(PKG_DIR)onr-sdk

.PHONY: help tidy build test clean run completion openai-chat openai-chat-stream

help:
	@echo "Targets:"
	@echo "  make tidy               Run go mod tidy"
	@echo "  make build              Build the Go CLI binary"
	@echo "  make test               Run Go tests"
	@echo "  make clean              Remove built CLI binary"
	@echo "  make run                Show CLI help"
	@echo "  make completion         Print zsh completion"
	@echo "  make openai-chat        Run a local OpenAI chat request"
	@echo "  make openai-chat-stream Run a local streaming OpenAI chat request"

tidy:
	cd $(PKG_DIR) && $(GO) mod tidy

build:
	cd $(PKG_DIR) && $(GO) build -o $(BIN) ./cmd/onr-sdk

test:
	cd $(PKG_DIR) && $(GO) test ./...

clean:
	rm -f $(BIN)

run:
	cd $(PKG_DIR) && $(GO) run ./cmd/onr-sdk --help

completion:
	cd $(PKG_DIR) && $(GO) run ./cmd/onr-sdk completion --shell zsh

openai-chat:
	cd $(PKG_DIR) && $(GO) run ./cmd/onr-sdk openai chat_completions "Hello"

openai-chat-stream:
	cd $(PKG_DIR) && $(GO) run ./cmd/onr-sdk openai chat_completions "Hello" --stream -v
