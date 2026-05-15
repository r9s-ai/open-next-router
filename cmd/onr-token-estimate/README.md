# onr-token-estimate

`onr-token-estimate` reads http-relay simple dump logs, runs ONR's `usageestimate` logic for each complete record, and compares estimated tokens with upstream official usage.

## Run

From `relay/open-next-router`:

```bash
go run ./cmd/onr-token-estimate \
  --file /path/to/codex.log \
  --api responses \
  --model gpt-5.5
```

From this directory:

```bash
go run . -f /path/to/codex.log --api responses -m gpt-5.5
```

## Flags

```text
--file, -f          dump file path
--api              usageestimate API name
--route            route alias for API name
--model, -m        model name
--allow-truncated  allow truncated dump bodies
--debug-id         write extracted request/response text for one dump id without estimating
--debug-dir        directory for --debug-id files, default dump file directory
```

Use either `--api` or `--route`.

Supported API names:

```text
chat.completions
responses
claude.messages
embeddings
gemini.generateContent
gemini.streamGenerateContent
```

Supported route aliases:

```text
openai-chat
openai-chat-completions
openai-responses
anthropic-messages
claude-messages
gemini-generate-content
gemini-stream-generate-content
embeddings
```

## Input Format

The command expects records in http-relay `--simple-dump` shape:

```json
{
  "id": 1,
  "request": {
    "body": {
      "format": "json",
      "size": 19,
      "truncated": false,
      "content": {
        "ping": "pong"
      }
    }
  },
  "response": {
    "body": {
      "format": "json",
      "size": 11,
      "truncated": false,
      "content": {
        "ok": true,
        "usage": {
          "input_tokens": 10,
          "output_tokens": 2,
          "total_tokens": 12
        }
      }
    }
  }
}
```

Response SSE body is also supported:

```json
{
  "id": 2,
  "request": {
    "body": {
      "format": "empty",
      "size": 0,
      "truncated": false
    }
  },
  "response": {
    "body": {
      "format": "sse",
      "size": 71,
      "truncated": false,
      "events": [
        {
          "event": "response.output_text.delta",
          "data": {
            "type": "response.output_text.delta",
            "delta": "hello"
          }
        },
        {
          "event": "response.completed",
          "data": {
            "type": "response.completed",
            "response": {
              "usage": {
                "input_tokens": 10,
                "output_tokens": 2,
                "total_tokens": 12
              }
            }
          }
        }
      ]
    }
  }
}
```

Supported file containers:

Single JSON object:

```json
{ "id": 1, "request": { "body": { "format": "empty" } }, "response": { "body": { "format": "empty" } } }
```

JSON array:

```json
[
  { "id": 1, "request": { "body": { "format": "empty" } }, "response": { "body": { "format": "empty" } } },
  { "id": 2, "request": { "body": { "format": "empty" } }, "response": { "body": { "format": "empty" } } }
]
```

JSON stream, as produced by logs that append one object after another:

```json
{ "id": 1, "request": { "body": { "format": "empty" } }, "response": { "body": { "format": "empty" } } }
{ "id": 2, "request": { "body": { "format": "empty" } }, "response": { "body": { "format": "empty" } } }
```

Loose arrays with a leading `[` and missing final `]` are tolerated when entries can still be decoded.

## Output

The command prints one aligned table.

```text
status     id  stage          in.actual  in.est  in.delta  out.actual  out.est  out.delta  reason
---------  --  -------------  ---------  ------  --------  ----------  -------  ---------  -------------------------
estimated   4  estimate_both       9594    9485    -1.14%          60      125   +108.33%
skipped     8                                                                             token usage not detected
summary entries=12 estimated=5 skipped=7
```

Columns:

- `in.actual` / `out.actual`: official upstream token usage extracted from the response.
- `in.est` / `out.est`: local estimate from `usageestimate.Estimate`.
- `in.delta` / `out.delta`: `(estimated - actual) / actual * 100`.
- `skipped`: record is not estimated. Most commonly the response does not expose complete token usage for comparison.

## Debug

Use `--debug-id` when estimation input or output extraction looks suspicious. It writes the extracted request and response text from the matching dump record to files, then exits without estimating any records or printing the summary table.

```bash
go run . \
  -f /path/to/codex.log \
  --api responses \
  -m gpt-5.5 \
  --debug-id 29 \
  --debug-dir /tmp/onr-token-estimate-debug
```

The command prints the generated file paths and extracted character counts:

```text
debug dump id=29
request_file=/tmp/onr-token-estimate-debug/onr-token-estimate-29-request.txt request_chars=1234
response_file=/tmp/onr-token-estimate-debug/onr-token-estimate-29-response.txt response_chars=5678
```

Without `--debug-dir`, files are written next to the dump file:

```text
onr-token-estimate-29-request.txt
onr-token-estimate-29-response.txt
```

The request file contains the same request text used by `usageestimate` input token estimation. The response file contains the same non-stream response text used by `usageestimate`; for SSE responses it contains text extracted from stream events. This is useful for checking whether request JSON is parsed as expected, or whether SSE events are missing from `streamtext.ExtractDeltaText`.
