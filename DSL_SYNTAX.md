# Provider Config DSL Syntax (v0.1)

This document describes the provider configuration DSL used by **onr** (open-next-router).
The main DSL entry point is usually `config/onr.conf` in this repository, and that file typically includes `config/providers/*.conf`.
By default ONR loads `config/onr.conf`, and that file can `include "modes/*.conf";` plus `include "providers";` to compose the full DSL config set.
You can still force directory mode via `providers.dir` (config) or `ONR_PROVIDERS_DIR` (env).

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
  - [5.8 models (upstream model list query)](#58-models-upstream-model-list-query)
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
  - [7.10 balance (upstream balance query)](#710-balance-upstream-balance-query)
  - [7.11 models (upstream model list query)](#711-models-upstream-model-list-query)
- [8. Built-in variables (reference)](#8-built-in-variables-reference)

---

## 1. Conventions

- **One provider per file**: `config/providers/<provider>.conf`
- **File name must match provider name**: e.g. `config/providers/openai.conf` must contain `provider "openai" { ... }`
- **Provider name matching is case-insensitive**: everything is normalized to lowercase for matching; use lowercase in configs
- **Semicolon `;` is required**: every statement ends with `;`
- **Only blocks use `{}`**: directives themselves do not use `{}` (nginx-like style)
- **Recommended style: one directive per line** (better diffs and readability)
- Comments are supported: `#`, `//`

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
include relative/or/absolute/path.conf;
include providers;
include providers/*.conf;
```

- Relative paths are resolved against the current file directory.
- A directory include expands to all `*.conf` files in that directory, sorted by file name.
- Glob patterns are supported and expanded in sorted order.
- Quoted include paths are still accepted for compatibility, but unquoted nginx-style paths are preferred.
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
- `match api` must be one of the supported API values below; unknown APIs are rejected at validation/load time.
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
- `images.edits`
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
- `balance { ... }`
- `models { ... }` (**defaults only** in v0.1)

Merge rule (important):

- `defaults` is applied first and the selected `match` is applied afterwards, but **merge behavior is phase-specific rather than whole-block replacement**.
- In `metrics`, `request`, and `balance`, most **scalar fields** use field-level inheritance plus explicit override:
  - a field set in `match` overrides the same field from `defaults`
  - a field omitted in `match` keeps the value from `defaults`
  - example: if `defaults.metrics` sets `finish_reason_extract openai_chat_completions;` and a matched `metrics` block only sets `usage_extract custom;`, the effective config still keeps `finish_reason_extract openai_chat_completions;`
- Header operations in `auth` / `request`, plus list-like directives such as `json_*` and `sse_json_del_if` in `response` / `error`, are usually merged by **appending match directives after defaults**; when later directives conflict on the same target, the later directive typically wins
- Scalar response/error directives such as the main `op` / `mode` can still be overridden by `match`
- `upstream` does not follow a general whole-block merge rule: `defaults.upstream_config` mainly provides the default `base_url`, while concrete routing actions such as `set_path` and query rewrites come from the selected `match.upstream`
- `models` is **defaults only** in v0.1, so there is no match-level override there
- In short: do not treat `defaults` / `match` as full block replacement; use the merge behavior of each phase

Phase boundary rule (important):

- `request` is responsible for constructing the upstream request content, including request body transforms, header operations, and model mapping.
- `upstream` is responsible only for routing the upstream target, such as path, query, and base-url-related target selection.
- Do not place header or body mutation semantics in `upstream`.

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

In v0.1, `request` is the phase that constructs the upstream request content.
It owns request-header operations, lightweight JSON body transforms, and model mapping used by downstream routing expressions.

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

#### pass_header (multiple allowed)

```conf
request {
  pass_header "anthropic-beta";
}
```

- Copies one header from the original client request to the upstream request.
- If the source header is absent, this is a no-op.
- Multiple directives are allowed; executed in order with `set_header` and `del_header`.
- If the same header is passed and later set or deleted, the later directive wins.

#### filter_header_values (multiple allowed)

```conf
request {
  filter_header_values "anthropic-beta" "context-1m-*" "fast-mode-*";
  filter_header_values "x-feature-flags" "exp-*" "debug" separator=";";
}
```

- Filters itemized values inside an upstream request header.
- Syntax: `filter_header_values <header> <pattern>... [separator="<sep>"];`
- Recommended style: keep the pattern list as plain positional arguments. Do not use comma-delimited argument style.
- The default separator is `,`.
- Runtime behavior:
  - Read the current upstream request header value
  - Split by `separator`
  - Apply `strings.TrimSpace` to each item
  - Remove items matching any pattern
  - Delete the whole header if no items remain
  - Otherwise re-join the remaining items
- Output formatting is normalized:
  - If `separator == ","`, items are joined with `", "`
  - Otherwise items are joined with `"<sep> "`, for example `"; "`
- Pattern matching uses simple `*` wildcards; regex is not supported.

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

`upstream` is limited to upstream target routing.
It should be used for path/query/base-url-related selection only, not for request-header or request-body mutation.

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

#### usage_mode (global reusable usage preset)

```conf
usage_mode "shared_openai" {
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}
```

- `usage_mode` is a top-level directive. It defines a reusable global usage preset for the whole providers config set.
- The recommended location for global DSL presets is a separate file under `config/modes/`, such as `config/modes/usage_modes.conf`, then let `config/onr.conf` include `modes/*.conf` before `include providers;`.
- It may appear in a dedicated `.conf` file that contains no `provider {}` block; such files are valid in `config/providers/` and are ignored by provider listing.
- Inside the block, you can use the same usage directives supported by `metrics`: `usage_extract`, `usage_fact`, `*_tokens_path`, and `*_tokens_expr`.
- Another `usage_mode` may be referenced from inside the block via `usage_extract <other_mode>;`, so larger presets can be composed. Recursive references are rejected.
- Names are global within a providers directory or merged providers file. Duplicate `usage_mode` names are validation errors.
- This repository's default `config/modes/usage_modes.conf` defines API-specific presets such as `openai_chat_completions`, `openai_prompt_completion`, `openai_responses`, `anthropic_messages`, `anthropic_messages_stream`, `gemini_generate_content`, and `gemini_generate_content_stream`. Defining the same name in DSL overrides that preset.
- At execution time, `usage_extract <custom_name>;` is resolved to the referenced preset and compiled into the same final usage plan as builtin modes.

#### finish_reason_mode (global reusable finish_reason preset)

```conf
finish_reason_mode "anthropic_messages_stream" {
  finish_reason_path "$.delta.stop_reason";
  finish_reason_path "$.message.stop_reason" fallback=true;
}
```

- `finish_reason_mode` is a top-level directive for reusable global finish reason presets.
- The recommended location is a separate file under `config/modes/`, such as `config/modes/finish_reason_modes.conf`, then let `config/onr.conf` include `modes/*.conf` before `include providers;`.
- It may appear in a dedicated `.conf` file that contains no `provider {}` block.
- Inside the block, you can use the same finish-reason directives supported by `metrics`: `finish_reason_extract` and `finish_reason_path`.
- Another `finish_reason_mode` may be referenced from inside the block via `finish_reason_extract <other_mode>;`. Recursive references are rejected.
- Names are global within a providers directory or merged providers file. Duplicate `finish_reason_mode` names are validation errors.
- This repository's default `config/modes/finish_reason_modes.conf` defines API-specific presets such as `openai_chat_completions`, `openai_completions`, `openai_responses`, `anthropic_messages`, `anthropic_messages_stream`, `gemini_generate_content`, and `gemini_generate_content_stream`. Defining the same name in DSL overrides that preset.

#### models_mode (global reusable models preset)

```conf
models_mode "openai" {
  path "/v2/models";
}
```

- `models_mode` is also available as a top-level directive for reusable global model-list presets.
- The recommended location is a separate file under `config/modes/`, such as `config/modes/models_modes.conf`, then let `config/onr.conf` include `modes/*.conf` before `include providers;`.
- It may appear in a dedicated `.conf` file that contains no `provider {}` block.
- Inside the block, you can use the same directives supported by `models`: `models_mode`, `method`, `path`, `id_path`, `id_regex`, `id_allow_regex`, `set_header`, and `del_header`.
- Another `models_mode` may be referenced from inside the block via `models_mode <other_mode>;`. Recursive references are rejected.
- Names are global within a providers directory or merged providers file. Duplicate `models_mode` names are validation errors.
- This repository's default `config/modes/models_modes.conf` defines `openai` and `gemini` as global `models_mode` presets. Defining the same name in DSL overrides that preset.

#### balance_mode (global reusable balance preset)

```conf
balance_mode "openai" {
  usage_path "/v9/dashboard/billing/usage";
}
```

- `balance_mode` is also available as a top-level directive for reusable global balance presets.
- The recommended location is a separate file under `config/modes/`, such as `config/modes/balance_modes.conf`, then let `config/onr.conf` include `modes/*.conf` before `include providers;`.
- It may appear in a dedicated `.conf` file that contains no `provider {}` block.
- Inside the block, you can use the same directives supported by `balance`: `balance_mode`, `method`, `path`, `balance_path`, `balance_expr`, `used_path`, `used_expr`, `balance_unit`, `subscription_path`, `usage_path`, `set_header`, and `del_header`.
- Another `balance_mode` may be referenced from inside the block via `balance_mode <other_mode>;`. Recursive references are rejected.
- Names are global within a providers directory or merged providers file. Duplicate `balance_mode` names are validation errors.
- This repository's default `config/modes/balance_modes.conf` defines `openai` as a global `balance_mode` preset. Defining the same name in DSL overrides that preset.

#### usage_extract

```conf
usage_mode "shared_openai" {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}

provider "openai" {
  defaults {
    metrics { usage_extract shared_openai; }
  }
}

metrics { usage_extract openai_chat_completions; }
metrics { usage_extract anthropic_messages; }
metrics { usage_extract gemini_generate_content; }
metrics { usage_extract custom; }
```

- `custom`: extract from response JSON via a restricted JSONPath subset and optional arithmetic (see below)
- any other mode name: a user-defined global `usage_mode`
- The default repository config now focuses on API/path-specific presets such as `openai_chat_completions`, `openai_prompt_completion`, `openai_responses`, `anthropic_messages`, `anthropic_messages_stream`, `gemini_generate_content`, and `gemini_generate_content_stream`.
- Generic names such as `openai`, `anthropic`, and `gemini` are no longer special builtin `usage_extract` modes. If you want those names, define them explicitly as global `usage_mode` presets.
- user-defined `usage_mode` names are resolved before execution, then compiled into the same normalized fact-based plan.
- Inside `metrics`, if you declare `usage_fact`, `*_tokens_path`, or `*_tokens_expr` without `usage_extract`, ONR treats the block as `usage_extract custom;`.
- For code-side introspection, it helps to distinguish three layers:
  - declared: explicit user-authored `usage_fact` rules
  - compiled: the final normalized usage plan that actually executes

Migration sketches for the legacy generic provider modes:

- `openai`:

```conf
metrics {
  usage_extract custom;

  usage_fact input token path="$.usage.prompt_tokens";
  usage_fact input token path="$.usage.input_tokens" fallback=true;

  usage_fact output token path="$.usage.completion_tokens";
  usage_fact output token path="$.usage.output_tokens" fallback=true;

  usage_fact cache_read token path="$.usage.prompt_tokens_details.cached_tokens";
  usage_fact cache_read token path="$.usage.input_tokens_details.cached_tokens" fallback=true;
  usage_fact cache_read token path="$.usage.cached_tokens" fallback=true;
}
```

- `anthropic`:

```conf
metrics {
  usage_fact input token path="$.usage.input_tokens";
  usage_fact input token path="$.usage.cache_read_input_tokens";
  usage_fact input token path="$.usage.cache_creation_input_tokens";
  usage_fact output token path="$.usage.output_tokens";
  usage_fact cache_read token path="$.usage.cache_read_input_tokens";

  usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
  usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h";
  usage_fact cache_write token path="$.usage.cache_creation_input_tokens" fallback=true;
}
```

- `gemini`:

```conf
metrics {
  usage_extract custom;

  usage_fact input token path='$.usageMetadata.promptTokensDetails[?(@.modality=="TEXT")].tokenCount';
  usage_fact input token path="$.usageMetadata.promptTokenCount" fallback=true;
  usage_fact image.input token path='$.usageMetadata.promptTokensDetails[?(@.modality=="IMAGE")].tokenCount';
  usage_fact video.input token path='$.usageMetadata.promptTokensDetails[?(@.modality=="VIDEO")].tokenCount';
  usage_fact audio.input token path='$.usageMetadata.promptTokensDetails[?(@.modality=="AUDIO")].tokenCount';

  usage_fact output token path="$.usageMetadata.candidatesTokenCount";
  usage_fact output token path="$.usageMetadata.thoughtsTokenCount";
}
```

Notes:

- `gemini`: the current default preset behavior can be fully replaced by `custom` configuration; `input token` usually prefers the `TEXT` modality and falls back to `promptTokenCount`, while multimodal details can be emitted as `image.input/audio.input/video.input token` facts.
- `anthropic`: ONR now treats `input` as the effective input size, so `cache_read_input_tokens` and `cache_creation_input_tokens` should also be included in `input`.
- `openai`: the example above only covers core token/cache extraction. Image/audio/tool supplemental facts still need extra explicit `usage_fact` rules in a custom-first setup.
- Gemini output tokens intentionally include both `candidatesTokenCount` and `thoughtsTokenCount`; you can express that either by multiple same-dimension `usage_fact` rules that sum together, or more explicitly with `output_tokens_expr = $.usageMetadata.candidatesTokenCount + $.usageMetadata.thoughtsTokenCount;`.
- `total_tokens` is derived from `input + output` by default; in most cases you should avoid setting `total_tokens_expr` explicitly, because that introduces a second total fact source.
- Gemini native usage fields are handled in camelCase: `usageMetadata.promptTokenCount`, `candidatesTokenCount`, `thoughtsTokenCount`, and `totalTokenCount`.

Anthropic streaming `custom` sketch:

```conf
metrics {
  usage_fact input token path="$.message.usage.input_tokens" event="message_start";
  usage_fact input token path="$.message.usage.cache_read_input_tokens" event="message_start";
  usage_fact input token path="$.message.usage.cache_creation_input_tokens" event="message_start";
  usage_fact input token path="$.usage.cache_read_input_tokens" event="message_delta";
  usage_fact input token path="$.usage.cache_creation_input_tokens" event="message_delta";

  usage_fact output token path="$.usage.output_tokens" event="message_delta";

  usage_fact cache_read token path="$.message.usage.cache_read_input_tokens" event="message_start";
  usage_fact cache_read token path="$.usage.cache_read_input_tokens" event="message_delta";

  usage_fact cache_write token path="$.message.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m" event="message_start";
  usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m" event="message_delta";
  usage_fact cache_write token path="$.message.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h" event="message_start";
  usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h" event="message_delta";
  usage_fact cache_write token path="$.message.usage.cache_creation_input_tokens" fallback=true event="message_start";
  usage_fact cache_write token path="$.usage.cache_creation_input_tokens" fallback=true event="message_delta";
}
```

- This covers the main Anthropic stream usage event shapes.
- `event="..."` gates a `usage_fact` rule by SSE `event:` name. It only applies to stream extraction.
- Compared with the old generic Anthropic mode, the main difference is ergonomics: the custom form is longer and easier to misconfigure.

OpenAI supplemental facts `custom` sketches:

```conf
# responses: web search tool calls
metrics {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
  usage_fact server_tool.web_search call count_path="$.output[*]" type="web_search_call" status="completed";
}

# images.generations
metrics {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
  usage_fact image.generate image count_path="$.data[*]";
}

# audio.speech
metrics {
  usage_extract custom;
  usage_fact audio.tts second source=derived path="$.audio_duration_seconds";
}
```

- The old generic OpenAI behavior can still be recreated with explicit `usage_fact` rules.
- The difference is scope: these supplemental facts are API-specific, so there is no single short custom snippet that fully replaces the builtin mode across every OpenAI-compatible endpoint.

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

#### usage_fact (recommended new form)

```conf
metrics {
  usage_extract custom;

  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
  usage_fact cache_read token path="$.usage.cache_read_input_tokens";

  usage_fact cache_write token path="$.usage.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
  usage_fact cache_write token path="$.usage.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h";
  usage_fact cache_write token path="$.usage.cache_creation_input_tokens" fallback=true;
}
```

- `usage_fact` normalizes measurable usage into canonical facts.
- In `metrics`, declaring `usage_fact` rules without `usage_extract` is equivalent to `usage_extract custom;`.
- Named presets are compiled into the same internal fact-based execution plan and may still be supplemented with extra `usage_fact` rules.
- Supported primitives: `path`, `count_path`, `sum_path`, `expr`.
- `count_path` can be combined with `type` and `status`.
- `event="..."` optionally restricts a `usage_fact` rule to SSE events such as `message_start` or `message_delta`.
- `attr.ttl` distinguishes Anthropic cache-write tiers.
- Multiple `usage_fact` rules may share the same `dimension + unit`; all matched non-fallback rules are summed in declaration order.
- `fallback=true` applies only when no more specific fact exists for the same `dimension + unit`.
- `source` defaults to `response` and currently supports `response`, `request`, and `derived`.
- `dimension` is a flat registry key; `.` is part of the name and does not imply nested structure.
- Supported `dimension` values and `dimension + unit` pairs are fixed by registry; see the later [`usage_fact`](#usage_fact) directive reference for the complete list.
- `path`, `count_path`, `sum_path`, and `expr` may use either double-quoted or single-quoted strings.
- For filter JSONPath, single-quoted DSL strings are usually easier to read because inner double quotes do not need escaping.
- The repository's API-specific OpenAI presets commonly model these canonical facts:
  - `openai_images_generations -> image.generate image`
  - `openai_images_edits -> image.edit image`
  - `openai_audio_transcriptions -> audio.stt second`
  - `openai_audio_translations -> audio.translate second`
  - `openai_audio_speech -> audio.tts second`
  - `openai_responses -> server_tool.web_search call`

Examples:

```conf
metrics {
  usage_extract openai_responses;

  usage_fact server_tool.web_search call count_path="$.output[*]" type="web_search_call" status="completed";
}
```

```conf
metrics {
  usage_extract openai_images_generations;

  usage_fact image.generate image count_path="$.data[*]";
  usage_fact image.generate image source=request expr="$.n" fallback=true;
}
```

```conf
metrics {
  usage_extract openai_audio_speech;

  usage_fact audio.tts second source=derived path="$.audio.tts.seconds";
}
```

```conf
metrics {
  usage_extract custom;

  usage_fact audio.tts second path='$.usage.details[?(@.modality=="AUDIO")].seconds';
}
```

#### finish_reason_extract

```conf
metrics { finish_reason_extract openai_chat_completions; }
metrics { finish_reason_extract anthropic_messages; }
metrics { finish_reason_extract gemini_generate_content; }
metrics { finish_reason_path "$.choices[0].finish_reason"; }
```

- `custom`: extract from response JSON via `finish_reason_path` (restricted JSONPath subset, see above)
- any other mode name: a user-defined global `finish_reason_mode`
- A `finish_reason_mode` block may omit `finish_reason_extract` when it only declares `finish_reason_path` rules, in which case ONR treats it as `custom`.
- Inside `metrics`, declaring `finish_reason_path` without `finish_reason_extract` is equivalent to `finish_reason_extract custom;`.
- The default repository config now focuses on API/path-specific presets such as `openai_chat_completions`, `openai_completions`, `openai_responses`, `anthropic_messages`, `anthropic_messages_stream`, `gemini_generate_content`, and `gemini_generate_content_stream`.
- Generic names such as `openai`, `anthropic`, and `gemini` are no longer special builtin `finish_reason_extract` modes. If you want those names, define them explicitly as global `finish_reason_mode` presets.

Path-specific preset mappings:

- `openai`
  - `chat.completions` / `completions`: checks `$.choices[*].finish_reason` and returns the first non-empty value
  - `responses` non-stream: checks `$.incomplete_details.reason`
  - `responses` stream SSE envelope: checks `$.response.incomplete_details.reason`
- `anthropic`
  - checks `$.stop_reason`
  - then falls back to `$.delta.stop_reason`
  - then falls back to `$.message.stop_reason`
- `gemini`
  - checks `$.candidates[*].finishReason`
  - then falls back to `$.candidates[*].finish_reason`

Equivalent `custom` examples:

- OpenAI Chat/Completions equivalent:

```conf
metrics {
  finish_reason_path "$.choices[0].finish_reason";
}
```

- OpenAI Responses raw reason extraction:

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.incomplete_details.reason";
}
```

  This is equivalent to builtin `openai` for non-stream `responses` payloads.

- OpenAI Responses SSE envelope equivalent:

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.incomplete_details.reason";
  finish_reason_path "$.response.incomplete_details.reason" fallback=true;
}
```

  This reproduces the current builtin `openai` coverage across non-stream and streamed `response.incomplete` payloads.

- Anthropic non-stream equivalent:

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.stop_reason";
}
```

- Anthropic stream delta equivalent:

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.delta.stop_reason";
}
```

- Anthropic stream message fallback equivalent:

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.message.stop_reason";
}
```

- Gemini equivalent:

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.candidates[0].finishReason";
}
```

  If a provider emits snake_case instead, use `$.candidates[0].finish_reason`.

#### finish_reason_path (optional override)

```conf
metrics {
  finish_reason_extract openai_chat_completions;
  finish_reason_path "$.choices[0].finish_reason";
}
```

- Optional escape hatch for providers that expose finish reason in a custom location.
- Multiple `finish_reason_path` directives are allowed.
- `fallback=true` means this path is only attempted when no earlier non-fallback path produced a non-empty finish reason.

Example:

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.delta.stop_reason";
  finish_reason_path "$.message.stop_reason" fallback=true;
}
```

Runtime cost calculation switch is controlled globally by `onr.yaml`:
`pricing.enabled: true|false`.

### 5.8 models (upstream model list query)

`models` declares how a provider exposes its upstream model listing API and how model IDs are extracted.
This block is supported in `defaults` (not `match`) in v0.1.

```conf
models {
  models_mode openai;             # openai|gemini|custom
}
```

Custom example:

```conf
models {
  models_mode custom;
  method GET;
  path "/v1beta/models";
  id_path "$.models[*].name";
  id_regex "^models/(.+)$";
  id_allow_regex "^gemini-";
}
```

Semantics:

- `models_mode openai` default path/extract:
  - `path /v1/models`
  - `id_path $.data[*].id`
- `models_mode gemini` default path/extract:
  - `path /v1beta/models`
  - `id_path $.models[*].name`
  - `id_regex ^models/(.+)$`
- `models_mode custom` requires explicit `path` and at least one `id_path`.
- `id_path` is repeatable; extracted IDs are unioned and deduplicated.
- `id_regex` rewrites each extracted value:
  - if regex contains a capture group, group 1 is used as final ID;
  - otherwise full match is used;
  - non-matching values are dropped.
- `id_allow_regex` is an optional final whitelist filter.

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

#### usage_mode / finish_reason_mode / models_mode

```text
Syntax:  usage_mode "<name>" { ... }
Syntax:  finish_reason_mode "<name>" { ... }
Syntax:  models_mode "<name>" { ... }
Default: —
Context: file
Multiple: yes
```

- Top-level reusable preset blocks for `metrics` and `models`.
- Recommended locations: `config/modes/usage_modes.conf`, `config/modes/finish_reason_modes.conf`, and `config/modes/models_modes.conf`.

#### balance_mode

```text
Syntax:  balance_mode "<name>" { ... }
Default: —
Context: file
Multiple: yes
```

- Top-level reusable preset blocks for `balance`.
- Recommended location: `config/modes/balance_modes.conf`.

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

#### pass_header

```text
Syntax:  pass_header <Header-Name>;
Default: —
Context: request
Multiple: yes
```

- Copies one header from the original client request to the upstream request.
- If the source header is absent, this is a no-op.

#### filter_header_values

```text
Syntax:  filter_header_values <Header-Name> <pattern>... [separator="<sep>"];
Default: separator=","
Context: request
Multiple: yes
```

- Filters itemized values from one upstream request header.
- Split by `separator`, trim each item, remove items matching any pattern, then re-join survivors.
- If nothing remains after filtering, the whole header is deleted.
- Join formatting is normalized to `", "` for comma and `"<sep> "` for any other separator.

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
- Builtin semantics:
  - `openai`:
    - `chat.completions` / `completions`: `choices[*].finish_reason`
    - `responses` non-stream: `incomplete_details.reason`
    - `responses` stream: `response.incomplete_details.reason`
  - `anthropic`: `stop_reason` -> `delta.stop_reason` -> `message.stop_reason`
  - `gemini`: `candidates[*].finishReason` -> `candidates[*].finish_reason`
- `custom` supports ordered fallback via multiple `finish_reason_path` directives and can fully replicate the builtin extraction order when the provider only needs path-based lookup.

#### finish_reason_path

```text
Syntax:  finish_reason_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Optional override / required for `finish_reason_extract custom;`.
- Supports optional `fallback=true|false` metadata after the path.
- JSONPath subset: `$.a.b.c` / `$.items[0].x` / `$.items[*].x` (returns first non-empty string with `[*]`).

#### input_tokens_expr

```text
Syntax:  input_tokens_expr = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- Recommended for `usage_extract custom;` only; last one wins.
- Compatibility-layer form: it is compiled into equivalent internal fact-based rules before execution.
- `<expr>` is a restricted expression: `+/-`, JSONPath, integer constants; no parentheses, no `*/`, no functions.
- JSONPath subset: `$.a.b.c` / `$.items[0].x` / `$.items[*].x` (sum with `[*]`).
- Missing/non-numeric values are treated as `0`.

#### output_tokens_expr

```text
Syntax:  output_tokens_expr = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- Same rules as `input_tokens_expr`.

#### cache_read_tokens_expr

```text
Syntax:  cache_read_tokens_expr = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- Same rules as `input_tokens_expr`.

#### cache_write_tokens_expr

```text
Syntax:  cache_write_tokens_expr = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- Same rules as `input_tokens_expr`.

#### total_tokens_expr

```text
Syntax:  total_tokens_expr = <expr>;
Default: input_tokens_expr + output_tokens_expr
Context: metrics
Multiple: yes
```

- Same rules as `input_tokens_expr`.
- If not explicitly set, defaults to `input_tokens_expr + output_tokens_expr`.
- Not recommended: setting `total_tokens_expr` introduces an independent total fact source that can diverge from the total derived from `input/output`.
- Also a compatibility-layer form: it is compiled into the unified internal usage plan before execution.

#### input_tokens_path

```text
Syntax:  input_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Shorthand for `input_tokens_expr = <jsonpath>;` (single JSONPath only; no arithmetic).
- Compatibility-layer shorthand: it is compiled into equivalent internal fact-based rules before execution.

#### output_tokens_path

```text
Syntax:  output_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Shorthand for `output_tokens_expr = <jsonpath>;` (single JSONPath only; no arithmetic).
- Compatibility-layer shorthand: it is compiled into equivalent internal fact-based rules before execution.

#### cache_read_tokens_path

```text
Syntax:  cache_read_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Shorthand for `cache_read_tokens_expr = <jsonpath>;` (single JSONPath only; no arithmetic).
- Compatibility-layer shorthand: it is compiled into equivalent internal fact-based rules before execution.

#### cache_write_tokens_path

```text
Syntax:  cache_write_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- Shorthand for `cache_write_tokens_expr = <jsonpath>;` (single JSONPath only; no arithmetic).
- Compatibility-layer shorthand: it is compiled into equivalent internal fact-based rules before execution.

#### usage_fact

```text
Syntax:  usage_fact <dimension> <unit> path|count_path|sum_path|expr ...;
Default: —
Context: metrics
Multiple: yes
```

- Recommended with `usage_extract custom;`, but builtin modes may also supplement canonical facts.
- The registry is intentionally fixed; arbitrary dimensions are not accepted.
- Supports `path`, `count_path`, `sum_path`, and `expr`.
- `count_path` may be combined with `type` / `status`.
- Constant attributes such as `attr.ttl="5m"` are supported.
- Multiple rules may share the same `dimension + unit`; all matched non-fallback rules are summed.
- `fallback=true` means the fact applies only when no more specific fact exists for the same `dimension + unit`.
- `source` defaults to `response` and currently supports `response`, `request`, and `derived`.
- `dimension` is a flat string key; `.` is part of the name and does not imply nesting.
- Supported `dimension` values:
  - `input`
  - `output`
  - `cache_read`
  - `cache_write`
  - `server_tool.web_search`
  - `server_tool.file_search`
  - `image.generate`
  - `image.edit`
  - `image.variation`
  - `audio.tts`
  - `audio.stt`
  - `audio.translate`
- Supported `dimension + unit` pairs are:
  - `input token`
  - `output token`
  - `cache_read token`
  - `cache_write token`
  - `server_tool.web_search call`
  - `server_tool.file_search call`
  - `image.generate image`
  - `image.edit image`
  - `image.variation image`
  - `audio.tts second`
  - `audio.stt second`
  - `audio.translate second`

### 7.10 balance (upstream balance query)

#### balance_mode

```text
Syntax:  balance_mode <mode>;
Default: —
Context: balance
Multiple: no
```

- Supported: `openai` / `custom`.
- Any other mode name is resolved as a global top-level `balance_mode` preset before execution.

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

#### balance_expr / used_expr

```text
Syntax:  balance_expr = <expr>;
Syntax:  used_expr = <expr>;
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

- `balance_path` is required if `balance_expr` is not set in custom mode.

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

### 7.11 models (upstream model list query)

#### models_mode

```text
Syntax:  models_mode <mode>;
Default: —
Context: models
Multiple: no
```

- Supported: `openai` / `gemini` / `custom`.
- Any other mode name is resolved as a global top-level `models_mode` preset before execution.

#### method

```text
Syntax:  method <GET|POST>;
Default: GET
Context: models
Multiple: yes
```

#### path

```text
Syntax:  path <path-or-url>;
Default: mode-dependent
Context: models
Multiple: yes
```

- `models_mode openai`: default `/v1/models`
- `models_mode gemini`: default `/v1beta/models`
- `models_mode custom`: required

#### id_path

```text
Syntax:  id_path <jsonpath>;
Default: mode-dependent
Context: models
Multiple: yes
```

- `models_mode openai`: default `$.data[*].id`
- `models_mode gemini`: default `$.models[*].name`
- `models_mode custom`: at least one `id_path` is required

#### id_regex / id_allow_regex

```text
Syntax:  id_regex <regex>;
Syntax:  id_allow_regex <regex>;
Default: —
Context: models
Multiple: yes
```

- `id_regex` is a rewrite step (capture group 1 preferred).
- `id_allow_regex` is a post-rewrite whitelist filter.

#### set_header / del_header

```text
Syntax:  set_header <Header-Name> <expr>;
Syntax:  del_header <Header-Name>;
Default: —
Context: models
Multiple: yes
```

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
