# ONR Python SDK CLI

Unified CLI for testing OpenAI, Anthropic, and Gemini providers.

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
python cli.py <provider> "<prompt>" [options]
```

### Quick Examples

```bash
# OpenAI
python cli.py openai "Hello"
python cli.py openai "Hello" --stream -v

# Anthropic
python cli.py anthropic "Hello" --model claude-haiku-4-5
python cli.py anthropic "Hello" --stream -v

# Gemini
python cli.py gemini "Hello" --model gemini-2.5-flash
python cli.py gemini "Hello" --stream -v
```

### Options

| Option | Description |
|--------|-------------|
| `--stream` | Enable streaming output |
| `--model` | Specify model (optional) |
| `-v, --verbose` | Show detailed metrics (provider, model, elapsed_sec, text_tps, status, etc.) |
| `--multimodal` | Gemini multimodal mode (requires `--stream`) |
| `--image-output-dir` | Image output directory (Gemini multimodal) |

### Complete Example

```bash
# Gemini multimodal streaming
python cli.py gemini "Draw a guitar" \
  --stream --multimodal \
  --model gemini-3-pro-image-preview \
  --image-output-dir . -v
```
