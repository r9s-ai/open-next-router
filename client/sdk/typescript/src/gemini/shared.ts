import { GoogleGenAI } from "@google/genai";

import type { ClientConfig } from "../config";

function trimBaseURL(baseURL: string): string {
  return baseURL.replace(/\/+$/, "");
}

export function createClient(config: ClientConfig): GoogleGenAI {
  return new GoogleGenAI({
    apiKey: config.apiKey,
    httpOptions: {
      baseUrl: trimBaseURL(config.baseURL),
    },
  });
}
