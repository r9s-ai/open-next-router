import type { ClientConfig } from "../config";

import { createClient } from "./shared";

export type EmbeddingResult = {
  object: string;
  model: string;
  dimensions: number;
  embedding: number[];
  promptTokens: number | null;
  totalTokens: number | null;
};

export async function createEmbedding(
  config: ClientConfig,
  text: string,
  model: string,
): Promise<EmbeddingResult> {
  const client = createClient(config);
  const response = await client.embeddings.create({
    model,
    input: text,
  });
  const embedding = response.data[0]?.embedding ?? [];

  return {
    object: "embedding",
    model,
    dimensions: embedding.length,
    embedding,
    promptTokens: response.usage?.prompt_tokens ?? null,
    totalTokens: response.usage?.total_tokens ?? null,
  };
}
