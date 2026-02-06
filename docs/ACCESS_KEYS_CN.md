# Access Keys 与 Token Key（onr:v1?）方案说明

本文档描述 open-next-router (ONR) 当前的「访问 Key」与「上游 Key」统一管理方案，以及客户端仅能配置单一 Key 时的请求格式。

## 1. keys.yaml：同时管理两类 Key

`keys.yaml` 包含两个顶层区块：

1) `providers:`：**上游 Key 池**（用于 ONR 调用各 provider 上游；原有结构不变）

2) `access_keys:`：**访问 Key 池**（用于客户端访问 ONR 的鉴权；新增结构）

示例：

```yaml
providers:
  openai:
    keys:
      - name: "key1"
        value: "sk-xxxx"

access_keys:
  - name: "client-a"
    value: "ak-xxx"
    comment: "iOS app"
  - name: "client-b"
    value: "ENC[v1:aesgcm:...]"
    disabled: false
```

### 1.1 access_keys 字段

- `name`：建议填写，用于运维识别与环境变量覆盖名
- `value`：访问 key（支持明文或 `ENC[...]` 加密值）
- `disabled`：可选；为 `true` 时该 key 不参与鉴权匹配
- `comment`：可选备注

### 1.2 环境变量覆盖

上游 key（providers）覆盖规则保持不变：

- 有 `name`：`ONR_UPSTREAM_KEY_<PROVIDER>_<NAME>`
- 无 `name`：`ONR_UPSTREAM_KEY_<PROVIDER>_<INDEX>`（1-based）

访问 key（access_keys）覆盖规则：

- 有 `name`：`ONR_ACCESS_KEY_<NAME>`（例：`client-a` -> `ONR_ACCESS_KEY_CLIENT_A`）
- 无 `name`：`ONR_ACCESS_KEY_<INDEX>`（1-based）

### 1.3 ENC[...] 加密

`providers.*.keys[].value` 与 `access_keys[].value` 都支持：

`ENC[v1:aesgcm:<base64(nonce+ciphertext)>]`

解密需要设置 `ONR_MASTER_KEY`。

## 2. 鉴权：两种方式

### 2.1 Legacy master key（兼容）

请求头：

- `Authorization: Bearer <ONR_API_KEY>`
- `x-api-key: <ONR_API_KEY>`
- `x-goog-api-key: <ONR_API_KEY>`

### 2.2 Token Key（onr:v1?，可编辑，无 sig）

适用于「客户端只能配置一个 key，不能加额外头」的场景。

请求头：

- `Authorization: Bearer onr:v1?...`

Token Key 格式：

- `onr:v1?k=<ACCESS_KEY>&...`
- `onr:v1?k64=<base64url(ACCESS_KEY)>&...`（推荐，默认）

其中 `ACCESS_KEY` 必须匹配 `keys.yaml` 的某个 `access_keys[].value`（或其 env override/解密后的值）。

## 3. Token Key 支持的参数（都在一个 key 里）

Token Key 的 query 参数：

- `k` / `k64`：访问 key（必需）
- `p`：provider（可选；不填则走原有 provider 选择逻辑）
- `m`：model override（可选；存在则强制替换请求里的 model；BYOK 模式下也生效）
- `uk`：BYOK upstream key（可选；存在则 ONR 直接用该 key 调上游）
- `exp`：可选；unix 秒时间戳；过期后拒绝

示例：

- 指定 provider：`onr:v1?k64=...&p=openai`
- 强制模型：`onr:v1?k64=...&m=gpt-4o-mini`
- BYOK + provider + 强制模型：`onr:v1?k64=...&p=openai&uk=sk-xxx&m=gpt-4o-mini`

## 4. onr-admin 交互式管理

`onr-admin` 支持管理 `providers` / `access_keys` 并生成 Token Key：

```bash
go run ./cmd/onr-admin --config ./onr.yaml
# 或覆盖 keys.yaml 路径
go run ./cmd/onr-admin --keys ./keys.yaml
```
