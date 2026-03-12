import type { ClientConfig } from "../config";

import { createClient } from "./shared";

export type GeminiTextEvent = {
  type: "text";
  text: string;
};

export type GeminiImageEvent = {
  type: "image";
  imageBytes: Uint8Array;
  imageMimeType: string;
};

export type StreamEvent = GeminiTextEvent | GeminiImageEvent;

function toUint8Array(data: unknown): Uint8Array {
  if (data instanceof Uint8Array) {
    return data;
  }
  if (Array.isArray(data)) {
    return Uint8Array.from(data);
  }
  if (data instanceof ArrayBuffer) {
    return new Uint8Array(data);
  }
  return new Uint8Array();
}

export async function* streamChatMultimodal(
  config: ClientConfig,
  prompt: string,
  model: string,
  modalities: string[] = ["TEXT", "IMAGE"],
): AsyncIterable<StreamEvent> {
  const client = createClient(config);
  const chat = client.chats.create({
    model,
    config: {
      responseModalities: modalities,
    },
  });
  const stream = await chat.sendMessageStream({ message: prompt });

  for await (const chunk of stream) {
    for (const candidate of chunk.candidates ?? []) {
      for (const part of candidate.content?.parts ?? []) {
        if (typeof part.text === "string" && part.text) {
          yield { type: "text", text: part.text };
        }
        if (part.inlineData?.data) {
          yield {
            type: "image",
            imageBytes: toUint8Array(part.inlineData.data),
            imageMimeType: part.inlineData.mimeType ?? "image/png",
          };
        }
      }
    }
  }
}
