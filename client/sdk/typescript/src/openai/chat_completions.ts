import type { ClientConfig } from "../config";

import { createClient } from "./shared";

export async function chat(config: ClientConfig, prompt: string, model: string): Promise<string> {
  const client = createClient(config);
  const response = await client.chat.completions.create({
    model,
    messages: [{ role: "user", content: prompt }],
  });
  return response.choices[0]?.message?.content ?? "";
}

export async function* streamChat(
  config: ClientConfig,
  prompt: string,
  model: string,
): AsyncIterable<string> {
  const client = createClient(config);
  const stream = await client.chat.completions.create({
    model,
    messages: [{ role: "user", content: prompt }],
    stream: true,
  });

  for await (const chunk of stream) {
    const delta = chunk.choices[0]?.delta?.content;
    if (delta) {
      yield delta;
    }
  }
}
