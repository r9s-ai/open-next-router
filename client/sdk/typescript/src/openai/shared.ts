import OpenAI from "openai";

import type { ClientConfig } from "../config";

export function createClient(config: ClientConfig): OpenAI {
  return new OpenAI({
    apiKey: config.apiKey,
    baseURL: `${config.baseURL}/v1`,
  });
}
