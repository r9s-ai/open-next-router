import type { ClientConfig } from "../config";

import { createClient } from "./shared";

export async function createResponse(
  config: ClientConfig,
  prompt: string,
  model: string,
): Promise<string> {
  const client = createClient(config);
  const response = await client.responses.create({
    model,
    input: prompt,
  });

  if (typeof response.output_text === "string" && response.output_text) {
    return response.output_text;
  }

  const texts: string[] = [];
  for (const item of response.output ?? []) {
    if (!("content" in item) || !Array.isArray(item.content)) {
      continue;
    }
    for (const content of item.content) {
      if ("text" in content && typeof content.text === "string" && content.text) {
        texts.push(content.text);
      }
    }
  }
  return texts.join("");
}

export async function* streamResponse(
  config: ClientConfig,
  prompt: string,
  model: string,
): AsyncIterable<string> {
  const client = createClient(config);
  const stream = await client.responses.create({
    model,
    input: prompt,
    stream: true,
  });

  for await (const event of stream) {
    if (event.type === "response.output_text.delta" && typeof event.delta === "string" && event.delta) {
      yield event.delta;
    }
  }
}
