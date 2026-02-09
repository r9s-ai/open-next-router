# Provider Config DSL Syntax (v0.1)

This document describes the provider configuration DSL used by **onr** (open-next-router).
Provider configs live under `config/providers/*.conf` in this repository and are loaded at startup (and on reload).
The directory can be customized via `providers.dir` (config) or `ONR_PROVIDERS_DIR` (env).

## Table of Contents

- [1. Conventions](#1-conventions)
- [2. include (reusable fragments)](#2-include-reusable-fragments)
- [3. Top-level structure](#3-top-level-structure)
  - [3.1 syntax](#31-syntax)
  - [3.2 provider](#32-provider)
- [4. match rules (selection)](#4-match-rules-selection)
- [5. Phases / blocks (can appear in defaults and match)](#5-phases--blocks-can-appear-in-defaults-and-match)
  - [5.1 upstream_config](#51-upstream_config)
  - [5.2 auth](#52-auth)
  - [5.3 request](#53-request)
  - [5.4 upstream](#54-upstream)
  - [5.5 response](#55-response)
  - [5.6 error](#56-error)
  - [5.7 metrics (usage extraction)](#57-metrics-usage-extraction)
- [6. Expression context (built-in variables)](#6-expression-context-built-in-variables)
- [7. Directive reference (nginx style)](#7-directive-reference-nginx-style)
  - [7.1 file-level directives](#71-file-level-directives)
  - [7.2 provider (structure blocks)](#72-provider-structure-blocks)
  - [7.3 upstream_config](#73-upstream_config)
  - [7.4 auth](#74-auth)
  - [7.5 request](#75-request)
  - [7.6 upstream](#76-upstream)
  - [7.7 response](#77-response)
  - [7.8 error](#78-error)
  - [7.9 metrics (usage extraction)](#79-metrics-usage-extraction)
- [8. Built-in variables (reference)](#8-built-in-variables-reference)

---

## 1. Conventions

- **One provider per file**: `config/providers/<provider>.conf`
- **File name must match provider name**: e.g. `config/providers/openai.conf` must contain `provider "openai" { ... }`
- **Provider name matching is case-insensitive**: everything is normalized to lowercase for matching; use lowercase in configs
- **Semicolon `;` is required**: every statement ends with `;`
- **Only blocks use `{}`**: directives themselves do not use `{}` (nginx-like style)
- **Recommended style: one directive per line** (better diffs and readability)
- Comments are supported: `#`, `//`, `/* ... */`

Recommended formatting example:

```conf
upstream_config {
  base_url = "https://api.example.com";
}

match api = "chat.completions" {
  upstream {
    set_path "/v1/chat/completions";
    set_query stream "true";
  }
  response {
    resp_passthrough;
  }
}
```

## 2. include (reusable fragments)

Syntax:

```conf
include "relative/or/absolute/path.conf";
```

- Relative paths are resolved against the current file directory.
- Max recursion depth: 20.
- Cycles are detected: a cyclic include fails parsing for that provider.

## 3. Top-level structure

```conf
syntax "next-router/0.1";

provider "openai" {
  defaults { ... }

  match api = "chat.completions" stream = true { ... }
  match api = "chat.completions" stream = false { ... }
  match api = "embeddings" { ... }
}
```

### 3.1 syntax

`syntax "next-router/0.1";` is a version marker / placeholder. The parser currently does not strictly validate it,
but it is recommended to keep it.

### 3.2 provider

```conf
provider "<name>" { ... }
```

## 4. match rules (selection)

Supported forms:

```conf
match api = "<api-name>" stream = true { ... }
match api = "<api-name>" stream = false { ... }
match api = "<api-name>" { ... }
```

- v0.1 only matches on `api` and optional `stream`.
- **First match wins** (top-to-bottom order in the file).
  - Put more specific rules first, more general rules later.
- If a provider is selected (DSL enabled) but **no match** is found, the request is rejected with **HTTP 400**.
  This avoids silent fallback behavior.

Currently supported `api` values (aligned with OpenAI-style endpoints):

- `completions`
- `chat.completions`
- `responses`
- `claude.messages`
- `embeddings`
- `images.generations`
- `audio.speech`
- `audio.transcriptions`
- `audio.translations`
- `gemini.generateContent` (Gemini native: `POST /v1beta/models/{model}:generateContent`)
- `gemini.streamGenerateContent` (Gemini native: `POST /v1beta/models/{model}:streamGenerateContent?alt=sse`)

## 5. Phases / blocks (can appear in defaults and match)

Available blocks:

- `upstream_config { ... }`
- `auth { ... }`
- `request { ... }`
- `upstream { ... }`
- `response { ... }`
- `error { ... }`
- `metrics { ... }`

Merge rule (important):

- `defaults` is applied first; the selected `match` is applied afterwards, so match settings can override defaults.

### 5.1 upstream_config

```conf
upstream_config {
  base_url = "https://api.example.com";
}
```

- `base_url` is **required** and must be a **string literal** (a fixed URL).
- `base_url` is also a default value: if the channel/provider key defines a `base_url` override at runtime, it should
  override this default; otherwise this default is used.

### 5.2 auth

Purpose: declare the **auth header shape**. Do not put token/value in `.conf`.
The token/value always comes from the runtime upstream key (equivalent to `$channel.key` in next-router).

#### auth_bearer

```conf
auth { auth_bearer; }
```

Effect: `Authorization: Bearer <channel.key>`

#### auth_header_key

```conf
auth { auth_header_key "x-api-key"; }
auth { auth_header_key api_key; }
```

Effect: `<Header-Name>: <channel.key>`

#### OAuth token exchange + bearer injection

```conf
auth {
  oauth_mode openai;              # openai|gemini|qwen|claude|iflow|antigravity|kimi|custom
  oauth_refresh_token $channel.key;
  auth_oauth_bearer;
}
```

- `oauth_mode` enables runtime OAuth token exchange before upstream calls.
- `auth_oauth_bearer;` injects `Authorization: Bearer <oauth.access_token>`.
- Builtin modes provide provider-specific defaults (token endpoint / request format).
- `custom` mode requires explicit `oauth_token_url` and at least one `oauth_form`.
- Optional overrides:
  - `oauth_token_url <expr>;`
  - `oauth_client_id <expr>;`
  - `oauth_client_secret <expr>;`
  - `oauth_refresh_token <expr>;` (default: `$channel.key`)
  - `oauth_scope <expr>;`
  - `oauth_audience <expr>;`
  - `oauth_method <GET|POST>;`
  - `oauth_content_type <form|json>;`
  - `oauth_form <key> <expr>;` (multiple allowed)
  - `oauth_token_path <jsonpath>;` (default: `$.access_token`)
  - `oauth_expires_in_path <jsonpath>;` (default: `$.expires_in`)
  - `oauth_token_type_path <jsonpath>;` (default: `$.token_type`)
  - `oauth_timeout_ms <int>;` (default: `5000`)
  - `oauth_refresh_skew_sec <int>;` (default: `300`)
  - `oauth_fallback_ttl_sec <int>;` (default: `1800`)

### 5.3 request

In v0.1, `request` provides request-header operations and lightweight JSON body transforms.

#### set_header (multiple allowed)

```conf
request {
  set_header "x-trace-id" "trace-123";
  set_header "x-foo" concat("a-", $request.model_mapped);
}
```

- Multiple directives are allowed; executed in order.
- If the same header is set multiple times, the last one wins.

#### del_header (multiple allowed)

```conf
request {
  del_header "x-remove-me";
}
```

- Multiple directives are allowed; executed in order.
- Defaults are applied before match; match directives are appended after defaults.

#### model_map (multiple allowed)

```conf
request {
  model_map "gpt-4o-mini" "gpt4o-mini-prod";
  model_map "gpt-4o-mini" $request.model;
}
```

- Maps `$request.model` to `$request.model_mapped`, for use in `set_path/set_query/set_header` expressions.
- Exact match on the `from` model name.
- If the same `from` appears multiple times, the last one wins.

#### model_map_default (multiple allowed; last one wins)

```conf
request {
  model_map_default $request.model;
}
```

- If no `model_map <from> ...;` matches, the default expression is used for `$request.model_mapped`.
- If not configured, `$request.model_mapped` defaults to `$request.model`.

#### json_set / json_set_if_absent / json_del / json_rename (multiple allowed)

```conf
request {
  json_set "$.stream" true;
  json_set_if_absent "$.instructions" "";
  json_set "$.user" "alice";
  json_rename "$.max_tokens" "$.max_completion_tokens";
  json_del "$.tools";
}
```

- Applies lightweight transforms to the upstream request JSON.
- JSONPath (v0.1) supports an object-path subset: `$.a.b.c` (no array indices for these request ops).
- `json_set` value expressions support: `true/false/null`, integer, string literal, variable, `concat(...)`.
- `json_set_if_absent` only sets when the path does not exist; existing values are preserved.

#### req_map

```conf
request { req_map <mode>; }
```

Built-in request mapping (non-streaming JSON transform). If multiple directives are present, the **last one wins**.

v0.1 includes:

- `openai_chat_to_openai_responses`: OpenAI-compatible `chat.completions` request JSON → OpenAI `/responses` request JSON
- `anthropic_to_openai_chat`: Anthropic `/v1/messages` request JSON → OpenAI `chat.completions` request JSON
- `gemini_to_openai_chat`: Gemini `generateContent` request JSON → OpenAI `chat.completions` request JSON
- `openai_chat_to_gemini_generate_content`: OpenAI `chat.completions` request JSON → Gemini `generateContent` request JSON
- `openai_chat_to_anthropic_messages`: OpenAI `chat.completions` request JSON → Anthropic `/v1/messages` request JSON

### 5.4 upstream

#### set_path

```conf
upstream {
  set_path "/v1/chat/completions";
}
```

- Sets the upstream request path (overrides the original path).

#### set_query (multiple allowed)

```conf
upstream {
  set_query "api-version" "2024-10-01";
  set_query stream "true";
  # Use built-in variables without quotes (variables are not expanded inside string literals):
  #   ✅ set_query key $channel.key;
  #   ❌ set_query key "$channel.key";  # becomes the literal string "$channel.key"
}
```

- Multiple directives are allowed.
- If the same key is set multiple times, the last one wins.
- **Important:** built-in variables (e.g. `$channel.key`, `$request.model_mapped`) are only expanded when used as bare expressions.
  If you wrap them in double quotes, they are treated as plain string literals and will not be expanded.

#### del_query (multiple allowed)

```conf
upstream {
  del_query "foo";
  del_query bar;
}
```

- Multiple directives are allowed.
- Execution order: all `del_query` first, then all `set_query`.

### 5.5 response

This phase selects a response strategy. If multiple directives are present, **the last one wins**.

#### resp_passthrough

```conf
response { resp_passthrough; }
```

Pass through the upstream response (already OpenAI-compatible).

#### resp_map

```conf
response { resp_map <mode>; }
```

Non-streaming response mapping (e.g. vendor JSON → OpenAI JSON).

#### sse_parse

```conf
response { sse_parse <mode>; }
```

Streaming SSE mapping (e.g. vendor SSE → OpenAI stream chunks).

Available modes depend on the built-in implementation. v0.1 includes:

- `anthropic_to_openai_chat` (`resp_map`): Anthropic `/v1/messages` JSON → OpenAI `chat.completions` JSON
- `anthropic_to_openai_chunks` (`sse_parse`): Anthropic `/v1/messages` SSE → OpenAI stream chunks
- `openai_to_anthropic_messages` (`resp_map`): OpenAI-compatible `chat.completions` JSON → Anthropic `/v1/messages` JSON
- `openai_to_anthropic_chunks` (`sse_parse`): OpenAI-compatible `chat.completions` SSE → Anthropic `/v1/messages` SSE
- `openai_to_gemini_chat` / `openai_to_gemini_generate_content` (`resp_map`): OpenAI-compatible `chat.completions` JSON → Gemini `generateContent` JSON
- `gemini_to_openai_chat` (`resp_map`): Gemini `generateContent` JSON → OpenAI `chat.completions` JSON
- `openai_to_gemini_chunks` (`sse_parse`): OpenAI-compatible `chat.completions` SSE → Gemini SSE
- `gemini_to_openai_chat_chunks` (`sse_parse`): Gemini SSE → OpenAI `chat.completions` SSE chunks
- `openai_responses_to_openai_chat` (`resp_map`): OpenAI/Azure `/responses` JSON → OpenAI `chat.completions` JSON
- `openai_responses_to_openai_chat_chunks` (`sse_parse`): OpenAI/Azure `/responses` SSE → OpenAI `chat.completions` SSE chunks

#### json_del / json_set / json_set_if_absent / json_rename (response JSON ops)

These directives apply **best-effort** JSON mutations to the downstream response body.

```conf
response {
  json_del "$.usage";
  json_set "$.foo" "bar";
  json_set_if_absent "$.bar" "baz";
  json_rename "$.a" "$.b";
}
```

Semantics:

- Non-streaming JSON: apply to the whole JSON **object** response body.
- Streaming SSE (`text/event-stream`): apply to each SSE event's joined `data:` JSON **object** payload.
- Non-JSON / non-object payloads are passed through unchanged.
- Execution order follows the order in the config block.

Limitations (v0.1):

- JSON path is restricted to object paths like `$.a.b.c` (no array indexes).

#### sse_json_del_if (conditional delete for SSE)

```conf
response {
  # If the SSE event JSON payload has $.type == "message_delta", delete $.usage in that event only.
  sse_json_del_if "$.type" "message_delta" "$.usage";
}
```

- Only applies to `text/event-stream`.
- Condition requires the value at `<cond_path>` to be a string and to **exactly** equal `<equals>`.
- Rules are executed in order, before `json_*` response ops.

### 5.6 error

```conf
error { error_map openai; }
```

- Allowed modes (whitelist enforced at load time): `openai`, `common`, `passthrough`.
- If multiple directives are present, the last one wins.

### 5.7 metrics (usage / finish_reason extraction)

#### usage_extract

```conf
metrics { usage_extract openai; }
metrics { usage_extract anthropic; }
metrics { usage_extract gemini; }
metrics { usage_extract custom; }
```

- `openai`: OpenAI/OpenAI-compatible usage fields
- `anthropic`: Anthropic usage fields
- `gemini`: Gemini native usage fields (`usageMetadata.*`)
- `custom`: extract from response JSON via a restricted JSONPath subset and optional arithmetic (see below)

#### Custom extraction fields (recommended with `custom`)

```conf
metrics {
  usage_extract custom;
  input_tokens_path "$.usage.input_tokens";
  output_tokens_path "$.usage.output_tokens";
  cache_read_tokens_path "$.usage.cache_read_input_tokens";
  cache_write_tokens_path "$.usage.cache_creation_input_tokens";
}
```

Supported JSONPath subset:

- `$.a.b.c`
- `$.items[0].x`
- `$.items[*].x` (sum of numeric values in the array)

Multiple/override rules:

- These fields are optional overrides; if the same field appears multiple times, the last one wins.

#### finish_reason_extract

```conf
metrics { finish_reason_extract openai; }
metrics { finish_reason_extract anthropic; }
metrics { finish_reason_extract gemini; }
metrics { finish_reason_extract custom; finish_reason_path "$.choices[0].finish_reason"; }
```

- `openai`: OpenAI/OpenAI-compatible finish reason (`choices[*].finish_reason`)
- `anthropic`: Anthropic stop reason (`stop_reason`)
- `gemini`: Gemini native finish reason (`candidates[*].finishReason`)
- `custom`: extract from response JSON via `finish_reason_path` (restricted JSONPath subset, see above)

#### finish_reason_path (optional override)

```conf
metrics {
  finish_reason_extract openai;
  finish_reason_path "$.choices[0].finish_reason";
}
```

- Optional escape hatch for providers that expose finish reason in a custom location.

Runtime cost calculation switch is controlled globally by `onr.yaml`:
`pricing.enabled: true|false`.

## 6. Expression context (built-in variables)

In `<expr>` positions, you can reference:

`$channel.base_url`  
The channel base URL (string).

`$channel.key`  
The channel key / token (string).

`$request.model`  
The original request model (string).

`$request.model_mapped`  
The mapped model (string). Defaults to `$request.model` unless modified via `model_map` / `model_map_default`.

Expression forms (v0.1):

- String literal: `"abc"`
- Variable: `$channel.key`
- Concatenation: `concat("Bearer ", $channel.key)`

> v0.1 intentionally keeps expressions minimal; there is no general-purpose scripting language.

---

## 7. Directive reference (nginx style)

Each directive is documented in an nginx-like format:

- `Syntax:` a short signature
- `Default:` default value or `—`
- `Context:` where it can appear (file/provider/defaults/match/request/upstream/response/metrics/etc.)
- `Multiple:` whether multiple occurrences are accepted

### 7.1 file-level directives

#### syntax

```text
Syntax:  syntax "<version>";
Default: —
Context: file
Multiple: no
```

- Mainly used as a version marker; recommended to keep.

#### include

```text
Syntax:  include "<path>";
Default: —
Context: file
Multiple: yes
```

- `<path>` can be relative or absolute; relative paths resolve against the current file directory.
- Includes are expanded as plain text before parsing; max depth 20; cycles are detected.

### 7.2 provider (structure blocks)

> `provider/defaults/match/...` are blocks (not directives), but listed here for quick reference.

#### provider

```text
Syntax:  provider "<name>" { ... }
Default: —
Context: file
Multiple: yes
```

- File name must match `<name>`: `config/providers/<name>.conf`.
- Matching is case-insensitive; use lowercase for `<name>` and file name.

#### defaults

```text
Syntax:  defaults { ... }
Default: —
Context: provider
Multiple: no
```

- Provider defaults; match overlays/overrides on top of defaults.

#### match

```text
Syntax:  match api = "<api-name>" [stream = true|false] { ... }
Default: —
Context: provider
Multiple: yes
```

- First match wins (by appearance order).

### 7.3 upstream_config

#### base_url (assignment)

```text
Syntax:  upstream_config { base_url = "<url>"; }
Default: —
Context: defaults
Multiple: no
```

- `base_url` is required and must be a string literal.
- Runtime channel/key `base_url` can override this default; otherwise this default is used.

### 7.4 auth

#### auth_bearer

```text
Syntax:  auth_bearer;
Default: —
Context: auth
Multiple: yes
```

- Sets `Authorization: Bearer <channel.key>`.
- Token/value is not configurable in `.conf` (fixed to channel key).

#### auth_header_key

```text
Syntax:  auth_header_key <Header-Name>;
Default: —
Context: auth
Multiple: yes
```

- Sets `<Header-Name>: <channel.key>`.
- `<Header-Name>` can be an identifier or a string (use string to support `-`).
- Token/value is not configurable in `.conf` (fixed to channel key).

#### oauth_mode

```text
Syntax:  oauth_mode <mode>;
Default: —
Context: auth
Multiple: yes (last wins)
```

- Allowed `<mode>`: `openai|gemini|qwen|claude|iflow|antigravity|kimi|custom`.
- Enables runtime OAuth token exchange.

#### auth_oauth_bearer

```text
Syntax:  auth_oauth_bearer;
Default: —
Context: auth
Multiple: yes
```

- Sets `Authorization: Bearer <oauth.access_token>`.

#### OAuth auth directives

```text
Syntax:
  oauth_token_url <expr>;
  oauth_client_id <expr>;
  oauth_client_secret <expr>;
  oauth_refresh_token <expr>;
  oauth_scope <expr>;
  oauth_audience <expr>;
  oauth_method <GET|POST>;
  oauth_content_type <form|json>;
  oauth_form <key> <expr>;
  oauth_token_path <jsonpath>;
  oauth_expires_in_path <jsonpath>;
  oauth_token_type_path <jsonpath>;
  oauth_timeout_ms <int>;
  oauth_refresh_skew_sec <int>;
  oauth_fallback_ttl_sec <int>;
Default: mode-specific (builtin) or required fields in custom mode
Context: auth
Multiple:
  - oauth_form: yes
  - others: yes (last wins)
```

- `custom` mode requires:
  - `oauth_token_url`
  - at least one `oauth_form`

### 7.5 request

#### set_header

```text
Syntax:  set_header <Header-Name> <expr>;
Default: —
Context: request
Multiple: yes
```

- Sets/overrides an upstream request header.
- Multiple allowed; executed in order.

#### del_header

```text
Syntax:  del_header <Header-Name>;
Default: —
Context: request
Multiple: yes
```

- Deletes an upstream request header.
- Multiple allowed; executed in order.

#### model_map

```text
Syntax:  model_map <from> <expr>;
Default: —
Context: request
Multiple: yes
```

- Maps `$request.model` to `$request.model_mapped` when `from` matches.
- If `from` repeats, the last one wins.

#### model_map_default

```text
Syntax:  model_map_default <expr>;
Default: $request.model
Context: request
Multiple: yes
```

- Used when no `model_map` matches.

#### json_set

```text
Syntax:  json_set <jsonpath> <value-expr>;
Default: —
Context: request
Multiple: yes
```

- Sets a JSON value on the upstream request payload.
- JSONPath is limited to object paths: `$.a.b.c`.

#### json_set_if_absent

```text
Syntax:  json_set_if_absent <jsonpath> <value-expr>;
Default: —
Context: request/response
Multiple: yes
```

- Sets a JSON value only when the path does not exist.
- If the path already exists (including `null`), the original value is kept.

#### json_del

```text
Syntax:  json_del <jsonpath>;
Default: —
Context: request
Multiple: yes
```

- Deletes a JSON field on the upstream request payload.
- JSONPath is limited to object paths: `$.a.b.c`.

#### json_rename

```text
Syntax:  json_rename <from-jsonpath> <to-jsonpath>;
Default: —
Context: request
Multiple: yes
```

- Renames a JSON field on the upstream request payload.
- JSONPath is limited to object paths: `$.a.b.c`.

### 7.6 upstream

#### set_path

```text
Syntax:  set_path <path-or-expr>;
Default: —
Context: upstream
Multiple: yes
```

- Sets upstream path.

#### set_query

```text
Syntax:  set_query <key> <value-expr>;
Default: —
Context: upstream
Multiple: yes
```

- Sets/overrides a query parameter; multiple allowed (last wins per key).

#### del_query

```text
Syntax:  del_query <key>;
Default: —
Context: upstream
Multiple: yes
```

- Deletes a query parameter; multiple allowed.

### 7.7 response

#### resp_passthrough

```text
Syntax:  resp_passthrough;
Default: —
Context: response
Multiple: yes
```

- Pass through the upstream response (already OpenAI-compatible).

#### resp_map

```text
Syntax:  resp_map <mode>;
Default: —
Context: response
Multiple: yes
```

- Non-streaming response mapping; modes are built-in.

#### sse_parse

```text
Syntax:  sse_parse <mode>;
Default: —
Context: response
Multiple: yes
```

- Streaming SSE mapping; modes are built-in.

### 7.8 error

#### error_map

```text
Syntax:  error_map <mode>;
Default: —
Context: error
Multiple: yes
```

- Allowed modes: `openai`, `common`, `passthrough` (whitelist validated at load time).
- `passthrough`: bypass error normalization and pass upstream error response through to the client.

### 7.9 metrics (usage / finish_reason extraction)

#### usage_extract

```text
Syntax:  usage_extract <mode>;
Default: —
Context: metrics
Multiple: no
```

- Supported: `openai` / `anthropic` / `gemini` / `custom`.

#### finish_reason_extract

```text
Syntax:  finish_reason_extract <mode>;
Default: —
Context: metrics
Multiple: no
```

- Supported: `openai` / `anthropic` / `gemini` / `custom`.

#### finish_reason_path

```text
Syntax:  finish_reason_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Optional override / required for `finish_reason_extract custom;`.
- JSONPath subset: `$.a.b.c` / `$.items[0].x` / `$.items[*].x` (returns first non-empty string with `[*]`).

#### input_tokens

```text
Syntax:  input_tokens = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- Recommended for `usage_extract custom;` only; last one wins.
- `<expr>` is a restricted expression: `+/-`, JSONPath, integer constants; no parentheses, no `*/`, no functions.
- JSONPath subset: `$.a.b.c` / `$.items[0].x` / `$.items[*].x` (sum with `[*]`).
- Missing/non-numeric values are treated as `0`.

#### output_tokens

```text
Syntax:  output_tokens = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- Same rules as `input_tokens`.

#### cache_read_tokens

```text
Syntax:  cache_read_tokens = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- Same rules as `input_tokens`.

#### cache_write_tokens

```text
Syntax:  cache_write_tokens = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- Same rules as `input_tokens`.

#### total_tokens

```text
Syntax:  total_tokens = <expr>;
Default: input_tokens + output_tokens
Context: metrics
Multiple: yes
```

- Same rules as `input_tokens`.
- If not explicitly set, defaults to `input_tokens + output_tokens`.

#### input_tokens_path

```text
Syntax:  input_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Shorthand for `input_tokens = <jsonpath>;` (single JSONPath only; no arithmetic).

#### output_tokens_path

```text
Syntax:  output_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Shorthand for `output_tokens = <jsonpath>;` (single JSONPath only; no arithmetic).

#### cache_read_tokens_path

```text
Syntax:  cache_read_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Shorthand for `cache_read_tokens = <jsonpath>;` (single JSONPath only; no arithmetic).

#### cache_write_tokens_path

```text
Syntax:  cache_write_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Shorthand for `cache_write_tokens = <jsonpath>;` (single JSONPath only; no arithmetic).

### 7.10 balance (upstream balance query)

#### balance_mode

```text
Syntax:  balance_mode <mode>;
Default: —
Context: balance
Multiple: no
```

- Supported: `openai` / `custom`.

#### method

```text
Syntax:  method <GET|POST>;
Default: GET
Context: balance
Multiple: yes
```

#### path

```text
Syntax:  path <path-or-url>;
Default: —
Context: balance
Multiple: yes
```

- Required in `balance_mode custom`.
- Supports absolute URL or path relative to provider `base_url`.

#### balance / used

```text
Syntax:  balance = <expr>;
Syntax:  used = <expr>;
Default: —
Context: balance
Multiple: yes
```

- Restricted expression: JSONPath / number with `+` `-` only.

#### balance_path / used_path

```text
Syntax:  balance_path <jsonpath>;
Syntax:  used_path <jsonpath>;
Default: —
Context: balance
Multiple: yes
```

- `balance_path` is required if `balance` is not set in custom mode.

#### balance_unit

```text
Syntax:  balance_unit <string>;
Default: USD
Context: balance
Multiple: yes
```

- Allowed values: `USD` / `CNY`.

#### set_header / del_header

```text
Syntax:  set_header <Header-Name> <expr>;
Syntax:  del_header <Header-Name>;
Default: —
Context: balance
Multiple: yes
```

#### subscription_path / usage_path

```text
Syntax:  subscription_path <path-or-url>;
Syntax:  usage_path <path-or-url>;
Default: OpenAI dashboard defaults
Context: balance
Multiple: yes
```

- Optional overrides for `balance_mode openai`.

---

## 8. Built-in variables (reference)

This section lists the built-in variables available in v0.1 `<expr>` positions (all are strings).

> Variables are evaluated at runtime; if a variable is empty in the current request context, it expands to an empty string.

### 8.1 `$channel.*`

`$channel.base_url`

Channel base URL (string). Acts as a runtime override source for `upstream_config.base_url`.

`$channel.key`

Channel key/token (string). `auth_bearer;` and `auth_header_key ...;` always use this value.

### 8.2 `$request.*`

`$request.model`

Model name from the client request.

`$request.model_mapped`

Mapped model name. Defaults to `$request.model`; can be modified by `model_map` and `model_map_default`.

### 8.3 Examples

```conf
request {
  set_header "x-upstream-model" $request.model_mapped;
}

auth {
  # Authorization: Bearer <channel.key>
  auth_bearer;
}

upstream {
  # Example: build a path using model_mapped (string concatenation demo)
  set_path concat("/v1/", $request.model_mapped, "/chat/completions");
}
```
