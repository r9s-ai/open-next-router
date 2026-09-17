# JavaScript 阶段扩展

本次实现覆盖计划阶段一至三：DSL 与工具链、onr-core 共用执行器、ONR 接入。Relay 接入、编辑器独立发布及 onr-core 发版属于后续工作。

完整契约、宿主 YAML、HTTP 审核示例、时序图及复用 API 见 [英文实现说明](JS_EXTENSIONS.md)；中文指令入口见 [DSL 参考](../DSL_SYNTAX_CN.md#9-javascript-阶段扩展)。

- [可执行 provider 示例](../onr/internal/proxy/testdata/js/provider.conf)：前置正文处理、后置文件脚本、响应头、JSON、SSE 和 log 六个阶段。
- [文件脚本示例](../onr/internal/proxy/testdata/js/after-map.js)：验证后置 JSON 操作已完成，并读取同一次尝试的 ctx.state。
- [ONR 集成测试](../onr/internal/proxy/js_integration_test.go)：真实 mock 上游，覆盖拒绝、usage 保留、长流固定版本、OAuth 重试和实际响应格式分派。

`request` 和 `request.after_req_map` 均使用 `request_by_js_block`。父 block 决定执行时机，不增加后置专用指令。即使内层 JS 写在 json_set 前，也在所有后置 JSON 操作之后运行；没有 req_map 时仍运行。

每次尝试创建独立 VM，多个阶段共享 state；无有效脚本时不创建 VM。OAuth 401 刷新从原始基线创建新尝试，attempt 递增、state 重置，仍使用同一配置快照。脚本错误和显式拒绝不重试；log 使用独立清理超时且恰好执行一次。响应已提交后出错仅关闭流，不追加 JSON。下游脚本改写和失败都不覆盖已采集的计费事实。

宿主 `js.root` 下的文件在准备配置时读取，使用抵御路径穿越和符号链接逃逸的文件打开机制；请求期间不读脚本文件。`js.reload` 支持 off/watch/poll，poll 每秒检查一次。所有触发都完整准备 DSL、modes、JS 及 HTTP 授权，任一失败保留旧快照。重载会重新读取 YAML 的 HTTP 授权；修改 root/reload 模式需要重启，以保持监听目录一致。

HTTP 仅开放给请求阶段和非流式正文阶段，按 provider 授权精确 origin。私网、链路本地/保留地址、重定向、环境代理和凭证转发均禁止。阶段超时、宿主最大时限和调用时限取最小值，取消传播到底层连接。

正文 hook 只处理 JSON，不能静默跳过 multipart/二进制。响应上限在解压后检查；ONR 原有入口不支持压缩请求 JSON，本功能未扩大该能力。SSE 按完整事件执行，保留多行 data、注释和心跳，保护选定协议的终止事件，不提供结束时补发能力。

脚本属于可信运维配置。Goja 中断不是操作系统级隔离：原生引擎操作不能被硬抢占，VM 堆也没有硬内存配额；脚本必须自行限制 state 的增长。正文/事件上限不等于任意 JavaScript 内存限制。

共享 dsllang 已支持语法岛、JS 诊断、指令元数据及代码块原样格式化；本次未发布独立编辑器扩展。打包保留文件引用，并通过普通 `# onr-js-origin:` 注释保留内联脚本源位置。`onr -t` 会结合宿主配置编译文件脚本；单独 `onr-pack --check-only` 只验证 DSL 和内联脚本，不执行任何脚本或外部 HTTP 请求。
