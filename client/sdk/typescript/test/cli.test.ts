import { mkdtemp, readFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const stdoutWrite = vi.spyOn(process.stdout, "write").mockImplementation(() => true);
const stderrWrite = vi.spyOn(process.stderr, "write").mockImplementation(() => true);

vi.mock("../src/config", () => ({
  createConfigFromEnv: vi.fn(() => ({ apiKey: "key", baseURL: "http://localhost:3300" })),
}));

vi.mock("../src/openai/chat_completions", () => ({
  chat: vi.fn(async () => "hello"),
  streamChat: vi.fn(async function* () {
    yield "he";
    yield "llo";
  }),
}));

vi.mock("../src/openai/responses", () => ({
  createResponse: vi.fn(async () => "response"),
  streamResponse: vi.fn(async function* () {
    yield "res";
  }),
}));

vi.mock("../src/openai/embeddings", () => ({
  createEmbedding: vi.fn(async () => ({
    object: "embedding",
    model: "text-embedding-3-small",
    dimensions: 3,
    embedding: [1, 2, 3],
    promptTokens: 3,
    totalTokens: 3,
  })),
}));

vi.mock("../src/anthropic/messages", () => ({
  chat: vi.fn(async () => "claude"),
  streamChat: vi.fn(async function* () {
    yield "cl";
  }),
}));

vi.mock("../src/gemini/models", () => ({
  chat: vi.fn(async () => "gemini"),
  streamChat: vi.fn(async function* () {
    yield "ge";
  }),
}));

vi.mock("../src/gemini/chats", () => ({
  streamChatMultimodal: vi.fn(async function* () {
    yield { type: "text", text: "hi" };
    yield { type: "image", imageBytes: Uint8Array.from([1, 2, 3]), imageMimeType: "image/png" };
  }),
}));

describe("cli", () => {
  beforeEach(() => {
    stdoutWrite.mockClear();
    stderrWrite.mockClear();
  });

  afterEach(() => {
    vi.resetModules();
  });

  it("prints completion scripts", async () => {
    const { createCLI } = await import("../src/cli");
    await createCLI().parseAsync(["completion", "--shell", "bash"], { from: "user" });
    expect(stdoutWrite).toHaveBeenCalled();
    expect(String(stdoutWrite.mock.calls[0]?.[0])).toContain("complete -F");
  });

  it("uses default model for openai chat", async () => {
    const { createCLI } = await import("../src/cli");
    await createCLI().parseAsync(["openai", "chat_completions", "hello"], { from: "user" });
    const openai = await import("../src/openai/chat_completions");
    expect(openai.chat).toHaveBeenCalledWith(
      { apiKey: "key", baseURL: "http://localhost:3300" },
      "hello",
      "gpt-4o-mini",
    );
  });

  it("writes stream output and metrics", async () => {
    const { createCLI } = await import("../src/cli");
    await createCLI().parseAsync(
      ["openai", "responses", "hello", "--stream", "--verbose"],
      { from: "user" },
    );
    const output = stdoutWrite.mock.calls.map((call) => String(call[0])).join("");
    expect(output).toContain("res");
    expect(output).toContain("=== Request Metrics ===");
  });

  it("writes gemini chat images to disk", async () => {
    const dir = await mkdtemp(path.join(os.tmpdir(), "onr-sdk-ts-"));
    const { createCLI } = await import("../src/cli");
    await createCLI().parseAsync(
      [
        "gemini",
        "chats",
        "draw a guitar",
        "--image-output-dir",
        dir,
      ],
      { from: "user" },
    );
    const image = await readFile(path.join(dir, "gemini_1.png"));
    expect(Array.from(image)).toEqual([1, 2, 3]);
  });

  it("prints sanitized error on run", async () => {
    const openai = await import("../src/openai/chat_completions");
    vi.mocked(openai.chat).mockRejectedValueOnce(new Error("boom"));
    const { run } = await import("../src/cli");
    await run(["node", "onr-sdk-ts", "openai", "chat_completions", "hello"]);
    expect(stderrWrite).toHaveBeenCalledWith("request failed: boom\n");
  });
});
