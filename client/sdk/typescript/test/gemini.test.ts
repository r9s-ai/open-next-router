import { beforeEach, describe, expect, it, vi } from "vitest";

const chatMock = {
  sendMessageStream: vi.fn(),
};

const geminiMock = {
  models: {
    generateContent: vi.fn(),
    generateContentStream: vi.fn(),
  },
  chats: {
    create: vi.fn(() => chatMock),
  },
};

vi.mock("@google/genai", () => ({
  GoogleGenAI: vi.fn(() => geminiMock),
}));

describe("gemini provider", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
  });

  it("returns generateContent text", async () => {
    geminiMock.models.generateContent.mockResolvedValue({ text: "hello" });
    const mod = await import("../src/gemini/models");
    await expect(mod.chat({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "gemini")).resolves.toBe("hello");
  });

  it("streams text only", async () => {
    geminiMock.models.generateContentStream.mockResolvedValue(
      (async function* () {
        yield { text: "a" };
        yield { text: "" };
        yield { text: "b" };
      })(),
    );
    const mod = await import("../src/gemini/models");
    const chunks: string[] = [];
    for await (const chunk of mod.streamChat({ apiKey: "k", baseURL: "http://localhost:3300" }, "hi", "gemini")) {
      chunks.push(chunk);
    }
    expect(chunks).toEqual(["a", "b"]);
  });

  it("streams multimodal events", async () => {
    chatMock.sendMessageStream.mockResolvedValue(
      (async function* () {
        yield {
          candidates: [
            {
              content: {
                parts: [
                  { text: "hello" },
                  { inlineData: { data: Uint8Array.from([1, 2, 3]), mimeType: "image/png" } },
                ],
              },
            },
          ],
        };
      })(),
    );
    const mod = await import("../src/gemini/chats");
    const events = [];
    for await (const event of mod.streamChatMultimodal(
      { apiKey: "k", baseURL: "http://localhost:3300" },
      "hi",
      "gemini",
      ["TEXT", "IMAGE"],
    )) {
      events.push(event);
    }
    expect(events).toEqual([
      { type: "text", text: "hello" },
      { type: "image", imageBytes: Uint8Array.from([1, 2, 3]), imageMimeType: "image/png" },
    ]);
  });
});
