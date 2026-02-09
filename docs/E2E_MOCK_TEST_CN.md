# ONR 代理层 Mock E2E 测试设计

## 目标

- 固化 `internal/proxy` 的端到端转换行为，覆盖 `req_map`、路由、`resp_map`、`sse_parse`。
- 测试不访问真实上游，不消耗 API 费用。
- 测试不依赖仓库中的 provider 配置文件，避免配置漂移导致用例失真。

## 范围

- 测试入口：`open-next-router/internal/proxy/e2e_mock_test.go`
- 重点验证：
1. 请求是否被正确改写并路由到目标上游路径。
2. 响应是否被正确转换为 OpenAI 兼容格式。
3. SSE 终止事件 `data: [DONE]` 的顺序与唯一性。

## 架构

1. 下游请求模拟
- 使用 `gin.CreateTestContext` + `httptest.NewRecorder` 模拟客户端请求 ONR。
- 通过 `Client.ProxyJSON(...)` 直接进入代理链路，不启动独立 onr 进程。

2. 上游服务 Mock
- 使用 `httptest.Server` 模拟 provider 上游接口。
- 按测试场景返回 JSON 或 SSE 文本。
- 在 handler 中断言上游路径、query、关键请求字段。

3. DSL 配置隔离
- 测试内联构造最小 provider DSL 字符串（`providerConf*`）。
- 在 `t.TempDir()` 动态写入 `.conf` 后用 `dslconfig.Registry.ReloadFromDir()` 加载。
- 每个测试仅加载本场景需要的 provider，避免受全局配置影响。

## 数据组织

目录：

- `open-next-router/internal/proxy/testdata/fixtures/`
- `open-next-router/internal/proxy/testdata/mock_upstream/`

约定：

1. `fixtures/` 存放下游请求体（输入样本）。
2. `mock_upstream/` 存放上游返回体（JSON/SSE 原文）。
3. 文件命名按 `provider/场景名` 组织，便于横向扩展。

## 已落地场景

1. Anthropic `/v1/messages` 非流
- OpenAI chat request -> Anthropic messages request
- Anthropic JSON -> OpenAI chat completion JSON

2. Anthropic `/v1/messages` 流式 tool_use
- Anthropic SSE tool_use 事件 -> OpenAI `tool_calls` chunk
- 验证 `finish_reason=tool_calls` 与单个 `[DONE]`

3. Gemini 流式
- OpenAI chat request -> Gemini `:streamGenerateContent?alt=sse`
- Gemini SSE -> OpenAI chat chunks（含 usage chunk）

4. Azure Responses 流式（completed 先到）
- OpenAI chat request -> `/openai/responses?api-version=2025-04-01-preview`
- 验证 `[DONE]` 不会早于后续 token

## 运行方式

在 `open-next-router` 目录执行：

```bash
go test ./internal/proxy -run E2EMock -v
```

或执行完整代理层测试：

```bash
go test ./internal/proxy
```

## Golden 基线

- e2e 用例同时做关键字段断言和 golden 全量输出对比。
- golden 文件位置：`open-next-router/internal/proxy/testdata/golden/`
- 为避免时间戳/随机 id 导致波动，测试会先标准化动态字段（如 `created`、`chatcmpl_*`）再比较。

更新 golden（仅在你确认行为变更时）：

```bash
cd open-next-router
UPDATE_GOLDEN=1 go test ./internal/proxy -run E2EMock -v
```

更新后请再执行一次普通回归，确认基线一致：

```bash
go test ./internal/proxy -run E2EMock -v
```

## 扩展规范

新增场景建议按以下步骤：

1. 在 `testdata/fixtures` 添加下游请求样本。
2. 在 `testdata/mock_upstream` 添加上游返回样本。
3. 新增 `TestE2EMock_*` 用例：
- 构造 `httptest.Server`。
- 断言上游请求路径/参数。
- 断言下游输出结构和关键字段。
4. 如涉及新 provider 映射，添加对应 `providerConf*` 最小 DSL 片段。

## 非目标

- 不覆盖真实网络稳定性、账号权限、上游限流等线上因素。
- 不替代真实联调，只保证代理映射与路由逻辑的确定性回归。
