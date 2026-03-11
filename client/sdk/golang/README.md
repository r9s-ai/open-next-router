# ONR Go SDK CLI

Unified Go SDK and CLI for testing OpenAI, Anthropic, and Gemini providers through ONR.

## Layout

- `cmd/onr-sdk` -> CLI entrypoint
- `pkg/openai` -> `/v1/chat/completions`, `/v1/responses`, `/v1/embeddings`
- `pkg/anthropic` -> `/v1/messages`
- `pkg/gemini` -> Gemini models and chats

## Installation

```bash
cd client/sdk/golang
go mod tidy
go build ./cmd/onr-sdk
```

From the repository root, you can also run:

```bash
go run ./client/sdk/golang/cmd/onr-sdk --help
```

## Environment Variables

```bash
export ONR_API_KEY="your-api-key"
export ONR_BASE_URL="http://localhost:3300"
```

## CLI Usage

```bash
go run ./cmd/onr-sdk openai chat_completions "Hello"
go run ./cmd/onr-sdk openai responses "Hello"
go run ./cmd/onr-sdk openai embeddings "hello embeddings"
go run ./cmd/onr-sdk anthropic messages "Hello"
go run ./cmd/onr-sdk gemini models "Hello"
go run ./cmd/onr-sdk gemini chats "Draw a guitar"
```

If you are in the repository root, use:

```bash
go run ./client/sdk/golang/cmd/onr-sdk openai chat_completions "Hello"
```

### Quick Examples

```bash
go run ./cmd/onr-sdk openai chat_completions "Hello" --stream -v
go run ./cmd/onr-sdk openai responses "Tell me a joke" --stream -v
go run ./cmd/onr-sdk openai embeddings "hello embeddings" -v
go run ./cmd/onr-sdk anthropic messages "Hello" --stream -v
go run ./cmd/onr-sdk gemini models "Hello" --stream -v
go run ./cmd/onr-sdk gemini chats "Draw a guitar" --response_modalities TEXT,IMAGE --image-output-dir . -v
```

## Default Models

- `openai chat_completions` -> `gpt-4o-mini`
- `openai responses` -> `gpt-4o-mini`
- `openai embeddings` -> `text-embedding-3-small`
- `anthropic messages` -> `claude-haiku-4-5`
- `gemini models` -> `gemini-2.5-flash`
- `gemini chats` -> `gemini-3-pro-image-preview`

## Completion

```bash
go run ./cmd/onr-sdk completion --shell zsh
go run ./cmd/onr-sdk completion --shell bash
go run ./cmd/onr-sdk completion --shell fish
```

## Go Examples

### OpenAI Chat

```go
cfg, err := config.FromEnv()
if err != nil {
	panic(err)
}

text, err := openai.Chat(context.Background(), cfg, "Hello", "gpt-4o-mini")
if err != nil {
	panic(err)
}
fmt.Println(text)
```

### OpenAI Responses Stream

```go
cfg, _ := config.FromEnv()
chunks, errc := openai.StreamResponse(context.Background(), cfg, "Hello", "gpt-4o-mini")
for chunk := range chunks {
	fmt.Print(chunk)
}
if err := <-errc; err != nil {
	panic(err)
}
```

### Anthropic Stream

```go
cfg, _ := config.FromEnv()
chunks, errc := anthropic.StreamChat(context.Background(), cfg, "Hello", "claude-haiku-4-5", 1024)
for chunk := range chunks {
	fmt.Print(chunk)
}
if err := <-errc; err != nil {
	panic(err)
}
```

### Gemini Chats Multimodal

```go
cfg, _ := config.FromEnv()
events, errc := gemini.StreamChatMultimodal(
	context.Background(),
	cfg,
	"Draw a guitar",
	"gemini-3-pro-image-preview",
	[]string{"TEXT", "IMAGE"},
)
for event := range events {
	switch event.Type {
	case "text":
		fmt.Print(event.Text)
	case "image":
		fmt.Println(len(event.ImageBytes))
	}
}
if err := <-errc; err != nil {
	panic(err)
}
```

## Notes

This SDK is intentionally thin. It wraps official Go SDKs and keeps behavior explicit for ONR endpoint testing.
