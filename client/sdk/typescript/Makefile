PKG_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
NPM := npm --prefix $(PKG_DIR)
NODE := node
CLI := $(PKG_DIR)dist/cli.cjs

.PHONY: help install build test typecheck clean openai-chat openai-chat-stream

help:
	@echo "Targets:"
	@echo "  make install           Install package dependencies"
	@echo "  make build             Build ESM/CJS outputs"
	@echo "  make test              Run vitest"
	@echo "  make typecheck         Run TypeScript typecheck"
	@echo "  make clean             Remove dist output"
	@echo "  make openai-chat       Run a local OpenAI chat request"
	@echo "  make openai-chat-stream Run a local streaming OpenAI chat request"

install:
	$(NPM) install

build:
	$(NPM) run build

test:
	$(NPM) test

typecheck:
	$(NPM) run typecheck

clean:
	rm -rf $(PKG_DIR)dist

openai-chat: build
	$(NODE) $(CLI) openai chat_completions "Hello"

openai-chat-stream:
	$(NODE) $(CLI) openai chat_completions "Hello" --stream -v
