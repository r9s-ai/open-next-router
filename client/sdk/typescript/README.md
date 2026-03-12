# ONR TypeScript SDK CLI

Unified TypeScript SDK and CLI for testing OpenAI, Anthropic, and Gemini providers through ONR.

## Layout

- `src/openai/chat_completions.ts` -> `/v1/chat/completions`
- `src/openai/responses.ts` -> `/v1/responses`
- `src/openai/embeddings.ts` -> `/v1/embeddings`
- `src/anthropic/messages.ts` -> `/v1/messages`
- `src/gemini/models.ts` -> Gemini models endpoints
- `src/gemini/chats.ts` -> Gemini chats streaming
- `src/cli.ts` -> CLI entrypoint

## Installation

```bash
cd client/sdk/typescript
npm install
npm run build
```

For local usage in this repository, do one of the following after `npm install` and `npm run build`:

```bash
# From client/sdk/typescript
node dist/cli.cjs openai chat_completions "Hello"
npm exec onr-sdk-ts -- openai chat_completions "Hello"

# From the repository root
node client/sdk/typescript/dist/cli.cjs openai chat_completions "Hello"
npm --prefix client/sdk/typescript exec onr-sdk-ts -- openai chat_completions "Hello"
```

## Environment Variables

```bash
export ONR_API_KEY="your-api-key"
export ONR_BASE_URL="http://localhost:3300"
```

## CLI Usage

```bash
node dist/cli.cjs openai chat_completions "Hello"
node dist/cli.cjs openai responses "Hello"
node dist/cli.cjs openai embeddings "hello embeddings"
node dist/cli.cjs anthropic messages "Hello"
node dist/cli.cjs gemini models "Hello"
node dist/cli.cjs gemini chats "Draw a guitar"
```

### Quick Examples

```bash
npm exec onr-sdk-ts -- openai chat_completions "Hello" --stream -v
npm exec onr-sdk-ts -- openai responses "Tell me a joke" --stream -v
npm exec onr-sdk-ts -- openai embeddings "hello embeddings" -v
npm exec onr-sdk-ts -- anthropic messages "Hello" --stream -v
npm exec onr-sdk-ts -- gemini models "Hello" --stream -v
npm exec onr-sdk-ts -- gemini chats "Draw a guitar" --response_modalities TEXT,IMAGE --image-output-dir . -v
```

From the repository root, use:

```bash
npm --prefix client/sdk/typescript exec onr-sdk-ts -- openai chat_completions "Hello" --stream -v
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
node dist/cli.cjs completion --shell zsh
node dist/cli.cjs completion --shell bash
node dist/cli.cjs completion --shell fish
```

## SDK Examples

### OpenAI Chat

```ts
import { createConfigFromEnv, openAIChat } from "@r9s-ai/onr-sdk-typescript";

const config = createConfigFromEnv();
const text = await openAIChat(config, "Hello", "gpt-4o-mini");
console.log(text);
```

### OpenAI Responses Stream

```ts
import { createConfigFromEnv, openAIStreamResponse } from "@r9s-ai/onr-sdk-typescript";

const config = createConfigFromEnv();
for await (const chunk of openAIStreamResponse(config, "Hello", "gpt-4o-mini")) {
  process.stdout.write(chunk);
}
process.stdout.write("\n");
```

### Anthropic Stream

```ts
import { anthropicStreamChat, createConfigFromEnv } from "@r9s-ai/onr-sdk-typescript";

const config = createConfigFromEnv();
for await (const chunk of anthropicStreamChat(config, "Hello", "claude-haiku-4-5")) {
  process.stdout.write(chunk);
}
process.stdout.write("\n");
```

### Gemini Chats Multimodal

```ts
import { createConfigFromEnv, streamChatMultimodal } from "@r9s-ai/onr-sdk-typescript";

const config = createConfigFromEnv();
for await (const event of streamChatMultimodal(
  config,
  "Draw a guitar",
  "gemini-3-pro-image-preview",
  ["TEXT", "IMAGE"],
)) {
  if (event.type === "text") {
    process.stdout.write(event.text);
    continue;
  }
  console.log(event.imageMimeType, event.imageBytes.length);
}
```

## Notes

This SDK is intentionally thin. It wraps official TypeScript SDKs and keeps behavior explicit for ONR endpoint testing.
