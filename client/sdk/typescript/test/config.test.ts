import { describe, expect, it } from "vitest";

import { createConfigFromEnv, DEFAULT_BASE_URL } from "../src/config";

describe("createConfigFromEnv", () => {
  it("requires ONR_API_KEY", () => {
    expect(() => createConfigFromEnv({ ONR_API_KEY: "", ONR_BASE_URL: "" })).toThrow(
      "ONR_API_KEY environment variable is not set",
    );
  });

  it("uses default base url", () => {
    expect(createConfigFromEnv({ ONR_API_KEY: "test-key", ONR_BASE_URL: "" })).toEqual({
      apiKey: "test-key",
      baseURL: DEFAULT_BASE_URL,
    });
  });

  it("uses custom base url", () => {
    expect(
      createConfigFromEnv({ ONR_API_KEY: "test-key", ONR_BASE_URL: "https://api.example.com" }),
    ).toEqual({
      apiKey: "test-key",
      baseURL: "https://api.example.com",
    });
  });
});
