export const DEFAULT_BASE_URL = "http://localhost:3300";

export type ClientConfig = {
  apiKey: string;
  baseURL: string;
};

function trimEnvValue(value: string | undefined): string {
  return (value ?? "").trim();
}

export function createConfigFromEnv(env: NodeJS.ProcessEnv = process.env): ClientConfig {
  const apiKey = trimEnvValue(env.ONR_API_KEY);
  const baseURL = trimEnvValue(env.ONR_BASE_URL) || DEFAULT_BASE_URL;

  if (!apiKey) {
    throw new Error("ONR_API_KEY environment variable is not set");
  }

  if (!baseURL) {
    throw new Error("base URL is empty");
  }

  return { apiKey, baseURL };
}
