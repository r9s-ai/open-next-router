# Provider 配置 DSL 语法（v0.1）

本文档描述 **onr**（open-next-router）使用的 Provider 配置 DSL。
本仓库的 DSL 主入口通常是 `config/onr.conf`，而这个文件一般会再 `include` 进 `config/providers/*.conf`。
默认情况下 ONR 会加载 `config/onr.conf`，而这个文件通常会先 `include modes/*.conf;`，再 `include providers;`，把全局预设和 provider 配置一起纳入进来。
如果需要，也仍然可以通过配置项 `providers.dir` 或环境变量 `ONR_PROVIDERS_DIR` 强制走目录模式。

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
    - [5.7.1 usage_root（推荐配合 usage_fact 使用）](#usage_root推荐配合-usage_fact-使用)
    - [5.7.2 usage_fact（推荐的新写法）](#usage_fact推荐的新写法)
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
    - [7.9.1 usage_root](#usage_root)
    - [7.9.2 usage_fact](#usage_fact)
- [8. 内置变量参考](#8-内置变量参考)

---

## 1. 基本约定

- **一个 provider 一个文件**：`config/providers/<provider>.conf`
- **文件名必须与 provider 名一致**：例如 `config/providers/openai.conf` 内必须是 `provider "openai" { ... }`
- **provider 名匹配不区分大小写**：匹配时会统一转小写；配置中建议使用小写
- **分号 `;` 必须写**：所有语句以 `;` 结束
- **只有 block 才使用花括号 `{}`**：指令本身不使用 `{}`（nginx 风格）
- **推荐书写风格：每行一条指令**（便于 diff 与可读性）
- 支持注释：`#`、`//`

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
include relative/or/absolute/path.conf;
include providers;
include providers/*.conf;
```

- 相对路径以「当前文件所在目录」为基准
- 如果 include 的目标是一个目录，会按文件名字典序展开该目录下全部 `*.conf`
- 支持 glob 模式，展开结果同样按字典序处理
- 目前仍兼容带引号写法，但更推荐使用 nginx 风格的不带引号写法
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
- `match api` 必须是下面列出的受支持 API 之一；未知 API 会在校验/加载阶段直接报错
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
- `gemini.predictLongRunning`（Gemini 原生：`POST /v1beta/models/{model}:predictLongRunning`）
- `gemini.getOperation`（Gemini 原生长任务查询：`GET /v1beta/{operation_name}`）
- `images.generations`
- `images.edits`
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

- `defaults` 先应用，再应用命中的 `match`，但**不是所有 phase 都按同一种方式合并**
- `metrics`、`request`、`balance` 中的大多数**单值字段**按“字段级继承 + 显式覆盖”处理：
  - `match` 里写了的字段覆盖 `defaults`
  - `match` 里没写的字段继续继承 `defaults`
  - 例如：`defaults.metrics` 写了 `finish_reason_extract openai_chat_completions;`，某个 `match.metrics` 只写 `usage_extract custom;`，则最终同时生效 `usage_extract custom;` 与 `finish_reason_extract openai_chat_completions;`
- `auth` / `request` 里的 header 操作，以及 `response` / `error` 里的 `json_*`、`sse_json_del_if` 等**列表型指令**，通常按“defaults 在前，match 追加在后”处理；若存在同一对象上的后续冲突，通常以后出现的指令结果为准
- `response` / `error` 里的单值指令（如主 `op` / `mode`）仍可被 `match` 覆盖
- `upstream` 不按通用“整块 merge”处理：`defaults.upstream_config` 主要提供 `base_url` 默认值；具体路由动作（如 `set_path`、query 改写）来自命中的 `match.upstream`
- `models` 在 v0.1 只有 `defaults`，没有 `match` 级覆盖
- 总结：不要把 `defaults` / `match` 理解成“整个 block 替换”，应以各 phase 的实际合并行为为准

phase 边界规则（非常重要）：

- `request` 负责“上游请求内容的构造”，包括请求体变换、header 操作，以及供后续路由表达式使用的 model 映射。
- `upstream` 只负责“上游目标的路由”，也就是 path、query、以及与 base_url 相关的目标选择。
- 不要把 header 或 body 的变更语义放进 `upstream`。

### 5.1 upstream_config（上游默认配置）

语法：

```conf
upstream_config {
  base_url = "https://api.example.com";
  transport http;
}
```

说明：

- `transport` 声明上游传输方式。默认是 `http`。
- `transport http` 使用普通 HTTP 上游，`base_url` 必填且必须非空；若渠道（DB）配置了 `base_url`，则优先使用渠道配置。
- `transport aws_sdk` 使用 AWS SDK Bedrock Runtime 上游；`base_url` 可选，配置时作为 SDK endpoint override，不配置时由 SDK 按 AWS region 派生 endpoint。

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
  oauth_mode openai;              # openai|gemini|qwen|claude|iflow|antigravity|kimi|google_service_account_file|custom
  oauth_refresh_token $channel.key;
  auth_oauth_bearer;
}
```

- `oauth_mode`：开启运行时 OAuth token 交换。
- `auth_oauth_bearer;`：注入 `Authorization: Bearer <oauth.access_token>`。
- 内置 mode 会提供默认 token endpoint 与请求格式。
- `custom` 模式必须显式配置 `oauth_token_url`，并至少有一条 `oauth_form`。
- `google_service_account_file` 使用运行时元数据中的 GCP service account credential：
  - ONR 文件模式提供 `credential_file`。
  - 嵌入式运行时可以直接提供 credential JSON。
  - `oauth_scope` 必填。Vertex AI 通常使用 `https://www.googleapis.com/auth/cloud-platform`。
  - 优先使用 credential JSON 中的 `token_uri`；为空时 fallback 到 `https://oauth2.googleapis.com/token`。
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

#### auth_sigv4_bedrock

```conf
auth { auth_sigv4_bedrock; }
```

用途：声明当前 provider 使用 AWS Bedrock SigV4 鉴权与 AWS SDK transport，不向 HTTP header 注入普通 token。

- ONR 文件模式从 key 配置读取 `aws_access_key_id`、`aws_secret_access_key`、`aws_session_token`、`aws_region`。
- 嵌入式运行时应从 channel/provider 元数据传入等价的 AWS credential 与 region。
- `aws_region` 是运行时元数据，不是 DSL `<expr>` 内置变量。
- 不能和 `auth_bearer`、`auth_header_key`、`auth_oauth_bearer` 或 OAuth token 交换指令混用。

### 5.3 request（请求处理）

> v0.1：`request` phase 负责“上游请求内容的构造”，同时承担请求头操作、请求体（JSON）变换，以及供后续路由表达式使用的 model 映射。

#### set_header（可多条）

```conf
request {
  set_header "x-trace-id" "trace-123";
  set_header "x-foo" concat("a-", $request.model_mapped);
  set_header "x-model-route" template("models/${request.model_mapped}");
}
```

说明：

- 支持多条；按顺序执行
- 同一 header 多次 `set_header`：后写的覆盖先写的
- header 值是字符串表达式，支持 `template(...)`

#### del_header（可多条）

```conf
request {
  del_header "x-remove-me";
}
```

说明：

- 支持多条；按顺序执行
- `defaults` 的操作在前，`match` 的操作追加在后，因此 match 更容易覆盖 defaults（例如 defaults 先 set，match 再 del）

#### pass_header（可多条）

```conf
request {
  pass_header "anthropic-beta";
}
```

说明：

- 将原始客户端请求中的某个 header 复制到上游请求。
- 如果源 header 不存在，则为 no-op。
- 支持多条；与 `set_header`、`del_header` 一样按声明顺序执行。
- 如果同一个 header 先 `pass_header`，后续又被 `set_header` 或 `del_header`，则以后出现的指令结果为准。

#### filter_header_values（可多条）

```conf
request {
  filter_header_values "anthropic-beta" "context-1m-*" "fast-mode-*";
  filter_header_values "x-feature-flags" "exp-*" "debug" separator=";";
}
```

说明：

- 用于过滤某个上游请求 header 中的“列表型值”。
- 语法：`filter_header_values <header> <pattern>... [separator="<sep>"];`
- 推荐风格：pattern 统一写成连续位置参数，不要使用逗号分隔参数的写法。
- 默认分隔符是 `,`。
- 运行时行为：
  - 读取当前上游请求 header 值
  - 按 `separator` 分割
  - 对每一项执行 `strings.TrimSpace`
  - 删除匹配任一 pattern 的项
  - 如果结果为空，则删除整个 header
  - 否则将剩余项重新拼接
- 输出格式会被规范化：
  - 如果 `separator == ","`，使用 `", "` 连接
  - 否则使用 `"<sep> "` 连接，例如 `"; "`
- pattern 使用简单的 `*` 通配语义；不支持正则表达式。

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

#### json_set / json_replace / json_set_if_absent / json_del / json_del_if_missing / json_rename / json_wrap_input_text / json_set_header_values / json_filter_values / json_del_with_condition（可多条）

```conf
request {
  json_set "$.stream" true;
  json_replace "$.model" $request.model;
  json_set_if_absent "$.instructions" "";
  json_set "$.user" "alice";
  json_rename "$.max_tokens" "$.max_completion_tokens";
  json_wrap_input_text "$.input";
  json_set_header_values "$.anthropic_beta" "anthropic-beta";
  json_filter_values "$.anthropic_beta" "computer-use-2025-01-24";
  after_req_map {
    json_set "$.anthropic_version" "bedrock-2023-05-31";
    json_del_with_condition "$.tools" "type" "web_search*" "web_fetch*";
    json_del_if_missing "$.tool_choice" "$.tools";
  }
  json_del "$.tools";
}
```

说明：

- 用途：对“已生成的上游请求 JSON”做轻量变换（在旧 adaptor 的 `ConvertRequest` 之后执行）
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）
- `json_set` 的值表达式支持：`true/false/null`、整数、字符串字面量、变量、`concat(...)`、`template(...)`
- `json_set`：设置字段；不存在的对象路径会自动创建
- `json_replace`：仅当目标路径已存在时替换字段；路径不存在时为 no-op，不会创建缺失对象或字段
- `json_set_if_absent`：仅当路径不存在时写入；已存在值会保留
- `json_del_if_missing`：当依赖路径不存在时删除目标路径，常用于前序变换删除依赖对象后同步移除伴随字段
- `json_wrap_input_text`：当路径值是字符串时，将其包装成 OpenAI Responses `input` message 列表；路径不存在或值已经是数组时为 no-op，其他类型会报错
- `json_set_header_values`：从原始下游用户请求 header 中读取条目并写成 JSON 字符串数组，不读取 request header 规则准备发送给上游的 header。默认按逗号拆分，可用 `separator="<sep>"` 覆盖；不接受额外过滤参数，需要过滤时在后面显式配置 `json_filter_values`
- `json_filter_values`：过滤已有 JSON 字符串数组，只保留允许的值或通配符匹配项
- `json_del_with_condition`：当对象字段匹配允许值或通配符时，删除该对象或数组中的匹配对象
- `after_req_map { ... }`：在 `req_map` 之后执行内部 JSON 操作；如果没有配置 `req_map`，则在普通请求 JSON 操作之后执行

#### json_map_value / json_clamp（字段值映射与数值钳制，可多条）

```conf
request {
  # 值映射：voice=alloy 时替换为 male-qn-qingse；未命中保持原值透传（与 model_map 的 fallthrough 语义一致）
  json_map_value "$.voice" "alloy" "male-qn-qingse";
  json_map_value "$.voice" "nova" "female-shaonv";

  # 数值钳制：把 speed 夹到上游允许区间 [0.5, 2.0]，区间内原值透传
  json_clamp "$.speed" min=0.5 max=2.0;
}
```

说明：

- `json_map_value <jsonpath> "<from>" <to-expr>;`：仅当路径上的值是**字符串**且等于 `<from>` 时替换为 `<to-expr>` 的求值结果；路径缺失、非字符串或未命中时保持原值。
- 同一路径的多条映射可用**块写法**一次列出，等价于展开成多条 `json_map_value`：
  ```conf
  json_map_value "$.voice" {
    "alloy" "male-qn-qingse";
    "echo"  "Deep_Voice_Man";
  }
  ```
- `json_clamp <jsonpath> min=<f> max=<f>;`：把路径上的数值钳制到 `[min, max]`（小于 `min` 取 `min`，大于 `max` 取 `max`，区间内原值不变）；路径缺失或非数值时为 no-op。`min`/`max` 均必填且要求 `max >= min`。
- 值表达式的数字字面量支持小数（如 `1.0`、`0.5`），`json_set` / `json_set_if_absent` 等写入时会以 JSON number 输出。

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
- `openai_chat_to_anthropic_messages`：OpenAI `chat.completions` 请求 JSON → Anthropic `/v1/messages` 请求 JSON。
  映射字段包括 `model`、`messages`、`system`、`tools`、`tool_choice`、`max_tokens`、`temperature`、`top_p`、`stream` 和 `response_format`。
  `response_format` 约束：
  - `type: "text"` 或未设置 — 不设置 `output_config`（默认行为）。
  - `type: "json_object"` — 返回 400 错误；请改用带有显式 schema 的 `json_schema`。
  - `type: "json_schema"` — 映射到 `output_config.format.type = "json_schema"`。`json_schema.schema` 字段必须存在，且必须设置 `additionalProperties: false`，否则返回 400 错误。

### 5.4 upstream（路径与 query 操作）

`upstream` phase 只负责上游目标路由。
它应只承载 path / query / base_url 相关的选择逻辑，不应承载请求头或请求体的变更语义。

#### set_path

```conf
upstream {
  set_path "/v1/chat/completions";
}
```

说明：

- 设置上游请求路径（会覆盖原路径）
- 支持字符串表达式。当需要在 path 字符串内部展开内置变量时，使用 `template("...")`：

```conf
upstream {
  set_path template("/v1beta/models/${request.model_mapped}:generateContent");
}
```

- 模板占位符按约定使用不带 `$` 前缀的内置变量名。
  `${$request.model_mapped}` 也可使用。
- 普通字符串字面量不会展开变量。例如，`"/v1/$request.model_mapped"` 会被当作普通 path 字面量。
- 如需在模板中保留字面量 `${...}`，可写成 `\${...}`。
- 使用 credential 与 channel 元数据的示例：

```conf
upstream {
  set_path template("/v1/projects/${credential.project_id}/locations/${channel.location}/publishers/google/models/${request.model_mapped}:generateContent");
}
```

- `set_path` 的值必须是 path 形态并以 `/` 开头。不接受纯变量 path；如需变量，请在带有静态 `/`
  前缀的 `template(...)` 或 `concat(...)` 中嵌入变量。

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
- query 值是字符串表达式，支持 `template(...)`
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

#### AWS SDK transport 下的 set_path

`transport aws_sdk` 仍然复用 `set_path` 表达 Bedrock Runtime 目标资源：

```conf
upstream {
  set_path template("/model/${request.model_mapped}/invoke");
}
```

流式 Bedrock Runtime：

```conf
upstream {
  set_path template("/model/${request.model_mapped}/invoke-with-response-stream");
}
```

说明：

- `/model/{modelId}/invoke` 映射为 Bedrock Runtime `InvokeModel`。
- `/model/{modelId}/invoke-with-response-stream` 映射为 Bedrock Runtime `InvokeModelWithResponseStream`。
- `modelId` 通常使用 `$request.model_mapped`，支持 base model ID、inference profile ID 或 ARN。
- `upstream` 在 request model 映射之后生效，因此 path template 可以取到 `model_map` 的结果。

### 5.5 response（响应处理）

该 phase 会选择响应策略。对 `resp_passthrough` / `resp_map` / `sse_parse` 这类策略指令，如写多条，**最后一条生效**。`sse_collect` 是独立的非流式前置聚合步骤，可以后接 `resp_map`。

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

#### sse_collect

```conf
response { sse_collect <mode>; }
```

用途：把上游 `text/event-stream` 聚合成同上游协议的非流式 JSON 对象，适用于下游非流式、但上游返回 SSE 的路由。

- `sse_collect` 不做跨协议转换。
- 聚合后可继续执行可选的 `resp_map` 和响应 JSON 操作。
- `resp_map` 不是必需的；没有 `resp_map` 时，ONR 直接返回聚合后的同协议 JSON。
- `sse_collect` 不能和 `sse_parse` 或 `resp_passthrough` 组合使用。
- `sse_collect` 只允许用于 `stream = false` 的 match。

支持的模式：

- `openai_responses`：OpenAI/Azure Responses SSE → Responses JSON
- `anthropic_messages`：Anthropic Messages SSE → Message JSON
- `gemini_generate_content`：Gemini `streamGenerateContent` SSE → `GenerateContentResponse` JSON

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

AWS Bedrock 简例：

```conf
provider "aws-bedrock" {
  defaults {
    upstream_config {
      transport aws_sdk;
      # base_url = "https://bedrock-runtime.us-east-1.amazonaws.com";
    }
    auth {
      auth_sigv4_bedrock;
    }
    request {
      after_req_map {
        json_set "$.anthropic_version" "bedrock-2023-05-31";
      }
    }
  }

  match api = "chat.completions" stream = false {
    request {
      model_map "claude-3-5-sonnet-20241022" "anthropic.claude-3-5-sonnet-20241022-v2:0";
      req_map openai_chat_to_anthropic_messages;
    }
    upstream {
      set_path template("/model/${request.model_mapped}/invoke");
    }
    response {
      resp_map anthropic_to_openai_chat;
    }
  }
}
```

#### json_del / json_set / json_replace / json_set_if_absent / json_rename（响应 JSON 操作）

这些指令会对下游响应体做**尽力而为（best-effort）**的 JSON 变换：

```conf
response {
  json_del "$.usage";
  json_set "$.foo" "bar";
  json_replace "$.model" $request.model;
  json_set_if_absent "$.bar" "baz";
  json_rename "$.a" "$.b";
}
```

语义：

- 非流式 JSON：对整个响应体的 JSON **对象**做变换
- 流式 SSE（`text/event-stream`）：对每个 SSE event 拼接后的 `data:` JSON **对象**做变换
- 非 JSON / 非对象负载：保持原样透传
- 执行顺序：按配置块内出现顺序依次执行
- `json_set` 会创建缺失路径；`json_replace` 只替换已存在路径，适合替换上游响应中的 `model` 字段而不污染其它事件。
- `json_set`、`json_replace`、`json_set_if_absent` 的值表达式支持 `template(...)`
- 流式 SSE 中，`json_set`、`json_replace`、`json_set_if_absent`、`json_del`、`json_rename` 可追加 `event="<name|name2>"`，按 SSE `event:` 名过滤执行。
- response JSON 操作还可在带 `event="..."` 时追加 `event_optional=true`：如果运行时有 event，仍必须匹配指定事件名；如果没有 event 上下文，则允许按普通 JSON 匹配回退执行。
- response JSON 操作可追加 `max_count=<n>` 限制单条指令在一次响应处理周期内的最大生效次数；默认 `max_count=0` 表示不限次数。

```conf
response {
  resp_passthrough;
  json_set "$.response.metadata.gateway" "onr" event="response.completed" event_optional=true;
  json_replace "$.message.model" $request.model event="message_start" event_optional=true max_count=1;
  json_replace "$.response.model" $request.model event="response.created|response.completed|response.incomplete" event_optional=true;
}
```

- `event="..."` 只影响 SSE JSON event；非流式 JSON 响应没有 event 上下文，带 event 的响应 JSON 操作会跳过，除非该指令支持并启用了 `event_optional=true`。
- `event_optional=true` 必须与 `event` 同时出现；当运行时没有 event 上下文时，这条指令会退化为普通 JSON 匹配。
- `max_count` 按单条指令计数：非流式 JSON 最多处理一次对象；SSE 会跨整个 stream 累计。只有实际修改成功才计数，例如 `json_replace` 路径不存在时不计数。

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

#### resp_body_extract / resp_content_type（JSON → 二进制响应）

用于“上游用 JSON 包裹二进制内容”的端点（例如 TTS 返回 hex 编码音频）：

```conf
match api = "audio.speech" stream = false {
  response {
    resp_body_extract path="$.data.audio" decode=hex;
    resp_content_type from_path="$.extra_info.audio_format" kind=audio default="mp3";
  }
}
```

说明：

- `resp_body_extract path="$.a.b" decode=hex|base64;`：非流式。把上游 JSON 响应中 `path` 处的字符串按 `decode` 解码，作为下游响应体输出（body 形态从 JSON 变为二进制）。
- 用量 / 错误检测先在**原始 JSON** 上完成，再做 body 变换；计费上下文保留原始 JSON。
- `resp_content_type from_path="$.path" kind=audio [default="mp3"];`：从上游 JSON 的格式字段解析下游 `Content-Type`。`kind=audio` 表示值是音频格式名（mp3/wav/flac/pcm/aac/opus/ogg），未知格式回退 `audio/mpeg`；`from_path` 缺失时用 `default`。
- 单个 response 块内各指令最多一条；与 `resp_map` / `sse_parse` 互斥使用（配合 `resp_passthrough` 或独立使用）。

#### sse_binary_extract（SSE → 二进制流）

用于“上游 SSE JSON chunk 包裹二进制分块”的流式端点：

```conf
match api = "audio.speech" stream = true {
  response {
    sse_binary_extract path="$.data.audio" decode=hex stop_path="$.data.status" stop_eq=2;
  }
}
```

说明：

- 对每个 SSE event 的 JSON payload：取 `path` 处字符串按 `decode` 解码，将解码后的**裸二进制**写入下游响应（不是 SSE 转发）。
- `stop_path` / `stop_eq` 成对出现：当 payload 中 `stop_path` 的值等于 `stop_eq`（数值或字符串比较）时结束流。
- 每个 chunk 的 JSON payload 仍会进入 usage 提取通路，供流式计费读取尾块字段。
- 流式 `Content-Type`：响应头在首个音频字节前发送，此时尾包字段还没到，`resp_content_type from_path=` 无法生效。运行时应优先取客户端请求的格式（如 `response_format`），再回退规则的 `default`。
- chunk 解码失败按致命处理：首字节写出前直接返回上游错误；已写出后在坏块处截断流（跳过坏块继续拼接会产生"中间缺一段"的静默损坏音频）。

### 5.6 error（错误归一化）

语法：

```conf
error { error_map openai; }
```

说明：

- 允许的 `mode`（加载期白名单校验）：`openai` / `common` / `passthrough`
- 如写多条，最后一条生效
- `passthrough`：跳过错误归一化，直接把上游错误响应透传给客户端

#### error_when（体内错误检测）

```conf
error {
  error_map openai;
  error_when path="$.base_resp.status_code" ne=0 status=400;
}
```

说明：

- 针对“HTTP 200 但业务错误藏在响应体里”的上游：当 `path` 处的值满足 `ne=`（不等于）或 `eq=`（等于）条件时，把该响应按上游错误处理，经 `error_map` 归一化后以 `status`（默认 400）返回下游。
- `ne` / `eq` 只能二选一；数值与字符串都支持（`ne=0` 会与 JSON number 做数值比较）。
- **路径缺失永远不算命中**（对 `ne` 也是），避免正常响应没有错误字段时误报。
- 可写多条，任意一条命中即判定为错误；仅对 HTTP 2xx 响应生效，非 2xx 仍走原有 `error_map` 流程。
- 流式边界：二进制流（`sse_binary_extract`）下规则仅在首个音频字节写出前生效。HTTP 状态行一旦发送便无法改写，流中途出现的错误信封只记日志、按已写出内容结束流,不再转换状态码。

### 5.7 metrics（用量提取 / usage）

#### usage_mode（全局可复用 usage 预设）

```conf
usage_mode "shared_openai" {
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}
```

说明：

- `usage_mode` 是顶层指令，用来给整个 providers 配置集合声明一个可复用的全局 usage 预设。
- 推荐把这类全局 DSL 预设单独放在 `config/modes/usage_modes.conf` 这类文件里，再由 `config/onr.conf` 通过 `include modes/*.conf;` 在 `include providers;` 之前先引入。
- 它可以单独放在一个没有 `provider {}` 的 `.conf` 文件里；这类文件在 `config/providers/` 中是合法的，但不会出现在 provider 列表里。
- `usage_mode` 块内支持和 `metrics` 相同的 usage 指令：`usage_extract`、`usage_root`、`usage_fact`、`*_tokens_path`、`*_tokens_expr`。
- `usage_mode` 内部也可以继续通过 `usage_extract <other_mode>;` 引用另一个 `usage_mode`，用于组合更大的预设；递归引用会报错。
- 在同一个 providers 目录或合并后的 providers 文件中，`usage_mode` 名字是全局唯一的；重名会在校验期报错。
- 本仓库默认的 `config/modes/usage_modes.conf` 会定义 `openai_chat_completions`、`openai_prompt_completion`、`openai_responses`、`openai_responses_stream`、`anthropic_messages`、`anthropic_messages_stream`、`gemini_generate_content`、`gemini_generate_content_stream` 这类按 API / 路径拆分的全局 `usage_mode` 预设；如果你在 DSL 里声明同名 `usage_mode`，就会覆盖这份默认预设。
- 执行时，`usage_extract <custom_name>;` 会先解析到对应的 `usage_mode`，再编译成与 builtin mode 相同的最终 usage plan。

#### finish_reason_mode（全局可复用 finish_reason 预设）

```conf
finish_reason_mode "anthropic_messages_stream" {
  finish_reason_path "$.delta.stop_reason";
  finish_reason_path "$.message.stop_reason" fallback=true;
}
```

说明：

- `finish_reason_mode` 是顶层指令，用来给整个 providers 配置集合声明一个可复用的全局 finish_reason 预设。
- 推荐把这类全局 DSL 预设单独放在 `config/modes/finish_reason_modes.conf` 这类文件里，再由 `config/onr.conf` 通过 `include modes/*.conf;` 在 `include providers;` 之前先引入。
- 它也可以单独放在一个没有 `provider {}` 的 `.conf` 文件里。
- `finish_reason_mode` 块内支持和 `metrics` 中 finish reason 提取相同的指令：`finish_reason_extract`、`finish_reason_path`。
- `finish_reason_mode` 内部也可以继续通过 `finish_reason_extract <other_mode>;` 引用另一个 `finish_reason_mode`，用于组合更大的预设；递归引用会报错。
- 在同一个 providers 目录或合并后的 providers 文件中，`finish_reason_mode` 名字是全局唯一的；重名会在校验期报错。
- 本仓库默认的 `config/modes/finish_reason_modes.conf` 会定义 `openai_chat_completions`、`openai_completions`、`openai_responses`、`anthropic_messages`、`anthropic_messages_stream`、`gemini_generate_content`、`gemini_generate_content_stream` 这类更具体的全局 `finish_reason_mode` 预设；如果你在 DSL 里声明同名 `finish_reason_mode`，就会覆盖这份默认预设。

#### models_mode（全局可复用 models 预设）

```conf
models_mode "openai" {
  path "/v2/models";
}
```

说明：

- `models_mode` 也可以作为顶层指令，用来给整个 providers 配置集合声明一个可复用的全局 models 预设。
- 推荐把这类全局 DSL 预设单独放在 `config/modes/models_modes.conf` 这类文件里，再由 `config/onr.conf` 通过 `include modes/*.conf;` 在 `include providers;` 之前先引入。
- 它也可以单独放在一个没有 `provider {}` 的 `.conf` 文件里。
- `models_mode` 块内支持和 `models` phase 相同的指令：`models_mode`、`method`、`path`、`id_path`、`id_regex`、`id_allow_regex`、`set_header`、`del_header`。
- `models_mode` 内部也可以继续通过 `models_mode <other_mode>;` 引用另一个 `models_mode`，用于组合更大的预设；递归引用会报错。
- 在同一个 providers 目录或合并后的 providers 文件中，`models_mode` 名字是全局唯一的；重名会在校验期报错。

#### balance_mode（全局可复用 balance 预设）

```conf
balance_mode "openai" {
  usage_path "/v9/dashboard/billing/usage";
}
```

说明：

- `balance_mode` 也可以作为顶层指令，用来给整个 providers 配置集合声明一个可复用的全局 balance 预设。
- 推荐把这类全局 DSL 预设单独放在 `config/modes/balance_modes.conf` 这类文件里，再由 `config/onr.conf` 通过 `include modes/*.conf;` 在 `include providers;` 之前先引入。
- 它也可以单独放在一个没有 `provider {}` 的 `.conf` 文件里。
- `balance_mode` 块内支持和 `balance` phase 相同的指令：`balance_mode`、`method`、`path`、`balance_path`、`balance_expr`、`used_path`、`used_expr`、`balance_unit`、`subscription_path`、`usage_path`、`set_header`、`del_header`。
- `balance_mode` 内部也可以继续通过 `balance_mode <other_mode>;` 引用另一个 `balance_mode`，用于组合更大的预设；递归引用会报错。
- 在同一个 providers 目录或合并后的 providers 文件中，`balance_mode` 名字是全局唯一的；重名会在校验期报错。
- 本仓库默认的 `config/modes/balance_modes.conf` 会定义 `openai` 这个全局 `balance_mode` 预设；如果你在 DSL 里声明同名 `balance_mode`，就会覆盖这份默认预设。

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

说明：

- `custom`：使用受限 JSONPath 子集从响应 JSON 中提取（可选加减表达式，见下）
- 其他任意 mode 名：用户自定义的全局 `usage_mode`
- 仓库默认配置现在只保留更具体的 API / 路径级预设，例如 `openai_chat_completions`、`openai_prompt_completion`、`openai_responses`、`openai_responses_stream`、`anthropic_messages`、`anthropic_messages_stream`、`gemini_generate_content`、`gemini_generate_content_stream`。
- `openai` / `anthropic` / `gemini` 这类泛化名字不再是特殊的 DSL 内置 `usage_extract` mode；如果你想继续用这些名字，需要自己显式定义对应的全局 `usage_mode` 预设。
- 用户自定义 `usage_mode` 也会先完成解析，再编译进同一套统一 fact-based 执行计划。
- 在 `metrics` 里，如果声明了 `usage_fact`、`*_tokens_path` 或 `*_tokens_expr`，但没有写 `usage_extract`，ONR 会自动按 `usage_extract custom;` 处理。仅声明 `usage_root` 不会触发 usage 提取，因为它只是给 `usage_fact` 提供读取根对象。
- 若在代码侧做 introspection，建议区分三层：
  - declared：用户显式写的 `usage_fact`
  - compiled：最终真正参与执行的统一 usage plan

旧的泛化 provider mode 迁移到 `custom` 时，可参考以下近似写法：

- `openai` 等价思路：

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

  # 可选：仅当上游真实返回分模态 cached token 时配置。
  usage_fact cache_read token path="$.usage.cache_details.text.cached_tokens" attr.modality="text";
  usage_fact cache_read token path="$.usage.cache_details.image.cached_tokens" attr.modality="image";
  usage_fact cache_read token path="$.usage.cache_details.audio.cached_tokens" attr.modality="audio";
}
```

- `anthropic` 等价思路：

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

- `gemini` 等价思路：

```conf
metrics {
  usage_extract custom;

  usage_fact input token path='$.usageMetadata.promptTokensDetails[?(@.modality=="TEXT")].tokenCount';
  usage_fact input token path="$.usageMetadata.promptTokenCount" fallback=true;
  usage_fact input.image token path='$.usageMetadata.promptTokensDetails[?(@.modality=="IMAGE")].tokenCount';
  usage_fact input.video token path='$.usageMetadata.promptTokensDetails[?(@.modality=="VIDEO")].tokenCount';
  usage_fact input.audio token path='$.usageMetadata.promptTokensDetails[?(@.modality=="AUDIO")].tokenCount';

  # 可选：仅当上游真实返回分模态 cached token 时配置。
  usage_fact cache_read token path='$.usageMetadata.cacheTokensDetails[?(@.modality=="TEXT")].tokenCount' attr.modality="text";
  usage_fact cache_read token path='$.usageMetadata.cacheTokensDetails[?(@.modality=="IMAGE")].tokenCount' attr.modality="image";
  usage_fact cache_read token path='$.usageMetadata.cacheTokensDetails[?(@.modality=="AUDIO")].tokenCount' attr.modality="audio";
  usage_fact cache_read token path='$.usageMetadata.cacheTokensDetails[?(@.modality=="VIDEO")].tokenCount' attr.modality="video";

  usage_fact output token path="$.usageMetadata.candidatesTokenCount";
  usage_fact output token path="$.usageMetadata.thoughtsTokenCount";
  usage_fact output.image token path='$.usageMetadata.candidatesTokensDetails[?(@.modality=="IMAGE")].tokenCount';
}
```

注意：

- `gemini`：当前默认预设行为已经可以用 `custom` 配置完整平替；`input token` 通常优先取 `TEXT` 模态，再 fallback 到 `promptTokenCount`
- `anthropic`：ONR 现在将 `input` 视为包含 cache 的有效输入总量，因此 `cache_read_input_tokens` 与 `cache_creation_input_tokens` 也应并入 `input`
- `openai`：上述配置只覆盖核心 token / cache 提取；图片、音频、tool usage 以及分模态 cache 等 API-specific supplemental facts 仍需额外显式写 `usage_fact`
- 分模态 cache 复用现有 `cache_read token` 维度，并通过 `attr.modality="text|image|audio|video"` 标识真实模态。只有当上游明确返回分模态 cached token 时才应配置这些字段；不要把一个总 cached token 字段按比例拆分后上报为分模态事实。
- Gemini 的 `candidatesTokenCount` 是候选输出总量。下游计费应将 `output.image token` 等分模态输出事实视为普通 output 的组成分量，避免同一批 token 被重复收费。
- `total_tokens` 默认会由 `input + output` 自动聚合；通常不建议再显式配置 `total_tokens_expr`，避免引入多个事实源
- 当前 Gemini 原生 usage 字段按驼峰命名处理：`usageMetadata.promptTokenCount` / `candidatesTokenCount` / `thoughtsTokenCount` / `totalTokenCount`

Anthropic 流式场景的 `custom` 写法示意：

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

- 这类配置可以覆盖 Anthropic stream 的主要 usage 事件
- `event="..."` 可以把 `usage_fact` 限制在指定的 SSE `event:` 名上，只在流式提取时生效
- `event_optional=true` 可与 `event="..."` 搭配使用：如果上游没有返回 `event:`，这条规则会退化为普通 chunk 匹配；如果有 `event:`，仍然必须匹配指定名字
- 相比旧的泛化 `anthropic` mode，主要差异在于配置更长、更容易漏掉某一种事件路径

OpenAI supplemental facts 的 `custom` 补充示意：

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

- 旧的泛化 OpenAI 行为本质上仍然可以用 `usage_fact` 补齐
- 但这些规则与具体 API 强相关，所以不像 Gemini/Anthropic 核心 token 提取那样能用一小段通用配置统一覆盖

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
- `$.items[?(@.field=="VALUE")].x`（对过滤后命中的数值求和）

说明：

- 当前 filter 仅支持数组过滤
- 当前仅支持单条件等值匹配：`==`
- 当前仅支持字符串字面量比较
- 例如：
  - `$.usageMetadata.promptTokensDetails[?(@.modality=="AUDIO")].tokenCount`
- 当路径写在 DSL 双引号字符串里时，filter 里的内部双引号需要转义，例如：

```conf
usage_fact input.audio token path='$.usageMetadata.promptTokensDetails[?(@.modality=="AUDIO")].tokenCount';
```

多条与覆盖规则：

- 这些字段都是“可选覆盖项”，同字段重复出现时以后者为准

#### usage_root（推荐配合 usage_fact 使用）

```conf
metrics {
  usage_extract custom;

  usage_root path="$.usage";
  usage_root path="$.message.usage" event="message_start" event_optional=true exclude="output_tokens";

  usage_fact input token path="$.input_tokens";
  usage_fact output token path="$.output_tokens";
}
```

- `usage_root` 用于先从响应 JSON / SSE event JSON 中提取 usage 子树。
- 配置了 `usage_root` 后，未显式写 `source` 的 `usage_fact` 默认从合并后的 usage 子树读取；未配置 `usage_root` 时仍兼容旧行为，默认从原始 `response` 读取。
- 可以配置多条 `usage_root`，命中的 usage 对象会合并到同一个结构里，后续非零字段可补齐或覆盖前面的零值字段。
- 流式提取时，ONR 会先在每个 chunk 中合并 `usage_root`，等 stream 结束后再从合并结果执行默认 source / `source=usage` 的 `usage_fact`；显式 `source=response` / `request` / `derived` 的事实仍在 chunk 阶段按自己的 source 与 event 执行。
- `event="a|b"` 可用于匹配多个 SSE 事件名。
- `event_optional=true` 可选，需要与 `event="..."` 一起使用，用于兼容上游有时不带 SSE `event:` 的情况。
- `exclude="field_a|field_b"` 会在合并前从提取出的 usage 对象里删除顶层字段；不支持 JSONPath 或嵌套路径。
- `usage_root` 不支持 `name`，统一合并为一个 usage 对象。
- 需要继续从原始响应读取的事实应显式写 `source="response"`，例如：

```conf
usage_fact server_tool.web_search call source="response" count_path="$.output[*]" type="web_search_call" status="completed";
```

#### usage_fact（推荐的新写法）

```conf
metrics {
  usage_extract custom;

  usage_root path="$.usage";

  usage_fact input token path="$.input_tokens";
  usage_fact output token path="$.output_tokens";
  usage_fact cache_read token path="$.cache_read_input_tokens";
  usage_fact cache_read token path="$.cache_details.image_cached_tokens" attr.modality="image";

  usage_fact cache_write token path="$.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
  usage_fact cache_write token path="$.cache_creation.ephemeral_1h_input_tokens" attr.ttl="1h";
  usage_fact cache_write token path="$.cache_creation_input_tokens" fallback=true;
}
```

- `usage_fact` 用于把不同来源的用量统一抽成规范化事实
- 在 `metrics` 里，只写 `usage_fact` 而省略 `usage_extract`，等价于 `usage_extract custom;`
- 命名 preset 现在也会先编译成同一套内部 fact-based 执行计划，再叠加显式 `usage_fact`
- 第一批支持 `path` / `count_path` / `sum_path` / `expr`
- `event="..."` 可选，用于把 `usage_fact` 限制在指定 SSE 事件，例如 `message_start` / `message_delta`；也支持 `response.completed|response.incomplete` 这种多事件写法
- `event_optional=true` 可选，需要与 `event="..."` 一起使用，用于兼容“有时有 event、有时没有 event”的上游
- `attr.ttl` 用于区分 Anthropic 的 `5m` / `1h` cache write
- `attr.modality="text|image|audio|video"` 可用于 `cache_read token`，表示上游真实返回的分模态 cached token。Relay 会用它生成 `openai_cache` / `gemini_cache` 这类 provider cache detail 字段。
- 同一 `dimension + unit` 可声明多条 `usage_fact`；所有命中的非 fallback 规则会按声明顺序累计求和
- `fallback=true` 用于在更具体的事实不存在时回退到总量字段
- `source` 在配置了 `usage_root` 时默认是 `usage`，否则默认是 `response`
- `source=usage` 表示从 `usage_root` 合并后的 usage 对象读取；`source=response` 表示从当前响应 JSON 提取；`source=request` 表示从当前请求 JSON 提取；`source=derived` 表示从 ONR 运行时派生出的聚合结果提取
- 当前不支持除 `usage` / `response` / `request` / `derived` 之外的其他 `source`
- `dimension` 是 registry 中的扁平命名空间字符串，`.` 只是名称的一部分，不表示嵌套结构
- 当前支持的 `dimension` 与 `dimension + unit` 固定 registry，完整列表见后文 [`usage_fact`](#usage_fact) 指令说明
- 仓库内置的 OpenAI API-specific 预设通常会补齐这些 canonical facts：
  - `openai_images_generations -> image.generate image`
  - `openai_images_edits -> image.edit image`
  - `openai_audio_transcriptions -> audio.stt second`
  - `openai_audio_translations -> audio.translate second`
  - `openai_audio_speech -> audio.tts second`
  - `openai_responses -> server_tool.web_search call`

补充示例：

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

#### 生成类 Gemini 多模态 token facts

这部分已拆到根仓文档，便于把 Gemini 方案细节与 DSL 语法说明分开维护。

详见：[ONR 生成类 Gemini 多模态 Token Facts](../../docs/specs/ONR_GEMINI_MULTIMODAL_TOKEN_FACTS.md)

这里保留一条最常用的最小示例：

```conf
metrics {
  usage_extract custom;

  usage_fact input token path='$.usageMetadata.promptTokensDetails[?(@.modality=="TEXT")].tokenCount';
  usage_fact input token path="$.usageMetadata.promptTokenCount" fallback=true;
  usage_fact input.image token path='$.usageMetadata.promptTokensDetails[?(@.modality=="IMAGE")].tokenCount';
  usage_fact input.video token path='$.usageMetadata.promptTokensDetails[?(@.modality=="VIDEO")].tokenCount';
  usage_fact input.audio token path='$.usageMetadata.promptTokensDetails[?(@.modality=="AUDIO")].tokenCount';

  usage_fact output token path="$.usageMetadata.candidatesTokenCount";
  usage_fact output token path="$.usageMetadata.thoughtsTokenCount";
  usage_fact output.image token path='$.usageMetadata.candidatesTokensDetails[?(@.modality=="IMAGE")].tokenCount';
}
```

- 范围：仅适用于 `gemini.generateContent` / `gemini.streamGenerateContent`
- 口径：`input token` 优先取 `TEXT` 模态，再 fallback 到 `promptTokenCount`
- 这类场景是 token 型 usage，不涉及 Live / realtime 的 `second` 型 fact

- 现在可以直接用 filter JSONPath 从 `promptTokensDetails` 按 `modality` 取值
- ONR builtin 与显式 `usage_fact` 现在都支持这套 Gemini 多模态 token 语义

后续范围说明：

- Gemini Live / Realtime 的 `input.audio second`、`output.audio second`、`input.vision second`
- 其他厂商 realtime 接口的通用 meter-based facts
- 需要 `source=derived` 的 runtime 聚合场景

#### finish_reason_extract

用于从响应 JSON 中提取 `finish_reason`（用于日志 / 计费上报等）。

```conf
metrics { finish_reason_extract openai_chat_completions; }
metrics { finish_reason_extract anthropic_messages; }
metrics { finish_reason_extract gemini_generate_content; }
metrics { finish_reason_path "$.choices[0].finish_reason"; }
```

说明：

- `custom`：必须提供 `finish_reason_path`（JSONPath 子集），从该路径提取 finish_reason
- 其他任意 mode 名：用户自定义的全局 `finish_reason_mode`
- `finish_reason_mode` 在块内如果省略 `finish_reason_extract`，但已经声明了 `finish_reason_path`，ONR 会自动按 `custom` 处理。
- 在 `metrics` 里，只写 `finish_reason_path` 而省略 `finish_reason_extract`，等价于 `finish_reason_extract custom;`
- 仓库默认配置现在只保留更具体的 API / 路径级预设，例如 `openai_chat_completions`、`openai_completions`、`openai_responses`、`openai_responses_stream`、`anthropic_messages`、`anthropic_messages_stream`、`gemini_generate_content`、`gemini_generate_content_stream`。
- `openai` / `anthropic` / `gemini` 这类泛化名字不再是特殊的 DSL 内置 `finish_reason_extract` mode；如果你想继续用这些名字，需要自己显式定义对应的全局 `finish_reason_mode` 预设。

这些 path-specific 预设对应的提取路径：

- `openai`
  - `chat.completions` / `completions`：读取 `$.choices[*].finish_reason`（取第一个非空）
  - `responses` 非流式：读取 `$.incomplete_details.reason`
  - `responses` 流式 SSE 包装层：读取 `$.response.incomplete_details.reason`
- `anthropic`
  - 依次读取 `$.stop_reason`、`$.delta.stop_reason`、`$.message.stop_reason`
- `gemini`
  - 优先读取 `$.candidates[*].finishReason`
  - 若为空则回退到 `$.candidates[*].finish_reason`

等效 `custom` 配置示例：

- OpenAI Chat/Completions：

```conf
metrics {
  finish_reason_path "$.choices[0].finish_reason";
}
```

- OpenAI Responses 原始 reason：

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.incomplete_details.reason";
}
```

  这和当前内置 `openai` 在非流式 `responses` 下是等价的。

- OpenAI Responses SSE 包装层：

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.incomplete_details.reason";
  finish_reason_path "$.response.incomplete_details.reason" fallback=true;
}
```

  这可以复刻当前内置 `openai` 对非流式和 `response.incomplete` 流式事件的覆盖范围。

- Anthropic 非流式：

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.stop_reason";
}
```

- Anthropic 流式 `message_delta`：

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.delta.stop_reason";
}
```

- Anthropic 流式 `message_start` 兜底：

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.message.stop_reason";
}
```

- Gemini：

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.candidates[0].finishReason";
}
```

  如果上游返回的是 snake_case，可改为 `$.candidates[0].finish_reason`。

多条与覆盖规则：

- `finish_reason_extract` 在同一个 `metrics` block 内只能出现一次
- `finish_reason_path` 可作为覆盖项；同字段重复出现时以后者为准

#### finish_reason_path（可选覆盖项）

```conf
metrics {
  finish_reason_extract openai_chat_completions;
  finish_reason_path "$.choices[0].finish_reason";
}
```

- 作为兜底：当某些 provider 把 finish_reason 暴露在自定义位置时，可用该字段覆盖提取路径。
- 允许声明多条 `finish_reason_path`。
- `fallback=true` 表示只有在前面的非 fallback 路径都没有提取到非空值时，这条路径才会生效。
- `event="<name>"` 可把某条路径限定在特定 SSE event 上，例如 `response.completed` 或 `message_delta`。
- `event_optional=true` 允许这条带事件的规则在“没有 event 上下文”的普通 JSON 提取场景继续作为回退规则生效。

示例：

```conf
metrics {
  finish_reason_extract custom;
  finish_reason_path "$.delta.stop_reason" event="message_delta" event_optional=true;
  finish_reason_path "$.message.stop_reason" event="message_start" event_optional=true fallback=true;
}
```

## 6. 可用的 Context（表达式变量）

在 `<expr>` 位置可以引用以下变量：

`$channel.base_url`  
渠道配置的 `base_url`（字符串）。

`$channel.key`  
渠道配置的 `key`（鉴权 token，字符串）。

`$channel.location`

渠道/provider 的 location（字符串），例如 `global` 或 `us-central1`。

`$credential.project_id`

从当前 credential 解析出的 project id（字符串）。

`$request.model`  
原始请求的模型名（字符串）。

`$request.model_mapped`  
映射后的模型名（字符串）。默认等于 `$request.model`，可通过 `request { model_map ...; model_map_default ...; }` 修改。

表达式形态（v0.1）：

- 字符串字面量：`"abc"`
- 变量引用：`$channel.key`
- 连接：`concat("Bearer ", $channel.key)`
- 模板字符串：`template("/v1/${request.model_mapped}")`
- Vertex 风格模板字符串：
  `template("/v1/projects/${credential.project_id}/locations/${channel.location}/publishers/google/models/${request.model_mapped}:generateContent")`

`template(...)` 只接受一个字符串字面量参数。模板占位符会在运行期使用与裸表达式相同的内置变量求值。
占位符名称通常省略开头的 `$`，例如 `${request.model_mapped}`。`${$request.model_mapped}` 也可使用。
未知占位符名称会在 provider 校验阶段报错。

模板字符串只在表达式位置生效，例如 `<expr>`、`<value-expr>`，以及文档明确标注的
`<path-or-url-or-template>` 字段。JSONPath、正则、mode 名称、header 名、query key、filter pattern
不会展开模板占位符。

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

#### usage_mode / finish_reason_mode / models_mode / balance_mode

```text
Syntax:  usage_mode "<name>" { ... }
Syntax:  finish_reason_mode "<name>" { ... }
Syntax:  models_mode "<name>" { ... }
Syntax:  balance_mode "<name>" { ... }
Default: —
Context: file
Multiple: yes
```

- 用于声明可复用的顶层 `metrics` / `models` / `balance` 预设块。
- 推荐分别放在 `config/modes/usage_modes.conf`、`config/modes/finish_reason_modes.conf`、`config/modes/models_modes.conf` 与 `config/modes/balance_modes.conf`。

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

- `transport http` 时，`base_url` 必填且必须是非空字符串字面量（固定 URL）。
- `transport aws_sdk` 时，`base_url` 可选；非空时作为 SDK endpoint override，不配置时由 SDK 按 region 派生 endpoint。
- 当渠道（DB）配置了 `base_url` 时优先使用渠道配置；仅当渠道 `base_url` 为空时才使用此处值。

#### transport

```text
Syntax:  transport http|aws_sdk;
Default: http
Context: upstream_config
Multiple: yes（最后一条生效）
```

- 声明上游传输方式。
- `http` 使用普通 HTTP `base_url + path/query` 路由。
- `aws_sdk` 使用 AWS SDK Bedrock Runtime transport。

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

- `<mode>` 可选：`openai|gemini|qwen|claude|iflow|antigravity|kimi|google_service_account_file|custom`。
- 含义：开启运行时 OAuth token 交换。
- `google_service_account_file` 会使用 Google service account JWT assertion 换取 access token。ONR 文件模式读取
  `credential_file`，嵌入式运行时可直接传入 credential JSON；优先使用 JSON 中的 `token_uri`，为空时默认
  `https://oauth2.googleapis.com/token`。

#### auth_oauth_bearer

```text
Syntax:  auth_oauth_bearer;
Default: —
Context: auth
Multiple: yes
```

- 效果：设置 `Authorization: Bearer <oauth.access_token>`。

#### auth_sigv4_bedrock

```text
Syntax:  auth_sigv4_bedrock;
Default: —
Context: auth
Multiple: yes
```

- 声明当前 provider 使用 AWS Bedrock SigV4 与 AWS SDK transport。
- 不向普通 HTTP header 注入 `$channel.key`。
- 不能和 `auth_bearer`、`auth_header_key`、`auth_oauth_bearer` 或 OAuth token 交换指令混用。

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
- `<expr>` 支持内置变量、`concat(...)`、`template(...)` 等字符串表达式。

#### del_header

```text
Syntax:  del_header <Header-Name>;
Default: —
Context: request
Multiple: yes
```

- 支持多条；按顺序执行。
- 合并规则：`defaults` 的操作先执行，`match` 的操作追加在后执行。

#### pass_header

```text
Syntax:  pass_header <Header-Name>;
Default: —
Context: request
Multiple: yes
```

- 将原始客户端请求中的某个 header 复制到上游请求。
- 如果源 header 不存在，则为 no-op。

#### filter_header_values

```text
Syntax:  filter_header_values <Header-Name> <pattern>... [separator="<sep>"];
Default: separator=","
Context: request
Multiple: yes
```

- 用于过滤上游请求 header 中的列表型值。
- 执行过程为：按 `separator` 分割，对每项做 trim，删除匹配项，再重组剩余项。
- 如果过滤后没有剩余项，则删除整个 header。
- 输出连接格式会被规范化：`,` 使用 `", "`，其他分隔符使用 `"<sep> "`。

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
Syntax:  json_set <jsonpath> <expr> [event="<name|name2>"] [event_optional=true|false] [max_count=<n>];
Default: —
Context: request/response
Multiple: yes
```

- 设置 JSON 字段；不存在的对象路径会自动创建。
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。
- `<expr>` 在此处支持：`true/false/null`、整数、字符串字面量、变量、`concat(...)`、`template(...)`。
- `event="..."` 仅在 `response` 的 SSE JSON 操作中有效，用于按 SSE `event:` 名过滤执行。
- `event_optional=true` 仅在 `response` 中有效，必须与 `event` 同时出现，并允许没有 event 上下文时继续执行。
- `max_count=<n>` 仅在 `response` 中有效；`0` 表示不限次数，`n > 0` 表示本指令在一次响应处理周期内最多实际修改 `n` 次。

#### json_replace

```text
Syntax:  json_replace <jsonpath> <expr> [event="<name|name2>"] [event_optional=true|false] [max_count=<n>];
Default: —
Context: request/response
Multiple: yes
```

- 仅当目标路径已存在时替换 JSON 字段；路径不存在时为 no-op。
- 不会创建缺失的父对象或叶子字段。
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。
- `<expr>` 支持同 `json_set`。
- `event="..."` 仅在 `response` 的 SSE JSON 操作中有效，用于按 SSE `event:` 名过滤执行。
- `event_optional=true` 仅在 `response` 中有效，必须与 `event` 同时出现，并允许没有 event 上下文时继续执行。
- `max_count=<n>` 仅在 `response` 中有效，语义同 `json_set`。

#### json_set_if_absent

```text
Syntax:  json_set_if_absent <jsonpath> <expr> [event="<name|name2>"] [event_optional=true|false] [max_count=<n>];
Default: —
Context: request/response
Multiple: yes
```

- 仅当路径不存在时设置字段。
- 若路径已存在（包括值为 `null`），则保留原值不覆盖。
- `event="..."` 仅在 `response` 的 SSE JSON 操作中有效。
- `event_optional=true` 仅在 `response` 中有效，必须与 `event` 同时出现，并允许没有 event 上下文时继续执行。
- `max_count=<n>` 仅在 `response` 中有效，语义同 `json_set`。

#### json_del

```text
Syntax:  json_del <jsonpath> [event="<name|name2>"] [event_optional=true|false] [max_count=<n>];
Default: —
Context: request/response
Multiple: yes
```

- 删除字段；字段不存在时为 no-op。
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。
- `event="..."` 仅在 `response` 的 SSE JSON 操作中有效。
- `event_optional=true` 仅在 `response` 中有效，必须与 `event` 同时出现，并允许没有 event 上下文时继续执行。
- `max_count=<n>` 仅在 `response` 中有效，语义同 `json_set`。

#### json_del_if_missing

```text
Syntax:  json_del_if_missing <target-jsonpath> <required-jsonpath>;
Default: —
Context: request
Multiple: yes
```

- 当 `<required-jsonpath>` 不存在时，删除 `<target-jsonpath>`。
- 如果目标路径不存在，则为 no-op。
- 两个 JSONPath 都仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。
- 适用于条件请求变换之后：只有依赖字段仍存在时，伴随字段才应保留。

示例：

```conf
request {
  after_req_map {
    json_del_with_condition "$.tools" "type" "web_search*" "web_fetch*";
    json_del_if_missing "$.tool_choice" "$.tools";
  }
}
```

#### json_rename

```text
Syntax:  json_rename <from-jsonpath> <to-jsonpath> [event="<name|name2>"] [event_optional=true|false] [max_count=<n>];
Default: —
Context: request/response
Multiple: yes
```

- 将字段从 `<from-jsonpath>` 移动到 `<to-jsonpath>`；源字段不存在时为 no-op。
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。
- `event="..."` 仅在 `response` 的 SSE JSON 操作中有效。
- `event_optional=true` 仅在 `response` 中有效，必须与 `event` 同时出现，并允许没有 event 上下文时继续执行。
- `max_count=<n>` 仅在 `response` 中有效，语义同 `json_set`。

#### json_wrap_input_text

```text
Syntax:  json_wrap_input_text <jsonpath>;
Default: —
Context: request
Multiple: yes
```

- 将 `<jsonpath>` 指向的字符串值包装成 OpenAI Responses `input` message 列表。
- 路径不存在时为 no-op；路径值已经是数组时为 no-op，避免二次包装。
- 路径值为对象、数字、布尔值或 `null` 时会报错，避免把异常请求静默转发给上游。
- JSONPath（v0.1）仅支持对象路径：`$.a.b.c`（不支持数组下标 `[]`）。

示例：

```conf
request {
  json_wrap_input_text "$.input";
}
```

输入：

```json
{
  "input": "Generate an image of gray tabby cat hugging an otter with an orange scarf"
}
```

输出：

```json
{
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "Generate an image of gray tabby cat hugging an otter with an orange scarf"
        }
      ]
    }
  ]
}
```

#### json_del_with_condition

```text
Syntax:  json_del_with_condition <jsonpath> <field> <pattern>...;
Default: —
Context: request
Multiple: yes
```

- 当 `<jsonpath>` 指向对象时，如果 `<field>` 匹配任一 pattern，则删除该对象。
- 当 `<jsonpath>` 指向数组时，删除数组中 `<field>` 匹配任一 pattern 的对象元素。
- 匹配不区分大小写，支持 `*` 通配符。
- 如果数组元素全部被删除，则移除该数组字段。

示例：

```conf
request {
  after_req_map {
    json_del_with_condition "$.tools" "type" "web_search*" "web_fetch*";
    json_del_with_condition "$.tool_choice" "type" "web_search*" "web_fetch*";
  }
}
```

### 7.6 upstream（路由）

#### set_path

```text
Syntax:  set_path <path-or-expr>;
Default: —
Context: upstream
Multiple: yes
```

- 设置上游请求路径（覆盖原 path）。
- `set_path` 支持 path 字符串字面量、`concat(...)` 与 `template(...)`。
- 当需要把变量嵌入 path 形态的字符串中时，推荐使用 `template(...)`：

```conf
set_path template("/openai/deployments/${request.model_mapped}/chat/completions");
```

- 配置的 path 表达式必须是 path 形态并以 `/` 开头；嵌入变量时请使用 `template(...)` 或 `concat(...)`。

#### set_query

```text
Syntax:  set_query <key> <expr>;
Default: —
Context: upstream
Multiple: yes
```

- 支持多条；同一 key 多次 set 时后者覆盖前者。
- `<expr>` 支持第 6 节中的表达式形态，包括 `template(...)`。

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

#### sse_collect

```text
Syntax:  sse_collect <mode>;
Default: —
Context: response
Multiple: yes
```

- 把上游 SSE 聚合成同协议非流式 JSON，再可选执行 `resp_map`。
- 支持的 `mode`：`openai_responses`、`anthropic_messages`、`gemini_generate_content`。
- 只允许用于 `stream = false` 的 match；不能和 `sse_parse` 或 `resp_passthrough` 组合。

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

#### usage_root

```text
Syntax:  usage_root path="$.usage" [event="a|b"] [event_optional=true] [exclude="field_a|field_b"];
Default: —
Context: metrics, usage_mode
Multiple: yes
```

- `path` 必填，必须以 `$.` 开头。
- `event` 可选，只对流式 SSE 提取有意义，支持用 `|` 配置多个事件名。
- `event_optional=true` 必须与 `event` 同时出现。
- `exclude` 可选，会在合并前从提取出的 usage 对象里删除顶层字段。
- 多条 `usage_root` 命中时会合并成一个 usage 对象。
- `usage_root` 不支持 `name`。

#### usage_fact

```text
Syntax:  usage_fact <dimension> <unit> path|count_path|sum_path|len_path|expr ...;
Default: —
Context: metrics
Multiple: yes
```

- 仅建议配合 `usage_extract custom;` 使用。
- 第一批支持固定 registry 维度，不开放任意自定义维度。
- 支持 `path` / `count_path` / `sum_path` / `len_path` / `expr` 五类原语(每条规则恰好一个)。
- `len_path` 读取路径处的字符串并以其 **rune 数**作为数量,用于按字符计费的维度(如 TTS 输入文本:`usage_fact input character source=request len_path="$.input";`)。
- `when_path="$.x" when_eq="v"` 为规则加条件门:仅当同一 source root 中 `when_path` 处的值等于 `when_eq`(数值或字符串比较)时规则才命中;路径缺失永不命中,因此可与 `fallback=true` 规则自然串联——例如按 OpenAI STT 的 `usage.type` 分支:

```conf
usage_fact audio.stt second path="$.seconds" when_path="$.type" when_eq="duration";
usage_fact audio.stt second path="$.input_token_details.audio_tokens" scale=0.048 when_path="$.type" when_eq="tokens" fallback=true;
```

- `count_path` 可搭配 `type` / `status` 过滤。
- `path` / `sum_path` / `expr` 中的 JSONPath 现在支持受限 filter 子集：
  - `$.items[?(@.field=="VALUE")].x`
- 支持常量属性，如 `attr.ttl="5m"` / `attr.ttl="1h"` / `attr.modality="image"`。
- `attr.modality` 对 `cache_read token` 有特殊含义，用于表达上游真实返回的分模态 cached token。支持值为 `text` / `image` / `audio` / `video`；OpenAI cache detail 使用 text/image/audio，Gemini cache detail 使用 text/image/audio/video。
- 同一 `dimension + unit` 可以出现多条规则；所有命中的非 fallback 规则会累计求和。
- `fallback=true` 表示在同一 `dimension + unit` 没有更具体事实时再生效。
- `scale=<正数>` 对提取出的数量做乘法缩放，用于单位换算；例如上游返回毫秒时：`usage_fact audio.tts second path="$.extra_info.audio_length" scale=0.001;`。缺省不缩放。
- `source` 在配置了 `usage_root` 时默认是 `usage`，否则默认是 `response`；当前支持 `usage` / `response` / `request` / `derived`。
- `usage` / `response` / `request` / `derived` 之外的其他 `source` 会在校验阶段直接报错。
- `dimension` 是扁平字符串键，`.` 只是名称的一部分，不表示嵌套。
- 实时多模态的 meter-based facts（例如 `input.audio second`）属于计划中的目标能力；若文档其他位置出现相关示例，应视为预览写法，不代表当前 registry 已开放。
- `path` / `count_path` / `sum_path` / `expr` 既可以用双引号，也可以用单引号包裹。
- 当使用双引号字符串时，filter 内部的双引号需要写成 `\"`。
- 当使用单引号字符串时，可以直接写：
  - `path='$.usageMetadata.promptTokensDetails[?(@.modality=="AUDIO")].tokenCount'`
- 当前支持的 `dimension`：
  - `input`
  - `output`
  - `input.image`
  - `input.video`
  - `input.audio`
  - `output.image`
  - `output.audio`
  - `output.video`
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
- 当前固定 registry 包括：
  - `input token`
  - `output token`
  - `input.image token`
  - `input.video token`
  - `input.audio token`
  - `output.image token`
  - `output.audio token`
  - `output.video token`
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
  - `input character`
  - `output character`

#### finish_reason_extract

```text
Syntax:  finish_reason_extract <mode>;
Default: —
Context: metrics
Multiple: no
```

- 目前支持：`openai` / `anthropic` / `gemini` / `custom`。
- 内置语义：
  - `openai`：
    - `chat.completions` / `completions`：`choices[*].finish_reason`
    - `responses` 非流式：`incomplete_details.reason`
    - `responses` 流式：`response.incomplete_details.reason`
  - `anthropic`：按 `stop_reason -> delta.stop_reason -> message.stop_reason`
  - `gemini`：按 `candidates[*].finishReason -> candidates[*].finish_reason`
- `custom` 现在可以通过多条 `finish_reason_path` 复刻有序 fallback；只要是纯路径查找类场景，就可以完整替代内置模式。

#### finish_reason_path

```text
Syntax:  finish_reason_path <jsonpath> [fallback=true|false] [event="<name>"] [event_optional=true|false];
Default: —
Context: metrics
Multiple: yes
```

- 可选覆盖项；当使用 `finish_reason_extract custom;` 时为必填。
- 支持在路径后追加 `fallback=true|false`、`event="<name>"`、`event_optional=true|false` 元数据。
- `event` 适用于 SSE 按事件名筛选的场景；命中事件时才会尝试这条 `finish_reason_path`。
- `event_optional=true` 仅可和 `event` 一起使用，表示当运行时没有提供事件名上下文时，仍允许这条规则按普通 JSON 匹配回退执行。
- JSONPath 子集：`$.a.b.c` / `$.items[0].x` / `$.items[*].x` / `$.items[?(@.field=="VALUE")].x`（`[*]` 或 filter 命中多项时取第一个非空字符串）。

#### input_tokens_expr

```text
Syntax:  input_tokens_expr = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 这是兼容层写法：执行前会被编译成等价的内部 fact-based 规则。
- `<expr>` 为受限表达式：只允许 `+/-`、JSONPath、整数常量；不支持括号/乘除/函数。
- JSONPath 子集与 `*_tokens_path` 相同：`$.a.b.c` / `$.items[0].x` / `$.items[*].x` / `$.items[?(@.field=="VALUE")].x`（`[*]` 或 filter 命中多项时对数组求和）。
- 取不到/非数值按 `0` 处理。

#### output_tokens_expr

```text
Syntax:  output_tokens_expr = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- 规则同 `input_tokens_expr`。

#### cache_read_tokens_expr

```text
Syntax:  cache_read_tokens_expr = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- 规则同 `input_tokens_expr`。

#### cache_write_tokens_expr

```text
Syntax:  cache_write_tokens_expr = <expr>;
Default: —
Context: metrics
Multiple: yes
```

- 规则同 `input_tokens_expr`。

#### total_tokens_expr

```text
Syntax:  total_tokens_expr = <expr>;
Default: input_tokens_expr + output_tokens_expr
Context: metrics
Multiple: yes
```

- 规则同 `input_tokens_expr`。
- 若未显式声明，则默认按 `input_tokens_expr + output_tokens_expr` 计算。
- 不推荐：显式配置 `total_tokens_expr` 会额外引入一个独立的 total 事实源，容易与由 `input/output` 聚合出的 total 产生口径分歧。
- 也是兼容层写法：执行前会被编译到统一的内部 usage plan。

#### input_tokens_path

```text
Syntax:  input_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 等价于 `input_tokens_expr = <jsonpath>;` 的简写（仅能写单个 JSONPath，不支持加减）。
- 同样属于兼容层写法：执行前会被编译成等价的内部 fact-based 规则。

#### output_tokens_path

```text
Syntax:  output_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 等价于 `output_tokens_expr = <jsonpath>;` 的简写（仅能写单个 JSONPath，不支持加减）。
- 同样属于兼容层写法：执行前会被编译成等价的内部 fact-based 规则。

#### cache_read_tokens_path

```text
Syntax:  cache_read_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 等价于 `cache_read_tokens_expr = <jsonpath>;` 的简写（仅能写单个 JSONPath，不支持加减）。
- 同样属于兼容层写法：执行前会被编译成等价的内部 fact-based 规则。

#### cache_write_tokens_path

```text
Syntax:  cache_write_tokens_path <jsonpath>;
Default: —
Context: metrics
Multiple: yes
```

- 建议仅配合 `usage_extract custom;` 使用；同字段多次出现时后者覆盖前者。
- 等价于 `cache_write_tokens_expr = <jsonpath>;` 的简写（仅能写单个 JSONPath，不支持加减）。
- 同样属于兼容层写法：执行前会被编译成等价的内部 fact-based 规则。

### 7.10 balance（上游余额查询）

#### balance_mode

```text
Syntax:  balance_mode <mode>;
Default: —
Context: balance
Multiple: no
```

- 支持：`openai` / `custom`。
- 其他任意 mode 名：会先解析为顶层全局 `balance_mode` 预设，再统一执行。
- 在 `balance` 里，如果省略 `balance_mode`，但已经声明了 `path`、`balance_path`、`balance_expr`、`used_path`、`used_expr` 等自定义查询字段，ONR 会自动按 `balance_mode custom;` 处理。

#### method

```text
Syntax:  method <GET|POST>;
Default: GET
Context: balance
Multiple: yes
```

#### path

```text
Syntax:  path <path-or-url-or-template>;
Default: —
Context: balance
Multiple: yes
```

- `balance_mode custom` 时必填。
- 支持绝对 URL、相对 provider `base_url` 的路径，以及 `template("...")`；模板字面量必须以 `/`、`http://` 或 `https://` 开头。

#### balance_expr / used_expr

```text
Syntax:  balance_expr = <expr>;
Syntax:  used_expr = <expr>;
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

- 在 custom 模式下，如果未设置 `balance_expr`，则 `balance_path` 必填。

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

- header 值是字符串表达式，支持 `template(...)`。

#### subscription_path / usage_path

```text
Syntax:  subscription_path <path-or-url-or-template>;
Syntax:  usage_path <path-or-url-or-template>;
Default: OpenAI dashboard 默认路径
Context: balance
Multiple: yes
```

- `balance_mode openai` 的可选覆盖项。
- 可使用 `template("...")`；模板字面量必须以 `/`、`http://` 或 `https://` 开头。

### 7.11 models（上游模型列表查询）

#### models_mode

```text
Syntax:  models_mode <mode>;
Default: —
Context: models
Multiple: no
```

- 支持：`openai` / `gemini` / `custom`
- 其他任意 mode 名：会先解析为顶层全局 `models_mode` 预设，再统一执行

#### method

```text
Syntax:  method <GET|POST>;
Default: GET
Context: models
Multiple: yes
```

#### path

```text
Syntax:  path <path-or-url-or-template>;
Default: 与 mode 相关
Context: models
Multiple: yes
```

- `models_mode openai`：默认 `/v1/models`
- `models_mode gemini`：默认 `/v1beta/models`
- `models_mode bedrock`：默认使用 `config/modes/models_modes.conf` 中的 `/foundation-models`
- 默认 `config/modes/models_modes.conf` 里的 `vertex` 预设会查询 Google publisher base models：
  - `path /v1beta1/publishers/google/models`
  - `id_path $.publisherModels[*].name`
  - `id_regex ^publishers/google/models/(.+)$`
- 默认 `config/modes/models_modes.conf` 里的 `bedrock` 预设会查询 AWS Bedrock foundation models：
  - `path /foundation-models`
  - `id_path $.modelSummaries[*].modelId`
  - 当 provider 使用 `transport aws_sdk` 时，ONR 会查询 Bedrock control-plane endpoint `https://bedrock.<region>.amazonaws.com`，并使用 SigV4 service `bedrock` 签名
- `models_mode custom`：必填
- `path` 可以使用 `template("...")`；模板字面量必须以 `/`、`http://` 或 `https://` 开头。
- 如果省略 `models_mode`，但已经声明了 `path`、`id_path`、`id_regex`、`id_allow_regex` 等自定义查询字段，ONR 会自动按 `models_mode custom;` 处理。

#### id_path

```text
Syntax:  id_path <jsonpath>;
Default: 与 mode 相关
Context: models
Multiple: yes
```

- `models_mode openai`：默认 `$.data[*].id`
- `models_mode gemini`：默认 `$.models[*].name`
- `models_mode bedrock`：默认使用 `config/modes/models_modes.conf` 中的 `$.modelSummaries[*].modelId`
- `models_mode custom`：至少需要一个 `id_path`

#### id_regex / id_allow_regex

```text
Syntax:  id_regex <regex>;
Syntax:  id_allow_regex <regex>;
Default: —
Context: models
Multiple: yes
```

- `id_regex` 用于重写模型 ID（优先取第 1 个捕获组）
- `id_allow_regex` 用于重写后的白名单过滤

#### set_header / del_header

```text
Syntax:  set_header <Header-Name> <expr>;
Syntax:  del_header <Header-Name>;
Default: —
Context: models
Multiple: yes
```

- header 值是字符串表达式，支持 `template(...)`。

---

## 8. 内置变量参考

本节列出 v0.1 可在 `<expr>` 中使用的内置变量（均为字符串）。

> 说明：变量在运行期求值；当某变量在当前请求上下文中为空时，会展开为空字符串。

### 8.1 `$channel.*`

`$channel.base_url`

渠道（DB）配置的 `base_url`。在 `upstream_config { base_url = "..."; }` 的默认行为中会作为“渠道优先”来源：渠道非空则优先用渠道。

`$channel.key`

渠道（DB）配置的 `key`（鉴权 token）。`auth_bearer;` 与 `auth_header_key ...;` 会固定使用该值作为 token/value。

`$channel.location`

Provider location。对 Vertex AI 通常是 `global` 或 `us-central1` 这类区域。

### 8.2 `$credential.*`

`$credential.project_id`

从当前 credential 解析出的 project id。对 Google service account 来说，对应 JSON 中的 `project_id`。

### 8.3 `$request.*`

`$request.model`

来自客户端请求的模型名。

`$request.model_mapped`

映射后的模型名。默认等于 `$request.model`；可通过 `model_map` 与 `model_map_default` 修改。

### 8.4 `$task.*`

`$task.upstream_id`

长任务查询路由中的上游任务/operation id。对 Gemini Veo 来说，它是 `predictLongRunning`
返回的 operation name，例如 `models/veo-3.1-generate-preview/operations/abc`。

### 8.5 使用示例

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

  # 示例：等价的 path 模板写法
  set_path template("/v1/${request.model_mapped}/chat/completions");

  # 示例：使用 credential 与 location 元数据拼接 Vertex AI path
  set_path template("/v1/projects/${credential.project_id}/locations/${channel.location}/publishers/google/models/${request.model_mapped}:generateContent");

  # 示例：Gemini 长任务 operation 查询路径
  set_path concat("/v1beta/", $task.upstream_id);
}
```
