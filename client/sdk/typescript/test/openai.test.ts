import { beforeEach, describe, expect, it, vi } from "vitest";

const openAIMock = {
  chat: {
    completions: {
      create: vi.fn(),
    },
  },
  responses: {
    create: vi.fn(),
  },
  embeddings: {
    create: vi.fn(),
  },
};

vi.mock("openai", () => ({
  default: vi.fn(() => openAIMock),
}));

describe("openai provider", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  it("extracts chat text", async () => {
    openAIMock.chat.completions.create.mockResolvedValue({
      choices: [{ message: { content: "hello" } }],
    });
    const mod = await import("../src/openai/chat_completions");
    const text = await mod.chat({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "gpt");
    expect(text).toBe("hello");
  });

  it("streams chat deltas only", async () => {
    openAIMock.chat.completions.create.mockResolvedValue(
      (async function* () {
        yield { choices: [{ delta: { content: "he" } }] };
        yield { choices: [{ delta: { content: "" } }] };
        yield { choices: [{ delta: { content: "llo" } }] };
      })(),
    );
    const mod = await import("../src/openai/chat_completions");
    const chunks: string[] = [];
    for await (const chunk of mod.streamChat({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "gpt")) {
      chunks.push(chunk);
    }
    expect(chunks).toEqual(["he", "llo"]);
  });

  it("extracts responses output text fallback", async () => {
    openAIMock.responses.create.mockResolvedValue({
      output: [
        {
          content: [{ text: "a" }, { text: "b" }],
        },
      ],
    });
    const mod = await import("../src/openai/responses");
    await expect(mod.createResponse({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "gpt")).resolves.toBe(
      "ab",
    );
  });

  it("streams response deltas only", async () => {
    openAIMock.responses.create.mockResolvedValue(
      (async function* () {
        yield { type: "ignored", delta: "x" };
        yield { type: "response.output_text.delta", delta: "ok" };
      })(),
    );
    const mod = await import("../src/openai/responses");
    const chunks: string[] = [];
    for await (const chunk of mod.streamResponse({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "gpt")) {
      chunks.push(chunk);
    }
    expect(chunks).toEqual(["ok"]);
  });

  it("maps embedding metadata", async () => {
    openAIMock.embeddings.create.mockResolvedValue({
      data: [{ embedding: [0.1, 0.2, 0.3] }],
      usage: { prompt_tokens: 3, total_tokens: 3 },
    });
    const mod = await import("../src/openai/embeddings");
    await expect(mod.createEmbedding({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "embed")).resolves.toEqual({
      object: "embedding",
      model: "embed",
      dimensions: 3,
      embedding: [0.1, 0.2, 0.3],
      promptTokens: 3,
      totalTokens: 3,
    });
  });
});
