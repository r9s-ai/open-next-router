<div align="center">

# onr (open-next-router)

**A lightweight, DSL-driven LLM gateway for routing, patching provider quirks, and enforcing consistent APIs across channels**

[![CI](https://github.com/r9s-ai/open-next-router/actions/workflows/ci.yml/badge.svg)](https://github.com/r9s-ai/open-next-router/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/r9s-ai/open-next-router)](https://github.com/r9s-ai/open-next-router/blob/main/go.mod)
[![Go Reference](https://pkg.go.dev/badge/github.com/r9s-ai/open-next-router.svg)](https://pkg.go.dev/github.com/r9s-ai/open-next-router)
[![Go Report Card](https://goreportcard.com/badge/github.com/r9s-ai/open-next-router)](https://goreportcard.com/report/github.com/r9s-ai/open-next-router)
[![GitHub Release](https://img.shields.io/github/v/release/r9s-ai/open-next-router)](https://github.com/r9s-ai/open-next-router/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Docs](https://img.shields.io/badge/docs-official-0ea5e9)](https://onr.mintlify.app/)

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/r9s-ai/open-next-router)
[![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/r9s-ai/open-next-router)
[![Telegram](https://img.shields.io/badge/Telegram-Join-blue?logo=telegram)](https://t.me/opennextrouter)
[![Discord](https://img.shields.io/badge/Discord-Join-5865F2?logo=discord&logoColor=white)](https://discord.gg/HBM67dP8)

</div>

---

open-next-router (ONR) is a lightweight, DSL-driven LLM gateway that routes requests, applies compatibility patches, and normalizes behavior across providers and channels.

## Why ONR?

- **Atomic, nginx-like DSL**: runtime behavior is explicitly declared in `config/providers/*.conf` (routing, auth headers, transforms, SSE parsing, usage extraction).
- **Fast provider onboarding and patching**: fix provider quirks by editing a `.conf` file instead of changing and redeploying code.
- **Hot reload**: reload `onr.yaml` / `keys.yaml` / `models.yaml` / provider DSL files via SIGHUP; provider DSL can also auto-reload by file watch (opt-in).
- **No hidden magic**: compatibility is opt-in via directives (e.g. `req_map`, `resp_map`, `sse_parse`, `json_del`, `set_header`) rather than implicit heuristics.
- **Streaming-aware normalization**: handle SSE framing and provider-specific streaming semantics while keeping a stable client-facing API.
- **Operational visibility**: one-line request logs with optional usage/cost extraction help you debug channels and control spend.

## DSL (nginx-like, atomic) at a glance

All runtime behavior (routing, auth headers, request/response transforms, SSE parsing, usage extraction, etc.) is explicitly described
by provider DSL files under `config/providers/*.conf`.

```conf
# Minimal: route + auth
# config/providers/acme.conf
syntax "next-router/0.1";

provider "acme" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    auth {
      auth_bearer;
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path "/v1/chat/completions";
    }
    response {
      resp_passthrough;
    }
  }
}
```

```conf
# Extended: opt-in compatibility transforms (examples)
# config/providers/anthropic.conf
syntax "next-router/0.1";

provider "anthropic" {
  defaults {
    upstream_config {
      base_url = "https://api.anthropic.com";
    }
    auth {
      auth_header_key "x-api-key";
    }
    request {
      set_header "anthropic-version" "2023-06-01";
    }
  }

  # OpenAI /v1/chat/completions -> Anthropic /v1/messages (non-stream)
  match api = "chat.completions" stream = false {
    request {
      req_map openai_chat_to_anthropic_messages;
      json_del "$.stream_options";
    }
    upstream {
      set_path "/v1/messages";
    }
    response {
      resp_map anthropic_to_openai_chat;
    }
  }

  # OpenAI /v1/chat/completions -> Anthropic /v1/messages (streaming)
  match api = "chat.completions" stream = true {
    request {
      req_map openai_chat_to_anthropic_messages;
      json_del "$.stream_options";
    }
    upstream {
      set_path "/v1/messages";
    }
    response {
      sse_parse anthropic_to_openai_chunks;
    }
  }
}
```

More examples: `config/providers/` • Full reference: [DSL_SYNTAX.md](https://github.com/r9s-ai/open-next-router/blob/main/DSL_SYNTAX.md)

## Quick Start

### One-click install (Linux, recommended)

Install latest runtime release as a systemd service:

```bash
curl -fsSL https://raw.githubusercontent.com/r9s-ai/open-next-router/main/tools/install_onr_service.sh | sudo bash -s -- \
  --mode service \
  --api-key 'change-me'
```

Health check:

```bash
curl -sS http://127.0.0.1:3300/v1/models -H "Authorization: Bearer change-me"
```

Other install modes:

- Docker: `--mode docker`
- Docker Compose: `--mode docker-compose`

### Run from source (development)

1) Prepare configs

- Copy `config/onr.example.yaml` -> `onr.yaml`
- Copy `config/keys.example.yaml` -> `keys.yaml`
- Copy `config/models.example.yaml` -> `models.yaml`

2) Run

```bash
cd open-next-router
go run ./cmd/onr --config ./onr.yaml
```

3) Reload (nginx-like)

After editing `onr.yaml` / `keys.yaml` / `models.yaml` / provider DSL files, you can reload runtime configs by sending SIGHUP:

```bash
go run ./cmd/onr --config ./onr.yaml -s reload
```

This uses `server.pid_file` (default: `/var/run/onr.pid`).

Optional: enable provider DSL auto-reload by file watch (disabled by default):

```yaml
providers:
  dir: "./config/providers"
  auto_reload:
    enabled: true
    debounce_ms: 300
```

4) Test config (nginx-like)

Test configs without starting the server:

```bash
# default config path
go run ./cmd/onr -t

# specify config file (flag)
go run ./cmd/onr -t -c ./onr.yaml

# specify config file (positional)
go run ./cmd/onr -t ./onr.yaml
```

5) Setup Git hooks with prek

```bash
# install git hooks (force-replace if pre-commit hooks already exist)
prek install -f

# run all hooks manually
prek run --all-files
```

## Docker Compose

Create runtime config files first:

```bash
cd open-next-router
cp config/onr.example.yaml onr.yaml
cp config/keys.example.yaml keys.yaml
cp config/models.example.yaml models.yaml
docker compose up --build
```

3) Call

```bash
curl -sS http://127.0.0.1:3300/v1/chat/completions \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

## Architecture (high level)

```text
                    ┌─────────────────────────────────────────┐
                    │              open-next-router           │
                    │                (Gin server)             │
                    └─────────────────────────────────────────┘
                                      │
                                      │ Auth: Bearer / x-api-key
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Request Pipeline                               │
│                                                                             │
│  ┌──────────────────────────────┐  ┌───────────────────────┐  ┌───────────┐ │
│  │ request_id + access log      │  │ provider selection    │  │ DSL exec  │ │
│  │ optional traffic dump        │  │ 1) x-onr-provider     │  │ (phases)  │ │
│  └──────────────────────────────┘  │ 2) models.yaml        │  └───────────┘ │
│                                    └───────────────────────┘                │
│                                                                             │
│  DSL phases (explicit, nginx-like):                                         │
│  - upstream_config: base_url (and per-channel overrides)                    │
│  - auth: auth header shape, optional OAuth exchange + bearer injection      │
│  - request: header/query/json patching, request mapping (compat mode)       │
│  - upstream: set_path/set_query, proxy settings                             │
│  - response: passthrough / resp_map / sse_parse (streaming normalization)   │
│  - error: error_map                                                         │
│  - metrics/pricing: usage_extract, cost estimation (optional)               │
│                                                                             │
│                                                                             │
│                         ┌──────────────────────────┐                        │
│                         │        upstream          │                        │
│                         │ provider base_url + path │                        │
│                         └──────────────────────────┘                        │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
                    ┌──────────────────────────────────────────┐
                    │                Upstream APIs             │
                    │   OpenAI-compatible and native provider  │
                    │   APIs (Anthropic, Gemini, etc.)         │
                    └──────────────────────────────────────────┘

Observability:
- [ONR] one-line request log
    • base (always): ts, status, latency, client_ip, method, path
    • fields (always): request_id, latency_ms
    • routing (when available): api, provider, provider_source, model, stream
    • upstream (when available): upstream_status, finish_reason
    • usage (when available): usage_stage, input_tokens, output_tokens, total_tokens, cache_read_tokens, cache_write_tokens, billable_input_tokens
    • cost (when enabled/available): cost_total, cost_input, cost_output, cost_cache_read, cost_cache_write, cost_multiplier, cost_model, cost_channel, cost_unit
        - usage_stage=upstream: usage returned by upstream
        - usage_stage=estimate_*: best-effort estimation when upstream usage is missing/zero

- Optional traffic dump (file-based)
    • META
    • ORIGIN
    • UPSTREAM
    • PROXY

Config reload:
- Send SIGHUP to reload `onr.yaml` / `keys.yaml` / `models.yaml` / `config/providers/*.conf` (nginx-like)
- Optional: enable `providers.auto_reload.enabled=true` to watch `providers.dir` and auto-reload provider DSL files
```

## Auth

- Recommended: `Authorization: Bearer <ACCESS_KEY_FROM_KEYS_YAML>`
- Compatible headers: `x-api-key` / `x-goog-api-key`
- `onr.yaml` can omit `auth` entirely when using `keys.yaml` `access_keys`
- Optional legacy mode: `auth.api_key` (master key in `onr.yaml`)

### URI-like token key (onr:v1?)

If your client can only set a single API key and cannot add custom headers, you can use a URI-like token key:

**No-sig mode (editable):**

`onr:v1?k=<ACCESS_KEY>&{query_without_k}`

or

`onr:v1?k64=<base64url(ACCESS_KEY)>&{query_without_k64}`

Supported query params:

- `k` / `k64`: access key (required by default)
- `p`: provider (optional)
- `m`: model override (optional; always enforced)
- `uk`: BYOK upstream key (optional; when set, ONR uses it directly to call upstream)

Optional config to allow BYOK token without `k`/`k64` (default: false):

```yaml
auth:
  token_key:
    allow_byok_without_k: true
```

Generate a token key:

```bash
make build
onr-admin token create \
  --config ./onr.yaml \
  --access-key-name client-a \
  --provider openai \
  --model gpt-4o-mini
```

More details: see `docs/ACCESS_KEYS_CN.md`.

## Upstream Keys (keys.yaml)

### Plaintext

You can put plaintext keys in `keys.yaml` (not recommended for public repos).

### Encrypted values (AES-256-GCM)

`keys.yaml` supports encrypted values in this format:

`ENC[v1:aesgcm:<base64(nonce+ciphertext)>]`

To decrypt `ENC[...]` values, set `ONR_MASTER_KEY` (32 bytes or base64-encoded 32 bytes).

To generate an encrypted value:

```bash
export ONR_MASTER_KEY='...'
echo -n 'sk-xxxx' | onr-admin crypto encrypt
```

### Env override (recommended for CI / docker / k8s)

For each key entry, you can override the value via environment variable:

- If `name` is set: `ONR_UPSTREAM_KEY_<PROVIDER>_<NAME>`
- Otherwise: `ONR_UPSTREAM_KEY_<PROVIDER>_<INDEX>` (1-based)

Example:

- `providers.openai.keys[0].name: key1` -> `ONR_UPSTREAM_KEY_OPENAI_KEY1`

## Access Keys (keys.yaml: access_keys)

`keys.yaml` can also contain access keys for clients:

```yaml
access_keys:
  - name: "client-a"
    value: "ak-xxx"
    comment: "iOS app"
```

Env override:

- If `name` is set: `ONR_ACCESS_KEY_<NAME>` (e.g. `ONR_ACCESS_KEY_CLIENT_A`)
- Otherwise: `ONR_ACCESS_KEY_<INDEX>` (1-based)

## Admin CLI (onr-admin)

`onr-admin` command usage is documented in:

- `onr-admin/USAGE.md`

## Multi-Module Layout

The repository now uses two Go modules:

- `.` (onr runtime/server + onr-admin CLI)
- `onr-core` (shared library for external ecosystem and internal reuse)

For local multi-module development, use the repository root `go.work`.

Quick checks:

```bash
# onr
go test ./...

# onr-core
(cd onr-core && go test ./...)

# onr-admin (included in root module)
go test ./...
```

### onr-core stable versioning for external consumers

`onr-core` is released with dedicated submodule tags: `onr-core/vX.Y.Z`.

```bash
go get github.com/r9s-ai/open-next-router/onr-core@v1.2.3
```

## Upstream HTTP Proxy (per provider)

You can configure an outbound HTTP proxy per upstream provider in `onr.yaml`:

```yaml
upstream_proxies:
  by_provider:
    openai: "http://127.0.0.1:7890"
    anthropic: "http://127.0.0.1:7891"
```

Supported proxy URL schemes:

- `http://` / `https://`
- `socks5://` / `socks5h://` (optional user/pass: `socks5://user:pass@host:port`)

Or override via environment variables:

- `ONR_UPSTREAM_PROXY_OPENAI=http://127.0.0.1:7890`
- `ONR_UPSTREAM_PROXY_ANTHROPIC=http://127.0.0.1:7891`

## OAuth Token Persistence

When provider DSL `auth` uses OAuth directives, ONR can persist exchanged access tokens to local files.

```yaml
oauth:
  token_persist:
    enabled: true
    dir: "./run/oauth"
```

Environment overrides:

- `ONR_OAUTH_TOKEN_PERSIST_ENABLED=true|false`
- `ONR_OAUTH_TOKEN_PERSIST_DIR=./run/oauth`

## Provider Selection

- Override: `x-onr-provider: <provider>`

## Gemini Native API (v1beta)

In addition to OpenAI-style endpoints, open-next-router supports a subset of Gemini native endpoints:

- `POST /v1beta/models/{model}:generateContent`
- `POST /v1beta/models/{model}:streamGenerateContent` (SSE; `alt=sse` will be added if missing)
- `GET /v1beta/models` (Gemini-style output)

Example (force provider selection via header):

```bash
curl -sS http://127.0.0.1:3300/v1beta/models/gemini-2.0-flash:generateContent \
  -H "Authorization: Bearer change-me" \
  -H "x-onr-provider: gemini" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}'
```

## Model Routing (models.yaml)

You can bind a model to one or more providers. If a model is bound to multiple providers,
open-next-router selects the provider using round-robin (per model).

Selection priority:

1) `x-onr-provider` header (force)
2) `models.yaml` routing (per model round-robin)

## Traffic Dump (files)

Enable file-based traffic dump to capture request/response for debugging.

Configuration (config or env):

- `traffic_dump.enabled` / `ONR_TRAFFIC_DUMP_ENABLED`
- `traffic_dump.dir` / `ONR_TRAFFIC_DUMP_DIR`
- `traffic_dump.file_path` / `ONR_TRAFFIC_DUMP_FILE_PATH` (template supports `{{.request_id}}`)
- `traffic_dump.max_bytes` / `ONR_TRAFFIC_DUMP_MAX_BYTES`
- `traffic_dump.mask_secrets` / `ONR_TRAFFIC_DUMP_MASK_SECRETS`

Captured sections:

- `=== META ===`
- `=== ORIGIN REQUEST ===`
- `=== UPSTREAM REQUEST ===`
- `=== UPSTREAM RESPONSE ===`
- `=== PROXY RESPONSE ===`

## System Log (runtime)

System logs are emitted to `stderr` in single-line text with a fixed prefix, and optional trailing KV fields.

Configuration (config or env):

- `logging.level` (`debug` | `info` | `warn` | `error`)
- `ONR_LOG_LEVEL`

Example:

```text
[ONR] 2026/02/27 - 12:34:56 | INFO | startup | startup config loaded | config_path=./onr.yaml providers_dir=./config/providers keys_file=./keys.yaml models_file=./models.yaml
[ONR] 2026/02/27 - 12:34:56 | INFO | startup | startup runtime flags | access_log_enabled=true access_log_target=stdout traffic_dump_enabled=false providers_auto_reload_enabled=false
[ONR] 2026/02/27 - 12:34:56 | INFO | server | open-next-router listening | listen_url=http://127.0.0.1:3300
```

Startup summary includes key runtime status fields:

- `config_path`
- `providers_dir` / `keys_file` / `models_file`
- `traffic_dump_enabled` / `traffic_dump_dir` / `traffic_dump_max_bytes`
- `access_log_enabled` / `access_log_target`
- `providers_auto_reload_enabled` / `providers_auto_reload_debounce_ms`
- `listen_url` (server listening log)

## Access Log Rotation

Built-in access log rotation is optional and applies to file output (`logging.access_log_path`).

Configuration (config or env):

- `logging.access_log_rotate.enabled` / `ONR_ACCESS_LOG_ROTATE_ENABLED`
- `logging.access_log_rotate.max_size_mb` / `ONR_ACCESS_LOG_ROTATE_MAX_SIZE_MB`
- `logging.access_log_rotate.max_backups` / `ONR_ACCESS_LOG_ROTATE_MAX_BACKUPS`
- `logging.access_log_rotate.max_age_days` / `ONR_ACCESS_LOG_ROTATE_MAX_AGE_DAYS`
- `logging.access_log_rotate.compress` / `ONR_ACCESS_LOG_ROTATE_COMPRESS`

Notes:

- When `logging.access_log_rotate.enabled=true`, `logging.access_log_path` must be non-empty.
- Rotation triggers on day boundary (local time) or when the file size threshold is exceeded.

# Partnership

<a href="https://llmapis.com?source=https%3A%2F%2Fgithub.com%2Fr9s-ai%2Fopen-next-router" target="_blank"><img src="https://llmapis.com/api/badge/r9s-ai/open-next-router" alt="LLMAPIS" width="60" /></a>

_Partnership with [https://llmapis.com](https://llmapis.com) - Discover more AI tools and resources_
