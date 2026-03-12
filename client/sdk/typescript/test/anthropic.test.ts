import { beforeEach, describe, expect, it, vi } from "vitest";

const anthropicMock = {
  messages: {
    create: vi.fn(),
    stream: vi.fn(),
  },
};

vi.mock("@anthropic-ai/sdk", () => ({
  default: vi.fn(() => anthropicMock),
}));

describe("anthropic provider", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  it("returns first text block", async () => {
    anthropicMock.messages.create.mockResolvedValue({
      content: [{ type: "text", text: "hello" }],
    });
    const mod = await import("../src/anthropic/messages");
    await expect(mod.chat({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "claude")).resolves.toBe("hello");
  });

  it("streams non-empty text chunks", async () => {
    anthropicMock.messages.stream.mockReturnValue({
      [Symbol.asyncIterator]: async function* () {
        yield { type: "content_block_delta", delta: { type: "text_delta", text: "a" } };
        yield { type: "content_block_delta", delta: { type: "text_delta", text: "" } };
        yield { type: "message_delta", delta: { type: "other" } };
        yield { type: "content_block_delta", delta: { type: "text_delta", text: "b" } };
      },
    });
    const mod = await import("../src/anthropic/messages");
    const chunks: string[] = [];
    for await (const chunk of mod.streamChat({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "claude")) {
      chunks.push(chunk);
    }
    expect(chunks).toEqual(["a", "b"]);
  });
});
