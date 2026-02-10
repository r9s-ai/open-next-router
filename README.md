<div align="center">

# onr (open-next-router)

**A lightweight API gateway for routing OpenAI-compatible endpoints to multiple upstream providers**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![GitHub Stars](https://img.shields.io/github/stars/r9s-ai/open-next-router?style=social)](https://github.com/r9s-ai/open-next-router)
[![GitHub Issues](https://img.shields.io/badge/github-issues-blue?logo=github)](https://github.com/r9s-ai/open-next-router/issues)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/r9s-ai/open-next-router)
[![Telegram](https://img.shields.io/badge/Telegram-Join-blue?logo=telegram)](https://t.me/opennextrouter)

</div>

---

open-next-router (ONR) is a lightweight API gateway that routes OpenAI-style endpoints to upstream providers via declarative provider configuration (DSL).

## Quick Start

1) Prepare configs

- Copy `config/onr.example.yaml` -> `onr.yaml`
- Copy `config/keys.example.yaml` -> `keys.yaml`
- Copy `config/models.example.yaml` -> `models.yaml`
- Provider DSL examples are under `config/providers/` (or write your own)
- Provider DSL reference: `DSL_SYNTAX.md`

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

For more information about version management and releases, see [docs/RELEASE.md](docs/RELEASE.md).

## Version

```bash
go run ./cmd/onr -V
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
│  ┌───────────────┐   ┌───────────────────────┐   ┌────────────────────────┐ │
│  │ request_id    │   │ provider selection    │   │ DSL provider execution │ │
│  │ response hdr  │   │ 1) x-onr-provider     │   │ - routing              │ │
│  │               │   │ 2) models.yaml        │   │ - headers/auth         │ │
│  └───────────────┘   └───────────────────────┘   │ - request transforms   │ │
│                                                  └────────────────────────┘ |
│                                                                             │
│                                                                             │
│                           ┌──────────────────────────┐                      │
│                           │        upstream          │                      │
│                           │ provider base_url + path │                      │
│                           └──────────────────────────┘                      │
└─────────────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
                 ┌──────────────────────────────────────────┐
                 │                Upstream APIs             │
                 │   OpenAI-compatible / Anthropic / etc.   │
                 └──────────────────────────────────────────┘

Observability:
- [ONR] one-line request log
    • always: request_id, status, latency, client_ip, method, path
    • when available: api, model, provider, provider_source, stream, latency_ms
    • when available: upstream_status, finish_reason
    • usage (when available): usage_stage, input_tokens, output_tokens, total_tokens, cache_read_tokens, cache_write_tokens
        - usage_stage=upstream: usage returned by upstream
        - usage_stage=estimate_*: best-effort estimation when upstream usage is missing/zero

- Optional traffic dump (file-based)
    • META
    • ORIGIN
    • UPSTREAM
    • PROXY
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

- `p`: provider (optional)
- `m`: model override (optional; always enforced)
- `uk`: BYOK upstream key (optional; when set, ONR uses it directly to call upstream)

Generate a token key:

```bash
make build
./bin/onr-admin token create \
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
echo -n 'sk-xxxx' | ./bin/onr-admin crypto encrypt
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

# Partnership

<a href="https://llmapis.com?source=https%3A%2F%2Fgithub.com%2Fr9s-ai%2Fopen-next-router" target="_blank"><img src="https://llmapis.com/api/badge/r9s-ai/open-next-router" alt="LLMAPIS" width="60" /></a>

_Partnership with [https://llmapis.com](https://llmapis.com) - Discover more AI tools and resources_
