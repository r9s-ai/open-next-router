import type { ClientConfig } from "../config";

import { createClient } from "./shared";

export async function chat(config: ClientConfig, prompt: string, model: string): Promise<string> {
  const client = createClient(config);
  const response = await client.models.generateContent({
    model,
    contents: prompt,
  });
  return response.text ?? "";
}

export async function* streamChat(
  config: ClientConfig,
  prompt: string,
  model: string,
): AsyncIterable<string> {
  const client = createClient(config);
  const stream = await client.models.generateContentStream({
    model,
    contents: prompt,
  });

  for await (const chunk of stream) {
    if (chunk.text) {
      yield chunk.text;
    }
  }
}
