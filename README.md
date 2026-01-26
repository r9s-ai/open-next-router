# open-next-router

open-next-router (ONR) is a lightweight API gateway that routes OpenAI-style endpoints to upstream providers via declarative provider configuration (DSL).

## Quick Start

1) Prepare configs

- Copy `open-next-router.example.yaml` -> `open-next-router.yaml`
- Copy `keys.example.yaml` -> `keys.yaml`
- Copy `models.example.yaml` -> `models.yaml`
- Copy provider configs under `providers/` (or write your own)

2) Run

```bash
cd open-next-router
go run ./cmd/open-next-router --config ./open-next-router.yaml
```

## Docker Compose

Create runtime config files first:

```bash
cd open-next-router
cp open-next-router.example.yaml open-next-router.yaml
cp keys.example.yaml keys.yaml
docker compose up --build
```

3) Call

```bash
curl -sS http://127.0.0.1:3000/v1/chat/completions \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

## Auth

- `Authorization: Bearer <ONR_API_KEY>`
- `x-api-key: <ONR_API_KEY>` (compatible)

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
echo -n 'sk-xxxx' | go run ./cmd/onr-crypt
```

### Env override (recommended for CI / docker / k8s)

For each key entry, you can override the value via environment variable:

- If `name` is set: `ONR_UPSTREAM_KEY_<PROVIDER>_<NAME>`
- Otherwise: `ONR_UPSTREAM_KEY_<PROVIDER>_<INDEX>` (1-based)

Example:

- `providers.openai.keys[0].name: key1` -> `ONR_UPSTREAM_KEY_OPENAI_KEY1`

## Provider Selection

- Override: `x-onr-provider: <provider>`

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
