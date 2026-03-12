PKG_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
PYTHON ?= python3
PIP := $(PYTHON) -m pip

.PHONY: help install test run completion openai-chat openai-chat-stream clean

help:
	@echo "Targets:"
	@echo "  make install            Install Python dependencies"
	@echo "  make test               Run CLI help as a smoke test"
	@echo "  make run                Show CLI help"
	@echo "  make completion         Print zsh completion"
	@echo "  make openai-chat        Run a local OpenAI chat request"
	@echo "  make openai-chat-stream Run a local streaming OpenAI chat request"
	@echo "  make clean              Remove Python cache directories"

install:
	cd $(PKG_DIR) && $(PIP) install -r requirements.txt

test:
	cd $(PKG_DIR) && $(PYTHON) cli.py --help

run:
	cd $(PKG_DIR) && $(PYTHON) cli.py --help

completion:
	cd $(PKG_DIR) && $(PYTHON) cli.py completion --shell zsh

openai-chat:
	cd $(PKG_DIR) && $(PYTHON) cli.py openai chat_completions "Hello"

openai-chat-stream:
	cd $(PKG_DIR) && $(PYTHON) cli.py openai chat_completions "Hello" --stream -v

clean:
	find $(PKG_DIR) -type d -name "__pycache__" -prune -exec rm -rf {} +
