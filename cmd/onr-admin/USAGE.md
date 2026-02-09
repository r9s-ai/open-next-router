# onr-admin Usage

This document summarizes the `onr-admin` CLI usage in one place.

## 1. Basic

```bash
make build
./bin/onr-admin <subcommand> [flags]
```

Help:

```bash
./bin/onr-admin --help
./bin/onr-admin <subcommand> --help
```

## 2. token

Generate a token key in the form of `onr:v1?...`, which can be used in `Authorization: Bearer ...`.

```bash
./bin/onr-admin token create --config ./onr.yaml --access-key-name client-a -p openai -m gpt-4o-mini
```

## 3. crypto

Encryption and master key helpers.

```bash
# Encrypt plaintext into ENC[...]
./bin/onr-admin crypto encrypt --text 'sk-xxxx'

# Encrypt plaintext values in keys.yaml in-place
./bin/onr-admin crypto encrypt-keys --config ./onr.yaml

# Generate a random ONR_MASTER_KEY (base64)
./bin/onr-admin crypto gen-master-key --export
```

## 4. validate

Validate config files.

```bash
./bin/onr-admin validate all --config ./onr.yaml
```

## 5. balance

Query upstream balance using the providers DSL registry.

```bash
./bin/onr-admin balance get --config ./onr.yaml -p openai
./bin/onr-admin balance get --config ./onr.yaml --providers openai,openrouter,deepseek
./bin/onr-admin balance get --config ./onr.yaml --all
./bin/onr-admin balance get --config ./onr.yaml -p moonshot --debug
```

## 6. pricing

Sync model pricing from `https://models.dev/api.json` into `price.yaml`.

```bash
# List providers on models.dev
./bin/onr-admin pricing providers
./bin/onr-admin pricing providers --search openai

# If --provider/--providers is omitted, it loads all providers from providers.dir in onr.yaml
./bin/onr-admin pricing sync --config ./onr.yaml --out ./price.yaml

./bin/onr-admin pricing sync -p openai --models gpt-4o-mini,gpt-4o --out ./price.yaml
./bin/onr-admin pricing sync --providers openai,anthropic --out ./price.yaml

# Provider alias example: gemini maps to models.dev's google
./bin/onr-admin pricing sync -p gemini --models gemini-2.5-flash --out ./price.yaml
```

## 7. oauth

Get OAuth `refresh_token` for a selected provider profile (authorization code flow).

```bash
# Provider is required
./bin/onr-admin oauth refresh-token --provider openai

# Select built-in provider profile
./bin/onr-admin oauth refresh-token --provider claude
./bin/onr-admin oauth refresh-token --provider gemini --client-id "<your-google-client-id>" --client-secret "<your-google-client-secret>"
./bin/onr-admin oauth refresh-token --provider qwen
./bin/onr-admin oauth refresh-token --provider kimi

# client-secret can be loaded from env automatically:
#   ONR_OAUTH_<PROVIDER>_CLIENT_SECRET
export ONR_OAUTH_IFLOW_CLIENT_SECRET="<your-iflow-client-secret>"
./bin/onr-admin oauth refresh-token --provider iflow

# Headless server mode: print URL and wait callback
./bin/onr-admin oauth refresh-token --provider openai --no-browser --callback-port 2468

# qwen/kimi use OAuth device-code flow (prints verification URL and optional user code)
./bin/onr-admin oauth refresh-token --provider qwen --no-browser

# Custom provider: override OAuth endpoints/params explicitly
./bin/onr-admin oauth refresh-token \
  --provider custom \
  --auth-url "https://example.com/oauth/authorize" \
  --token-url "https://example.com/oauth/token" \
  --client-id "your-client-id" \
  --client-secret "your-client-secret" \
  --scope "openid profile offline_access" \
  --auth-param "prompt=consent"
```

## 8. tui

Open the interactive TUI (dump log viewer).

```bash
./bin/onr-admin tui --config ./onr.yaml
```

Notes:

- The TUI reads traffic dump logs from `traffic_dump.dir` (default `./dumps`).
- Key hints: use `↑/↓` to navigate, `enter` to open, `/` to filter by provider/model/path/status/rid, `r` to reload, `q` to quit.
