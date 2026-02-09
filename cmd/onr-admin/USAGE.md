# onr-admin Usage

This document summarizes the `onr-admin` CLI usage in one place.

## 1. Basic

```bash
go run ./cmd/onr-admin <subcommand> [flags]
```

Help:

```bash
go run ./cmd/onr-admin --help
go run ./cmd/onr-admin <subcommand> --help
```

## 2. token

Generate a token key in the form of `onr:v1?...`, which can be used in `Authorization: Bearer ...`.

```bash
go run ./cmd/onr-admin token create --config ./onr.yaml --access-key-name client-a -p openai -m gpt-4o-mini
```

## 3. crypto

Encryption and master key helpers.

```bash
# Encrypt plaintext into ENC[...]
go run ./cmd/onr-admin crypto encrypt --text 'sk-xxxx'

# Encrypt plaintext values in keys.yaml in-place
go run ./cmd/onr-admin crypto encrypt-keys --config ./onr.yaml

# Generate a random ONR_MASTER_KEY (base64)
go run ./cmd/onr-admin crypto gen-master-key --export
```

## 4. validate

Validate config files.

```bash
go run ./cmd/onr-admin validate all --config ./onr.yaml
```

## 5. balance

Query upstream balance using the providers DSL registry.

```bash
go run ./cmd/onr-admin balance get --config ./onr.yaml -p openai
go run ./cmd/onr-admin balance get --config ./onr.yaml --providers openai,openrouter,deepseek
go run ./cmd/onr-admin balance get --config ./onr.yaml --all
go run ./cmd/onr-admin balance get --config ./onr.yaml -p moonshot --debug
```

## 6. pricing

Sync model pricing from `https://models.dev/api.json` into `price.yaml`.

```bash
# List providers on models.dev
go run ./cmd/onr-admin pricing providers
go run ./cmd/onr-admin pricing providers --search openai

# If --provider/--providers is omitted, it loads all providers from providers.dir in onr.yaml
go run ./cmd/onr-admin pricing sync --config ./onr.yaml --out ./price.yaml

go run ./cmd/onr-admin pricing sync -p openai --models gpt-4o-mini,gpt-4o --out ./price.yaml
go run ./cmd/onr-admin pricing sync --providers openai,anthropic --out ./price.yaml

# Provider alias example: gemini maps to models.dev's google
go run ./cmd/onr-admin pricing sync -p gemini --models gemini-2.5-flash --out ./price.yaml
```

## 7. tui

Open the interactive TUI.

```bash
go run ./cmd/onr-admin tui --config ./onr.yaml
```
