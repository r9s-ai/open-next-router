# Provider 配置 DSL 语法（v0.1）

本文档描述 **onr**（open-next-router）使用的 Provider 配置 DSL。
Provider 配置文件位于本仓库 `config/providers/*.conf`，在启动时（以及 reload 时）加载。
目录可通过配置项 `providers.dir` 或环境变量 `ONR_PROVIDERS_DIR` 自定义。

## 目录

- [1. 基本约定](#1-基本约定)
- [2. include（复用片段）](#2-include复用片段)
- [3. 顶层结构](#3-顶层结构)
  - [3.1 syntax](#31-syntax)
  - [3.2 provider](#32-provider)
- [4. match 规则（选择逻辑）](#4-match-规则选择逻辑)
- [5. phase/block 列表（defaults 与 match 中都可写）](#5-phaseblock-列表defaults-与-match-中都可写)
  - [5.1 upstream_config（上游默认配置）](#51-upstream_config上游默认配置)
  - [5.2 auth（鉴权）](#52-auth鉴权)
  - [5.3 request（请求处理）](#53-request请求处理)
  - [5.4 upstream（路径与 query 操作）](#54-upstream路径与-query-操作)
  - [5.5 response（响应处理）](#55-response响应处理)
  - [5.6 error（错误归一化）](#56-error错误归一化)
  - [5.7 metrics（用量提取--usage）](#57-metrics用量提取--usage)
- [6. 可用的 Context（表达式变量）](#6-可用的-context表达式变量)
- [7. 指令参考（nginx 风格）](#7-指令参考nginx-风格)
  - [7.1 顶层指令](#71-顶层指令)
  - [7.2 provider（结构块）](#72-provider结构块)
  - [7.3 upstream_config（默认上游配置）](#73-upstream_config默认上游配置)
  - [7.4 auth（鉴权）](#74-auth鉴权)
  - [7.5 request（请求处理）](#75-request请求处理)
  - [7.6 upstream（路由）](#76-upstream路由)
  - [7.7 response（响应）](#77-response响应)
  - [7.8 error（错误归一化）](#78-error错误归一化)
  - [7.9 metrics（用量提取--usage）](#79-metrics用量提取--usage)
- [8. 内置变量参考](#8-内置变量参考)

---

## 1. 基本约定

- **一个 provider 一个文件**：`config/providers/<provider>.conf`
- **文件名必须与 provider 名一致**：例如 `config/providers/openai.conf` 内必须是 `provider "openai" { ... }`
- **provider 名匹配不区分大小写**：匹配时会统一转小写；配置中建议使用小写
- **分号 `;` 必须写**：所有语句以 `;` 结束
- **只有 block 才使用花括号 `{}`**：指令本身不使用 `{}`（nginx 风格）
- **推荐书写风格：每行一条指令**（便于 diff 与可读性）
- 支持注释：`#`、`//`、`/* ... */`

推荐格式示例：

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

## 2. include（复用片段）

语法：

```conf
include "relative/or/absolute/path.conf";
```

- 相对路径以「当前文件所在目录」为基准
- 最大递归深度：20
- 检测循环 include：出现循环会报错并跳过该 provider 文件加载

## 3. 顶层结构

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

目前仅用于占位与版本标识（`syntax "next-router/0.1";`），解析器不会做强校验，但建议保留。

### 3.2 provider

语法：

```conf
provider "<name>" { ... }
```

## 4. match 规则（选择逻辑）

语法（支持的条件）：

```conf
match api = "<api-name>" stream = true { ... }
match api = "<api-name>" stream = false { ... }
match api = "<api-name>" { ... }
```

- 当前仅解析 `api` 与 `stream`
- **只会命中第一条匹配的 match**（按文件中出现顺序）
  - 建议：更具体的规则放前面，更泛的规则放后面
- 当某个 provider 被选中（DSL enabled）但 **没有任何 match 命中** 时，请求会直接被拒绝（HTTP 400），避免静默回退导致行为不透明。

当前支持的 `api`（与 OpenAI 风格端点对齐）：

- `completions`
- `chat.completions`
- `responses`
- `claude.messages`
- `embeddings`
- `gemini.generateContent`（Gemini 原生：`POST /v1beta/models/{model}:generateContent`）
- `gemini.streamGenerateContent`（Gemini 原生：`POST /v1beta/models/{model}:streamGenerateContent?alt=sse`）
- `images.generations`
- `audio.speech`
- `audio.transcriptions`
- `audio.translations`

## 5. phase/block 列表（defaults 与 match 中都可写）

可用 block：

- `upstream_config { ... }`
- `auth { ... }`
- `request { ... }`
- `upstream { ... }`
- `response { ... }`
- `error { ... }`
- `metrics { ... }`

合并规则（非常重要）：

- `defaults` 先应用，再应用命中的 `match`（因此 match 中的同名设置通常可以覆盖 defaults）

### 5.1 upstream_config（上游默认配置）

语法：

```conf
upstream_config {
  base_url = "https://api.example.com";
}
```

说明：

- `base_url` 是**必填**，同时也是**默认值**
- 若渠道（DB）配置了 `base_url`，则优先使用渠道配置；只有当渠道 `base_url` 为空时，才使用这里的默认值

### 5.2 auth（鉴权）

用途：声明鉴权头的“形式”。**token/value 不需要写在 conf 中**，统一取渠道的 `key`（等价于 `$channel.key`）。

#### auth_bearer

```conf
auth { auth_bearer; }
```

效果：`Authorization: Bearer <channel.key>`

#### auth_header_key

```conf
auth { auth_header_key "x-api-key"; }
auth { auth_header_key api_key; }
```

效果：`<Header-Name>: <channel.key>`

#### OAuth 换 token + Bearer 注入

```conf
auth {
  oauth_mode openai;              # openai|gemini|qwen|claude|iflow|antigravity|kimi|custom
  oauth_refresh_token $channel.key;
  auth_oauth_bearer;
}
```

- `oauth_mode`：开启运行时 OAuth token 交换。
- `auth_oauth_bearer;`：注入 `Authorization: Bearer <oauth.access_token>`。
- 内置 mode 会提供默认 token endpoint 与请求格式。
- `custom` 模式必须显式配置 `oauth_token_url`，并至少有一条 `oauth_form`。
- 可选覆盖项：
  - `oauth_token_url <expr>;`
  - `oauth_client_id <expr>;`
  - `oauth_client_secret <expr>;`
  - `oauth_refresh_token <expr>;`（默认 `$channel.key`）
  - `oauth_scope <expr>;`
  - `oauth_audience <expr>;`
  - `oauth_method <GET|POST>;`
  - `oauth_content_type <form|json>;`
  - `oauth_form <key> <expr>;`（可多条）
  - `oauth_token_path <jsonpath>;`（默认 `$.access_token`）
  - `oauth_expires_in_path <jsonpath>;`（默认 `$.expires_in`）
  - `oauth_token_type_path <jsonpath>;`（默认 `$.token_type`）
  - `oauth_timeout_ms <int>;`（默认 `5000`）
  - `oauth_refresh_skew_sec <int>;`（默认 `300`）
  - `oauth_fallback_ttl_sec <int>;`（默认 `1800`）

### 5.3 request（请求处理）

> v0.1：`request` phase 同时承担“请求头操作”与“请求体（JSON）变换”的能力。

#### set_header（可多条）

```conf
request {
  set_header "x-trace-id" "trace-123";
  set_header "x-foo" concat("a-", $request.model_mapped);
}
```

说明：

- 支持多条；按顺序执行
- 同一 header 多次 `set_header`：后写的覆盖先写的

#### del_header（可多条）

```conf
request {
  del_header "x-remove-me";
}
```

说明：

- 支持多条；按顺序执行
- `defaults` 的操作在前，`match` 的操作追加在后，因此 match 更容易覆盖 defaults（例如 defaults 先 set，match 再 del）

#### model_map（可多条）

```conf
request {
  model_map "gpt-4o-mini" "gpt4o-mini-prod";
  model_map "gpt-4o-mini" $request.model;
}
```

说明：

- 用途：把 `$request.model` 映射为 `$request.model_mapped`，供 `set_path/set_query/set_header` 等表达式使用（例如 Azure deployments 路径拼接）
- 支持多条；按“from 模型名”精确匹配
- 同一 from 多次出现：后写的覆盖先写的

#### model_map_default（可多条，最后一条生效）

```conf
request {
  model_map_default $request.model;
}
```

说明：

- 当没有任何 `model_map <from> ...;` 命中时，使用该默认表达式作为 `$request.model_mapped`
- 如未配置该指令：`$request.model_mapped` 默认等于 `$request.model`

#### json_set / json_del / json_rename（可多条）

```conf
request {
  json_set "$.stream" true;
  json_set "$.user" "alice";
  json_rename "$.max_tokens" "$.max_completion_tokens";
  json_del "$.tools";
}
```

说明：

- 用途：对“已生成的上游请求 JSON”做轻量变换（在旧 adaptor 的 `ConvertRequest` 之后执行）
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）
- `json_set` 的值表达式支持：`true/false/null`、整数、字符串字面量、变量/concat

#### req_map

```conf
request { req_map <mode>; }
```

内置请求映射（非流式 JSON 变换）。若写多条指令，**最后一条生效**。

v0.1 内置：

- `openai_chat_to_openai_responses`：OpenAI-compatible `chat.completions` 请求 JSON → OpenAI `/responses` 请求 JSON
- `anthropic_to_openai_chat`：Anthropic `/v1/messages` 请求 JSON → OpenAI `chat.completions` 请求 JSON
- `gemini_to_openai_chat`：Gemini `generateContent` 请求 JSON → OpenAI `chat.completions` 请求 JSON
- `openai_chat_to_gemini_generate_content`：OpenAI `chat.completions` 请求 JSON → Gemini `generateContent` 请求 JSON
- `openai_chat_to_anthropic_messages`：OpenAI `chat.completions` 请求 JSON → Anthropic `/v1/messages` 请求 JSON

### 5.4 upstream（路径与 query 操作）

#### set_path

```conf
upstream {
  set_path "/v1/chat/completions";
}
```

说明：

- 设置上游请求路径（会覆盖原路径）

#### set_query（可多条）

```conf
upstream {
  set_query "api-version" "2024-10-01";
  set_query stream "true";
  # 内置变量不要加引号（字符串字面量内不会做变量展开）：
  #   ✅ set_query key $channel.key;
  #   ❌ set_query key "$channel.key";  # 会变成普通字符串 "$channel.key"
}
```

说明：

- 支持多条
- 同一 key 多次 `set_query`：后写的覆盖先写的
- **重要：**内置变量（例如 `$channel.key`、`$request.model_mapped`）仅在作为“裸表达式”使用时才会展开；如果放进双引号里，会被当作普通字符串字面量，不会展开。

#### del_query（可多条）

```conf
upstream {
  del_query "foo";
  del_query bar;
}
```

说明：

- 支持多条
- 执行顺序：先执行所有 `del_query`，再执行所有 `set_query`

### 5.5 response（响应处理）

该 phase 目前语义是“选择一个响应策略”。如写多条，**最后一条生效**。

#### resp_passthrough

```conf
response { resp_passthrough; }
```

用途：上游响应已经是 OpenAI-compatible，直接透传。

#### resp_map

```conf
response { resp_map <mode>; }
```

用途：非流式响应映射（例如把某供应商 JSON 映射为 OpenAI chat.completions）。

#### sse_parse

```conf
response { sse_parse <mode>; }
```

用途：流式 SSE 映射（例如把某供应商 SSE 映射为 OpenAI stream chunks）。

`mode` 的可用值取决于内置实现；当前 v0.1 已内置：

- `anthropic_to_openai_chat`（`resp_map`）：Anthropic `/v1/messages` JSON → OpenAI `chat.completions` JSON
- `anthropic_to_openai_chunks`（`sse_parse`）：Anthropic `/v1/messages` SSE → OpenAI stream chunks
- `openai_to_anthropic_messages`（`resp_map`）：OpenAI-compatible `chat.completions` JSON → Anthropic `/v1/messages` JSON
- `openai_to_anthropic_chunks`（`sse_parse`）：OpenAI-compatible `chat.completions` SSE → Anthropic `/v1/messages` SSE
- `openai_to_gemini_chat` / `openai_to_gemini_generate_content`（`resp_map`）：OpenAI-compatible `chat.completions` JSON → Gemini `generateContent` JSON
- `gemini_to_openai_chat`（`resp_map`）：Gemini `generateContent` JSON → OpenAI `chat.completions` JSON
- `openai_to_gemini_chunks`（`sse_parse`）：OpenAI-compatible `chat.completions` SSE → Gemini SSE
- `gemini_to_openai_chat_chunks`（`sse_parse`）：Gemini SSE → OpenAI `chat.completions` SSE chunks
- `openai_responses_to_openai_chat`（`resp_map`）：OpenAI/Azure `/responses` JSON → OpenAI `chat.completions` JSON
- `openai_responses_to_openai_chat_chunks`（`sse_parse`）：OpenAI/Azure `/responses` SSE → OpenAI `chat.completions` SSE chunks

#### json_del / json_set / json_rename（响应 JSON 操作）

这些指令会对下游响应体做**尽力而为（best-effort）**的 JSON 变换：

```conf
response {
  json_del "$.usage";
  json_set "$.foo" "bar";
  json_rename "$.a" "$.b";
}
```

语义：

- 非流式 JSON：对整个响应体的 JSON **对象**做变换
- 流式 SSE（`text/event-stream`）：对每个 SSE event 拼接后的 `data:` JSON **对象**做变换
- 非 JSON / 非对象负载：保持原样透传
- 执行顺序：按配置块内出现顺序依次执行

限制（v0.1）：

- JSONPath 仅支持对象路径 `$.a.b.c`（不支持数组下标）

#### sse_json_del_if（SSE 条件删除）

```conf
response {
  # 若 SSE event 的 JSON payload 满足 $.type == "message_delta"，则仅在该 event 中删除 $.usage
  sse_json_del_if "$.type" "message_delta" "$.usage";
}
```

- 仅对 `text/event-stream` 生效
- 条件要求 `<cond_path>` 取到的值是字符串，且必须**完全等于** `<equals>`
- 规则按顺序执行，且在 `json_*` 响应操作之前执行

### 5.6 error（错误归一化）

语法：

```conf
error { error_map openai; }
```

说明：

- 允许的 `mode`（加载期白名单校验）：`openai` / `common` / `passthrough`
- 如写多条，最后一条生效
- `passthrough`：跳过错误归一化，直接把上游错误响应透传给客户端

### 5.7 metrics（用量提取 / usage）

#### usage_extract

```conf
metrics { usage_extract openai; }
metrics { usage_extract anthropic; }
metrics { usage_extract gemini; }
metrics { usage_extract custom; }
```

说明：

- `openai`：兼容 OpenAI/OpenAI-compatible 的 usage 字段
- `anthropic`：兼容 Anthropic 的 usage 字段
- `gemini`：兼容 Gemini 原生 usage 字段（`usageMetadata.*`）
- `custom`：使用受限 JSONPath 子集从响应 JSON 中提取（可选加减表达式，见下）

#### 自定义 JSONPath 字段（仅建议配合 custom）

```conf
metrics {
  usage_extract custom;
  input_tokens_path "$.usage.input_tokens";
  output_tokens_path "$.usage.output_tokens";
  cache_read_tokens_path "$.usage.cache_read_input_tokens";
  cache_write_tokens_path "$.usage.cache_creation_input_tokens";
}
```

支持的 JSONPath 子集：

- `$.a.b.c`
- `$.items[0].x`
- `$.items[*].x`（对数组中的数值求和）

多条与覆盖规则：

- 这些字段都是“可选覆盖项”，同字段重复出现时以后者为准

#### finish_reason_extract

用于从响应 JSON 中提取 `finish_reason`（用于日志 / 计费上报等）。

```conf
metrics { finish_reason_extract openai; }
metrics { finish_reason_extract anthropic; }
metrics { finish_reason_extract gemini; }
metrics { finish_reason_extract custom; finish_reason_path "$.choices[0].finish_reason"; }
```

说明：

- `openai`：从 `$.choices[*].finish_reason` 提取（取第一个非空）
- `anthropic`：从 `$.stop_reason` / `$.delta.stop_reason` / `$.message.stop_reason` 提取（取第一个非空）
- `gemini`：从 `$.candidates[*].finishReason`（或 `finish_reason` 兜底）提取（取第一个非空）
- `custom`：必须提供 `finish_reason_path`（JSONPath 子集），从该路径提取 finish_reason

多条与覆盖规则：

- `finish_reason_extract` 在同一个 `metrics` block 内只能出现一次
- `finish_reason_path` 可作为覆盖项；同字段重复出现时以后者为准

#### finish_reason_path（可选覆盖项）

```conf
metrics {
  finish_reason_extract openai;
  finish_reason_path "$.choices[0].finish_reason";
}
```

- 作为兜底：当某些 provider 把 finish_reason 暴露在自定义位置时，可用该字段覆盖提取路径。

## 6. 可用的 Context（表达式变量）

在 `<expr>` 位置可以引用以下变量：

`$channel.base_url`  
渠道配置的 `base_url`（字符串）。

`$channel.key`  
渠道配置的 `key`（鉴权 token，字符串）。

`$request.model`  
原始请求的模型名（字符串）。

`$request.model_mapped`  
映射后的模型名（字符串）。默认等于 `$request.model`，可通过 `request { model_map ...; model_map_default ...; }` 修改。

表达式形态（v0.1）：

- 字符串字面量：`"abc"`
- 变量引用：`$channel.key`
- 连接：`concat("Bearer ", $channel.key)`

> 注意：除上述最小能力外，v0.1 不支持更复杂的函数/运算。

---

## 7. 指令参考（nginx 风格）

本节按 nginx 文档风格为每条指令给出：`Syntax / Default / Context`，并补充 `Multiple`（是否支持多条）与要点说明。

> 约定：Context 用 DSL 中的 block 名表示（例如 `auth` / `request` / `upstream` / `response` / `error` / `metrics` / `file`）。

### 7.1 顶层指令

#### syntax

```text
Syntax:  syntax "<version>";
Default: —
Context: file
Multiple: no
```

- 目前主要用于占位与版本标识，建议保留。

#### include

```text
Syntax:  include "<path>";
Default: —
Context: file
Multiple: yes
```

- `<path>` 可为相对路径或绝对路径；相对路径以当前文件所在目录为基准。
- include 在解析前做纯文本展开；最大深度 20；会检测循环引用。

### 7.2 provider（结构块）

> `provider/defaults/match/...` 是 block，不属于“指令”；但这里给出 nginx 文档风格摘要，方便查阅。

#### provider

```text
Syntax:  provider "<name>" { ... }
Default: —
Context: file
Multiple: yes
```

- 文件名必须与 `<name>` 一致：`config/providers/<name>.conf`。
- 名称匹配会统一转小写；建议 `<name>` 与文件名都使用小写。

#### defaults

```text
Syntax:  defaults { ... }
Default: —
Context: provider
Multiple: no
```

- 作为 provider 的默认配置；命中 match 后会基于 defaults 叠加/覆盖。

#### match

```text
Syntax:  match api = "<api-name>" [stream = true|false] { ... }
Default: —
Context: provider
Multiple: yes
```

- 只会命中第一条匹配的 match（按出现顺序）。

### 7.3 upstream_config（默认上游配置）

#### base_url（assignment）

```text
Syntax:  upstream_config { base_url = "<url>"; }
Default: —
Context: defaults
Multiple: no
```

- `base_url` 必填，且必须是字符串字面量（固定 URL）。
- 同时 `base_url` 仍是默认值：当渠道（DB）配置了 `base_url` 时优先使用渠道配置；仅当渠道 `base_url` 为空时才使用此处值。

### 7.4 auth（鉴权）

#### auth_bearer

```text
Syntax:  auth_bearer;
Default: —
Context: auth
Multiple: yes
```

- 效果：设置 `Authorization: Bearer <channel.key>`。
- 不支持配置 token/value（固定取渠道 key）。

#### auth_header_key

```text
Syntax:  auth_header_key <Header-Name>;
Default: —
Context: auth
Multiple: yes
```

- 效果：设置 `<Header-Name>: <channel.key>`。
- `<Header-Name>` 支持标识符或字符串（建议用字符串以支持 `-`）。
- 不支持配置 token/value（固定取渠道 key）。

#### oauth_mode

```text
Syntax:  oauth_mode <mode>;
Default: —
Context: auth
Multiple: yes（最后一条生效）
```

- `<mode>` 可选：`openai|gemini|qwen|claude|iflow|antigravity|kimi|custom`。
- 含义：开启运行时 OAuth token 交换。

#### auth_oauth_bearer

```text
Syntax:  auth_oauth_bearer;
Default: —
Context: auth
Multiple: yes
```

- 效果：设置 `Authorization: Bearer <oauth.access_token>`。

#### OAuth 相关指令

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
Default: 内置 mode 有默认值；custom 模式需显式配置关键字段
Context: auth
Multiple:
  - oauth_form: yes
  - 其他: yes（最后一条生效）
```

- `custom` 模式必填：
  - `oauth_token_url`
  - 至少一条 `oauth_form`

### 7.5 request（请求处理）

#### set_header

```text
Syntax:  set_header <Header-Name> <expr>;
Default: —
Context: request
Multiple: yes
```

- 支持多条；按顺序执行；同名 header 多次 set 时后者覆盖前者。

#### del_header

```text
Syntax:  del_header <Header-Name>;
Default: —
Context: request
Multiple: yes
```

- 支持多条；按顺序执行。
- 合并规则：`defaults` 的操作先执行，`match` 的操作追加在后执行。

#### model_map

```text
Syntax:  model_map <from-model> <expr>;
Default: —
Context: request
Multiple: yes
```

- 将 `<from-model>`（精确匹配 `$request.model`）映射为 `$request.model_mapped`。
- `<from-model>` 支持标识符或字符串（建议用字符串以避免特殊字符问题）。
- 同一 `<from-model>` 多次出现：后写的覆盖先写的。

#### model_map_default

```text
Syntax:  model_map_default <expr>;
Default: $request.model
Context: request
Multiple: yes
```

- 当没有任何 `model_map` 命中时，使用该表达式作为 `$request.model_mapped`。
- 多次出现时，以最后一条为准。

#### json_set

```text
Syntax:  json_set <jsonpath> <expr>;
Default: —
Context: request
Multiple: yes
```

- 对“已生成的上游请求 JSON”设置字段；不存在的对象路径会自动创建。
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。
- `<expr>` 在此处支持：`true/false/null`、整数、字符串字面量、变量/concat。

#### json_del

```text
Syntax:  json_del <jsonpath>;
Default: —
Context: request
Multiple: yes
```

- 删除字段；字段不存在时为 no-op。
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。

#### json_rename

```text
Syntax:  json_rename <from-jsonpath> <to-jsonpath>;
Default: —
Context: request
Multiple: yes
```

- 将字段从 `<from-jsonpath>` 移动到 `<to-jsonpath>`；源字段不存在时为 no-op。
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。

### 7.6 upstream（路由）

#### set_path

```text
Syntax:  set_path <path-or-expr>;
Default: —
Context: upstream
Multiple: yes
```

- 设置上游请求路径（覆盖原 path）。

#### set_query

```text
Syntax:  set_query <key> <expr>;
Default: —
Context: upstream
Multiple: yes
```

- 支持多条；同一 key 多次 set 时后者覆盖前者。

#### del_query

```text
Syntax:  del_query <key>;
Default: —
Context: upstream
Multiple: yes
```

- 支持多条。
- 执行顺序：先执行所有 `del_query`，再执行所有 `set_query`。

### 7.7 response（响应）

> 该 phase 语义是“选择一个响应策略”。如写多条，最后一条生效。

#### resp_passthrough

```text
Syntax:  resp_passthrough;
Default: —
Context: response
Multiple: yes
```

- 透传上游响应（上游已是 OpenAI-compatible）。

#### resp_map

```text
Syntax:  resp_map <mode>;
Default: —
Context: response
Multiple: yes
```

- 非流式响应映射；`mode` 取决于内置实现。

#### sse_parse

```text
Syntax:  sse_parse <mode>;
Default: —
Context: response
Multiple: yes
```

- 流式 SSE 映射；`mode` 取决于内置实现。

### 7.8 error（错误归一化）

#### error_map

```text
Syntax:  error_map <mode>;
Default: —
Context: error
Multiple: yes
```

- 允许的 `mode`（加载期白名单校验）：`openai` / `common` / `passthrough`。
- `passthrough`：跳过错误归一化，直接透传上游错误响应。

### 7.9 metrics（用量提取 / usage）

#### usage_extract

```text
Syntax:  usage_extract <mode>;
Default: —
Context: metrics
Multiple: no
```

- 目前支持：`openai` / `anthropic` / `gemini` / `custom`。

#### finish_reason_extract

```text
Syntax:  finish_reason_extract <mode>;
Default: —
Context: metrics
Multiple: no
```

- 目前支持：`openai` / `anthropic` / `gemini` / `custom`。

#### finish_reason_path

```text
Syntax:  finish_reason_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 可选覆盖项；当使用 `finish_reason_extract custom;` 时为必填。
- JSONPath 子集：`$.a.b.c` / `$.items[0].x` / `$.items[*].x`（`[*]` 时取第一个非空字符串）。

#### input_tokens

```text
Syntax:  input_tokens = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- `<expr>` 为受限表达式：只允许 `+/-`、JSONPath、整数常量；不支持括号/乘除/函数。
- JSONPath 子集与 `*_tokens_path` 相同：`$.a.b.c` / `$.items[0].x` / `$.items[*].x`（`[*]` 对数组求和）。
- 取不到/非数值按 `0` 处理。

#### output_tokens

```text
Syntax:  output_tokens = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- `<expr>` 规则同 `input_tokens`。

#### cache_read_tokens

```text
Syntax:  cache_read_tokens = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- `<expr>` 规则同 `input_tokens`。

#### cache_write_tokens

```text
Syntax:  cache_write_tokens = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- `<expr>` 规则同 `input_tokens`。

#### total_tokens

```text
Syntax:  total_tokens = <expr>;
Default: input_tokens + output_tokens
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 若未显式声明，则默认按 `input_tokens + output_tokens` 计算。

#### input_tokens_path

```text
Syntax:  input_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 等价于 `input_tokens = <jsonpath>;` 的简写（仅能写单个 JSONPath，不支持加减）。

#### output_tokens_path

```text
Syntax:  output_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 等价于 `output_tokens = <jsonpath>;` 的简写（仅能写单个 JSONPath，不支持加减）。

#### cache_read_tokens_path

```text
Syntax:  cache_read_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 等价于 `cache_read_tokens = <jsonpath>;` 的简写（仅能写单个 JSONPath，不支持加减）。

#### cache_write_tokens_path

```text
Syntax:  cache_write_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 等价于 `cache_write_tokens = <jsonpath>;` 的简写（仅能写单个 JSONPath，不支持加减）。

### 7.10 balance（上游余额查询）

#### balance_mode

```text
Syntax:  balance_mode <mode>;
Default: —
Context: balance
Multiple: no
```

- 支持：`openai` / `custom`。

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

- `balance_mode custom` 时必填。
- 支持绝对 URL 或相对 provider `base_url` 的路径。

#### balance / used

```text
Syntax:  balance = <expr>;
Syntax:  used = <expr>;
Default: —
Context: balance
Multiple: yes
```

- 受限表达式：只支持 JSONPath / 数值常量 + `+` `-`。

#### balance_path / used_path

```text
Syntax:  balance_path <jsonpath>;
Syntax:  used_path <jsonpath>;
Default: —
Context: balance
Multiple: yes
```

- 在 custom 模式下，如果未设置 `balance`，则 `balance_path` 必填。

#### balance_unit

```text
Syntax:  balance_unit <string>;
Default: USD
Context: balance
Multiple: yes
```

- 仅支持：`USD` / `CNY`。

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
Default: OpenAI dashboard 默认路径
Context: balance
Multiple: yes
```

- `balance_mode openai` 的可选覆盖项。

---

## 8. 内置变量参考

本节列出 v0.1 可在 `<expr>` 中使用的内置变量（均为字符串）。

> 说明：变量在运行期求值；当某变量在当前请求上下文中为空时，会展开为空字符串。

### 8.1 `$channel.*`

`$channel.base_url`

渠道（DB）配置的 `base_url`。在 `upstream_config { base_url = "..."; }` 的默认行为中会作为“渠道优先”来源：渠道非空则优先用渠道。

`$channel.key`

渠道（DB）配置的 `key`（鉴权 token）。`auth_bearer;` 与 `auth_header_key ...;` 会固定使用该值作为 token/value。

### 8.2 `$request.*`

`$request.model`

来自客户端请求的模型名。

`$request.model_mapped`

映射后的模型名。默认等于 `$request.model`；可通过 `model_map` 与 `model_map_default` 修改。

### 8.3 使用示例

```conf
request {
  set_header "x-upstream-model" $request.model_mapped;
}

auth {
  # Authorization: Bearer <channel.key>
  auth_bearer;
}

upstream {
  # 示例：把 model 拼进 path（注意这只是字符串拼接示例）
  set_path concat("/v1/", $request.model_mapped, "/chat/completions");
}
```
