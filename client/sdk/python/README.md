# ONR Python SDK CLI

Unified CLI for testing OpenAI, Anthropic, and Gemini providers.

## Endpoint-Based File Layout

Provider files are organized by endpoint capability (one file per endpoint family):

- `openai/chat_completions.py` -> `/v1/chat/completions`
- `openai/embeddings.py` -> `/v1/embeddings`
- `openai/responses.py` -> `/v1/responses`
- `anthropic/messages.py` -> `/v1/messages`
- `gemini/models.py` -> `models.generate_content` / `models.generate_content_stream` / chats streaming

When adding a new endpoint, add a new file in the provider directory.
Example: add `openai/embeddings.py` for embeddings calls.

## Installation

```bash
pip install -r requirements.txt
```

## Environment Variables

```bash
export ONR_API_KEY="your-api-key"
export ONR_BASE_URL="https://api.r9s.ai"  # Optional, default: localhost:3300
```

## Usage

```bash
python cli.py openai chat_completions "<prompt>" [options]
python cli.py openai responses "<prompt>" [options]
python cli.py openai embeddings "<text>" [options]
python cli.py anthropic messages "<prompt>" [options]
python cli.py gemini models "<prompt>" [options]
python cli.py gemini chats "<prompt>" [options]
```

### Quick Examples

```bash
# OpenAI
python cli.py openai chat_completions "Hello"
python cli.py openai chat_completions "Hello" --stream -v
python cli.py openai responses "Write a one-line summary of HTTP" -v
python cli.py openai responses "Tell me a joke" --stream -v
python cli.py openai embeddings "hello embeddings" -v

# Anthropic
python cli.py anthropic messages "Hello" --model claude-haiku-4-5
python cli.py anthropic messages "Hello" --stream -v

# Gemini
python cli.py gemini models "Hello" --model gemini-2.5-flash
python cli.py gemini models "Hello" --stream -v
python cli.py gemini chats "Draw a guitar" --response_modalities TEXT,IMAGE
```

### Options

| Option | Description |
|--------|-------------|
| `--stream` | Enable streaming output |
| `--model` | Specify model (optional) |
| `-v, --verbose` | Show detailed metrics (provider, model, elapsed_sec, text_tps, status, etc.) |
| `--response_modalities` | Gemini chats only, comma-separated modalities |
| `--image-output-dir` | Image output directory (Gemini multimodal) |

OpenAI default models:
- `openai chat_completions` -> `gpt-4o-mini`
- `openai responses` -> `gpt-4o-mini`
- `openai embeddings` -> `text-embedding-3-small`

## Shell Completion

Generate completion script:

```bash
python cli.py completion --shell zsh
python cli.py completion --shell bash
python cli.py completion --shell fish
```

Enable (zsh):

```bash
eval "$(python cli.py completion --shell zsh)"
```

## Official SDK Examples

### OpenAI

- Python SDK repository examples: https://github.com/openai/openai-python
- Responses API reference: https://platform.openai.com/docs/api-reference/responses/create
- Chat Completions API reference: https://platform.openai.com/docs/api-reference/chat/create
- Embeddings API reference: https://platform.openai.com/docs/api-reference/embeddings/create

### Anthropic

- Python SDK repository examples: https://github.com/anthropics/anthropic-sdk-python
- Messages API reference: https://docs.anthropic.com/en/api/messages

### Gemini (Google Gen AI SDK)

- Python SDK repository examples: https://github.com/googleapis/python-genai
- Generate content (models) reference: https://ai.google.dev/gemini-api/docs/text-generation
- Chats (multimodal/conversational) reference: https://ai.google.dev/gemini-api/docs/text-generation#multi-turn-conversations

### Complete Example

```bash
# Gemini multimodal streaming
python cli.py gemini chats "Draw a guitar" \
  --response_modalities TEXT,IMAGE \
  --model gemini-3-pro-image-preview \
  --image-output-dir . -v
```
