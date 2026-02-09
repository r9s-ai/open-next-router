# onr-admin 命令使用说明

本文档集中说明 `onr-admin` 的 CLI 用法，避免分散在多个文档中重复维护。

## 1. 基本用法

```bash
go run ./cmd/onr-admin <subcommand> [flags]
```

查看帮助：

```bash
go run ./cmd/onr-admin --help
go run ./cmd/onr-admin <subcommand> --help
```

## 2. token

用于生成可放入 `Authorization: Bearer ...` 的 `onr:v1?...` Token Key。

```bash
# 生成 Token Key
go run ./cmd/onr-admin token create --config ./onr.yaml --access-key-name client-a -p openai -m gpt-4o-mini
```

## 3. crypto

用于密钥加密与主密钥生成。

```bash
# 生成 ENC[...] 加密值
go run ./cmd/onr-admin crypto encrypt --text 'sk-xxxx'

# 一键加密 keys.yaml 中的明文 value
go run ./cmd/onr-admin crypto encrypt-keys --config ./onr.yaml

# 生成随机 ONR_MASTER_KEY（base64）
go run ./cmd/onr-admin crypto gen-master-key --export
```

## 4. validate

用于校验配置文件。

```bash
# 校验 keys/models/providers
go run ./cmd/onr-admin validate all --config ./onr.yaml
```

## 5. balance

按 providers DSL 查询上游余额。

```bash
go run ./cmd/onr-admin balance get --config ./onr.yaml -p openai
go run ./cmd/onr-admin balance get --config ./onr.yaml --providers openai,openrouter,deepseek
go run ./cmd/onr-admin balance get --config ./onr.yaml --all
go run ./cmd/onr-admin balance get --config ./onr.yaml -p moonshot --debug
```

## 6. pricing

从 `https://models.dev/api.json` 同步模型价格到 `price.yaml`。

```bash
# 列出 models.dev provider
go run ./cmd/onr-admin pricing providers
go run ./cmd/onr-admin pricing providers --search openai

# 不传 --provider/--providers 时，默认读取 onr.yaml 中 providers.dir 下的全部 provider
go run ./cmd/onr-admin pricing sync --config ./onr.yaml --out ./price.yaml

go run ./cmd/onr-admin pricing sync -p openai --models gpt-4o-mini,gpt-4o --out ./price.yaml
go run ./cmd/onr-admin pricing sync --providers openai,anthropic --out ./price.yaml

# provider 别名示例：gemini 会自动映射到 models.dev 的 google
go run ./cmd/onr-admin pricing sync -p gemini --models gemini-2.5-flash --out ./price.yaml
```

## 7. tui

打开交互式管理界面。

```bash
go run ./cmd/onr-admin tui --config ./onr.yaml
```
