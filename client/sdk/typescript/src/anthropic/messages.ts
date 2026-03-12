import type { ClientConfig } from "../config";

import { createClient, DEFAULT_MAX_TOKENS } from "./shared";

export async function chat(
  config: ClientConfig,
  prompt: string,
  model: string,
  maxTokens = DEFAULT_MAX_TOKENS,
): Promise<string> {
  const client = createClient(config);
  const response = await client.messages.create({
    model,
    max_tokens: maxTokens,
    messages: [{ role: "user", content: prompt }],
  });

  for (const block of response.content) {
    if (block.type === "text" && block.text) {
      return block.text;
    }
  }
  return "";
}

export async function* streamChat(
  config: ClientConfig,
  prompt: string,
  model: string,
  maxTokens = DEFAULT_MAX_TOKENS,
): AsyncIterable<string> {
  const client = createClient(config);
  const stream = client.messages.stream({
    model,
    max_tokens: maxTokens,
    messages: [{ role: "user", content: prompt }],
  });

  for await (const event of stream) {
    if (
      event.type === "content_block_delta" &&
      event.delta.type === "text_delta" &&
      event.delta.text
    ) {
      yield event.delta.text;
    }
  }
}
