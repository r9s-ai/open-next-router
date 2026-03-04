# onr-admin Usage

This document summarizes the `onr-admin` CLI usage in one place.

## 1. Basic

```bash
make build
onr-admin <subcommand> [flags]
```

Help:

```bash
onr-admin --help
onr-admin <subcommand> --help
```

## 2. token

Generate a token key in the form of `onr:v1?...`, which can be used in `Authorization: Bearer ...`.

```bash
onr-admin token create --config ./onr.yaml --access-key-name client-a -p openai -m gpt-4o-mini
```

## 3. crypto

Encryption and master key helpers.

```bash
# Encrypt plaintext into ENC[...]
onr-admin crypto encrypt --text 'sk-xxxx'

# Decrypt ENC[...] into plaintext
onr-admin crypto decrypt --text 'ENC[v1:aesgcm:...]'

# Encrypt plaintext values in keys.yaml in-place
onr-admin crypto encrypt-keys --config ./onr.yaml

# Generate a random ONR_MASTER_KEY (base64)
onr-admin crypto gen-master-key --export
```

## 4. validate

Validate config files.

```bash
onr-admin validate all --config ./onr.yaml
```

## 5. balance

Query upstream balance using the providers DSL registry.

```bash
onr-admin balance get --config ./onr.yaml -p openai
onr-admin balance get --config ./onr.yaml --providers openai,openrouter,deepseek
onr-admin balance get --config ./onr.yaml --all
onr-admin balance get --config ./onr.yaml -p moonshot --debug
```

## 6. pricing

Sync model pricing from `https://models.dev/api.json` into `price.yaml`.

```bash
# List providers on models.dev
onr-admin pricing providers
onr-admin pricing providers --search openai

# If --provider/--providers is omitted, it loads all providers from providers.dir in onr.yaml
onr-admin pricing sync --config ./onr.yaml --out ./price.yaml

onr-admin pricing sync -p openai --models gpt-4o-mini,gpt-4o --out ./price.yaml
onr-admin pricing sync --providers openai,anthropic --out ./price.yaml

# Provider alias example: gemini maps to models.dev's google
onr-admin pricing sync -p gemini --models gemini-2.5-flash --out ./price.yaml
```

## 7. oauth

Get OAuth `refresh_token` for a selected provider profile (authorization code flow).

```bash
# Provider is required
onr-admin oauth refresh-token --provider openai

# Select built-in provider profile
onr-admin oauth refresh-token --provider claude
onr-admin oauth refresh-token --provider gemini --client-id "<your-google-client-id>" --client-secret "<your-google-client-secret>"
onr-admin oauth refresh-token --provider qwen
onr-admin oauth refresh-token --provider kimi

# client-secret can be loaded from env automatically:
#   ONR_OAUTH_<PROVIDER>_CLIENT_SECRET
export ONR_OAUTH_IFLOW_CLIENT_SECRET="<your-iflow-client-secret>"
onr-admin oauth refresh-token --provider iflow

# Headless server mode: print URL and wait callback
onr-admin oauth refresh-token --provider openai --no-browser --callback-port 2468

# qwen/kimi use OAuth device-code flow (prints verification URL and optional user code)
onr-admin oauth refresh-token --provider qwen --no-browser

# Custom provider: override OAuth endpoints/params explicitly
onr-admin oauth refresh-token \
  --provider custom \
  --auth-url "https://example.com/oauth/authorize" \
  --token-url "https://example.com/oauth/token" \
  --client-id "your-client-id" \
  --client-secret "your-client-secret" \
  --scope "openid profile offline_access" \
  --auth-param "prompt=consent"
```

## 8. update

Update runtime binaries or provider configs from GitHub Release assets.

```bash
# Update only onr binary
onr-admin update onr

# Update only onr-admin binary
onr-admin update onr-admin

# Update providers dir from open-next-router_config_vX.Y.Z.tar.gz
onr-admin update providers --config ./onr.yaml

# Update all (order: onr -> providers -> onr-admin)
onr-admin update all

# Pin a specific runtime version
onr-admin update all --version v1.2.3
```

Notes:

- `--all` flag is not supported. Use `onr-admin update all`.
- Shared flags: `--version`, `--repo`, `--config`, `--providers-dir`, `--onr-bin`, `--onr-admin-bin`, `--backup`.
- Providers update validates the full providers directory after writing.
- ONR runtime is not auto-restarted; run reload manually if needed.

## 9. tui

Open the interactive TUI (dump log viewer).

```bash
onr-admin tui --config ./onr.yaml
```

Notes:

- The TUI reads traffic dump logs from `traffic_dump.dir` (default `./dumps`).
- Key hints: use `↑/↓` to navigate, `enter` to open, `/` to filter by provider/model/path/status/rid, `r` to reload, `q` to quit.

## 10. web

Start local web editor for provider DSL configs.

```bash
onr-admin web --config ./onr.yaml --listen 0.0.0.0:3310
```

Then open `http://127.0.0.1:3310` in your browser.

Optional env for default cURL API base URL shown in page:

```bash
export ONR_ADMIN_WEB_CURL_API_BASE_URL="http://127.0.0.1:3300"
```

Optional env for web listen address (used when `--listen` is not provided):

```bash
export ONR_ADMIN_WEB_LISTEN="0.0.0.0:3310"
```

Behavior:

- Submit `provider + content` to validate against the whole providers directory.
- Save happens only after validation succeeds.
- Target file is `providers.dir/<provider>.conf`.
- Test Response supports extracting `request_id` from response headers (`X-Onr-Request-Id` first, then `X-Request-Id`) and loading the matching dump file inline.
- Dump lookup reads files from `traffic_dump.dir` in config (fallback `./dumps`), so ONR must enable `traffic_dump.enabled=true` and the directory must be accessible.
