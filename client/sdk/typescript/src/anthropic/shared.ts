import Anthropic from "@anthropic-ai/sdk";

import type { ClientConfig } from "../config";

export const DEFAULT_MAX_TOKENS = 1024;

export function createClient(config: ClientConfig): Anthropic {
  return new Anthropic({
    apiKey: config.apiKey,
    baseURL: config.baseURL,
  });
}
