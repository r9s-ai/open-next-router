import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { Command } from "commander";

import { chat as anthropicChat, streamChat as anthropicStreamChat } from "./anthropic/messages";
import { renderCompletion } from "./completion";
import { createConfigFromEnv } from "./config";
import { streamChatMultimodal } from "./gemini/chats";
import { chat as geminiChat, streamChat as geminiStreamChat } from "./gemini/models";
import { formatRequestMetrics } from "./metrics";
import { createEmbedding } from "./openai/embeddings";
import { chat as openAIChat, streamChat as openAIStreamChat } from "./openai/chat_completions";
import { createResponse, streamResponse } from "./openai/responses";

const DEFAULT_OPENAI_MODEL = "gpt-4o-mini";
const DEFAULT_OPENAI_EMBEDDING_MODEL = "text-embedding-3-small";
const DEFAULT_ANTHROPIC_MODEL = "claude-haiku-4-5";
const DEFAULT_GEMINI_MODEL = "gemini-2.5-flash";
const DEFAULT_GEMINI_CHATS_MODEL = "gemini-3-pro-image-preview";
const DEFAULT_ANTHROPIC_MAX_TOKENS = 1024;

function sanitizeError(error: unknown): Error {
  if (error instanceof Error) {
    return new Error(`request failed: ${error.message}`);
  }
  return new Error(`request failed: ${String(error)}`);
}

function userError(error: unknown): Error {
  if (error instanceof Error && error.name === "AbortError") {
    return error;
  }
  if (error instanceof Error) {
    return new Error(`error: ${error.message}`);
  }
  return new Error(`error: ${String(error)}`);
}

function splitModalities(raw: string): string[] {
  return raw
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function imageExt(mime: string): string {
  switch (mime) {
    case "image/jpeg":
      return ".jpg";
    case "image/webp":
      return ".webp";
    default:
      return ".png";
  }
}

async function writeStream(iterable: AsyncIterable<string>): Promise<number> {
  let textChars = 0;
  for await (const text of iterable) {
    process.stdout.write(text);
    textChars += text.length;
  }
  process.stdout.write("\n");
  return textChars;
}

export function createCLI(): Command {
  const program = new Command();
  program
    .name("onr-sdk-ts")
    .description("ONR SDK CLI - Unified interface for OpenAI, Anthropic, Gemini")
    .showHelpAfterError();

  program
    .command("completion")
    .requiredOption("--shell <shell>", "Shell type")
    .action((options: { shell: string }) => {
      process.stdout.write(renderCompletion(options.shell.toLowerCase(), program));
    });

  const openAI = program.command("openai").description("OpenAI API");

  openAI
    .command("chat_completions")
    .argument("<prompt>")
    .option("-m, --model <model>", "Model name", DEFAULT_OPENAI_MODEL)
    .option("--stream", "Enable streaming", false)
    .option("-v, --verbose", "Print request metrics", false)
    .action(async (prompt: string, options: { model: string; stream: boolean; verbose: boolean }) => {
      const cfg = createConfigFromEnv();
      const start = Date.now();
      let textChars = 0;
      let status = "ok";
      let exceptionMessage = "";
      try {
        if (options.stream) {
          textChars = await writeStream(openAIStreamChat(cfg, prompt, options.model));
          return;
        }
        const text = await openAIChat(cfg, prompt, options.model);
        process.stdout.write(`${text}\n`);
        textChars = text.length;
      } catch (error) {
        status = "error";
        exceptionMessage = sanitizeError(error).message;
        throw sanitizeError(error);
      } finally {
        if (options.verbose) {
          process.stdout.write(
            formatRequestMetrics({
              provider: "openai/chat_completions",
              model: options.model,
              baseURL: cfg.baseURL,
              stream: options.stream,
              elapsedSec: (Date.now() - start) / 1000,
              textChars,
              status,
              exceptionMessage,
            }),
          );
        }
      }
    });

  openAI
    .command("responses")
    .argument("<prompt>")
    .option("-m, --model <model>", "Model name", DEFAULT_OPENAI_MODEL)
    .option("--stream", "Enable streaming", false)
    .option("-v, --verbose", "Print request metrics", false)
    .action(async (prompt: string, options: { model: string; stream: boolean; verbose: boolean }) => {
      const cfg = createConfigFromEnv();
      const start = Date.now();
      let textChars = 0;
      let status = "ok";
      let exceptionMessage = "";
      try {
        if (options.stream) {
          textChars = await writeStream(streamResponse(cfg, prompt, options.model));
          return;
        }
        const text = await createResponse(cfg, prompt, options.model);
        process.stdout.write(`${text}\n`);
        textChars = text.length;
      } catch (error) {
        status = "error";
        exceptionMessage = sanitizeError(error).message;
        throw sanitizeError(error);
      } finally {
        if (options.verbose) {
          process.stdout.write(
            formatRequestMetrics({
              provider: "openai/responses",
              model: options.model,
              baseURL: cfg.baseURL,
              stream: options.stream,
              elapsedSec: (Date.now() - start) / 1000,
              textChars,
              status,
              exceptionMessage,
            }),
          );
        }
      }
    });

  openAI
    .command("embeddings")
    .argument("<text>")
    .option("-m, --model <model>", "Model name", DEFAULT_OPENAI_EMBEDDING_MODEL)
    .option("-v, --verbose", "Print request metrics", false)
    .action(async (text: string, options: { model: string; verbose: boolean }) => {
      const cfg = createConfigFromEnv();
      const start = Date.now();
      let textChars = 0;
      let status = "ok";
      let exceptionMessage = "";
      try {
        const result = await createEmbedding(cfg, text, options.model);
        const preview = result.embedding.slice(0, 8);
        process.stdout.write(`object: ${result.object}\n`);
        process.stdout.write(`model: ${result.model}\n`);
        process.stdout.write(`dimensions: ${result.dimensions}\n`);
        process.stdout.write(`prompt_tokens: ${result.promptTokens}\n`);
        process.stdout.write(`total_tokens: ${result.totalTokens}\n`);
        process.stdout.write(`embedding_preview: ${JSON.stringify(preview)}\n`);
        textChars = JSON.stringify(preview).length;
      } catch (error) {
        status = "error";
        exceptionMessage = sanitizeError(error).message;
        throw sanitizeError(error);
      } finally {
        if (options.verbose) {
          process.stdout.write(
            formatRequestMetrics({
              provider: "openai/embeddings",
              model: options.model,
              baseURL: cfg.baseURL,
              stream: false,
              elapsedSec: (Date.now() - start) / 1000,
              textChars,
              status,
              exceptionMessage,
            }),
          );
        }
      }
    });

  const anthropic = program.command("anthropic").description("Anthropic API");
  anthropic
    .command("messages")
    .argument("<prompt>")
    .option("-m, --model <model>", "Model name", DEFAULT_ANTHROPIC_MODEL)
    .option("--stream", "Enable streaming", false)
    .option("-v, --verbose", "Print request metrics", false)
    .action(async (prompt: string, options: { model: string; stream: boolean; verbose: boolean }) => {
      const cfg = createConfigFromEnv();
      const start = Date.now();
      let textChars = 0;
      let status = "ok";
      let exceptionMessage = "";
      try {
        if (options.stream) {
          textChars = await writeStream(anthropicStreamChat(cfg, prompt, options.model, DEFAULT_ANTHROPIC_MAX_TOKENS));
          return;
        }
        const text = await anthropicChat(cfg, prompt, options.model, DEFAULT_ANTHROPIC_MAX_TOKENS);
        process.stdout.write(`${text}\n`);
        textChars = text.length;
      } catch (error) {
        status = "error";
        exceptionMessage = sanitizeError(error).message;
        throw sanitizeError(error);
      } finally {
        if (options.verbose) {
          process.stdout.write(
            formatRequestMetrics({
              provider: "anthropic/messages",
              model: options.model,
              baseURL: cfg.baseURL,
              stream: options.stream,
              elapsedSec: (Date.now() - start) / 1000,
              textChars,
              status,
              exceptionMessage,
            }),
          );
        }
      }
    });

  const gemini = program.command("gemini").description("Google Gemini API");
  gemini
    .command("models")
    .argument("<prompt>")
    .option("-m, --model <model>", "Model name", DEFAULT_GEMINI_MODEL)
    .option("--stream", "Enable streaming", false)
    .option("-v, --verbose", "Print request metrics", false)
    .action(async (prompt: string, options: { model: string; stream: boolean; verbose: boolean }) => {
      const cfg = createConfigFromEnv();
      const start = Date.now();
      let textChars = 0;
      let status = "ok";
      let exceptionMessage = "";
      try {
        if (options.stream) {
          textChars = await writeStream(geminiStreamChat(cfg, prompt, options.model));
          return;
        }
        const text = await geminiChat(cfg, prompt, options.model);
        process.stdout.write(`${text}\n`);
        textChars = text.length;
      } catch (error) {
        status = "error";
        exceptionMessage = sanitizeError(error).message;
        throw sanitizeError(error);
      } finally {
        if (options.verbose) {
          process.stdout.write(
            formatRequestMetrics({
              provider: "gemini/models",
              model: options.model,
              baseURL: cfg.baseURL,
              stream: options.stream,
              elapsedSec: (Date.now() - start) / 1000,
              textChars,
              status,
              exceptionMessage,
            }),
          );
        }
      }
    });

  gemini
    .command("chats")
    .argument("<prompt>")
    .option("-m, --model <model>", "Model name", DEFAULT_GEMINI_CHATS_MODEL)
    .option("--response_modalities <modalities>", "Comma-separated response modalities", "TEXT,IMAGE")
    .option("--image-output-dir <dir>", "Directory to save generated images", ".")
    .option("-v, --verbose", "Print request metrics", false)
    .action(
      async (
        prompt: string,
        options: { model: string; response_modalities: string; imageOutputDir: string; verbose: boolean },
      ) => {
        const cfg = createConfigFromEnv();
        const start = Date.now();
        let textChars = 0;
        let imageCount = 0;
        let status = "ok";
        let exceptionMessage = "";
        try {
          await mkdir(options.imageOutputDir, { recursive: true });
          for await (const event of streamChatMultimodal(
            cfg,
            prompt,
            options.model,
            splitModalities(options.response_modalities),
          )) {
            if (event.type === "text") {
              process.stdout.write(event.text);
              textChars += event.text.length;
              continue;
            }
            imageCount += 1;
            const imagePath = path.join(
              options.imageOutputDir,
              `gemini_${imageCount}${imageExt(event.imageMimeType)}`,
            );
            await writeFile(imagePath, event.imageBytes);
            process.stdout.write(`\n[image saved] ${imagePath}\n`);
          }
          process.stdout.write("\n");
        } catch (error) {
          status = "error";
          exceptionMessage = sanitizeError(error).message;
          throw sanitizeError(error);
        } finally {
          if (options.verbose) {
            process.stdout.write(
              formatRequestMetrics({
                provider: "gemini/chats",
                model: options.model,
                baseURL: cfg.baseURL,
                stream: true,
                elapsedSec: (Date.now() - start) / 1000,
                textChars,
                imageCount,
                status,
                exceptionMessage,
              }),
            );
          }
        }
      },
    );

  return program;
}

export async function run(argv = process.argv): Promise<void> {
  const program = createCLI();
  try {
    await program.parseAsync(argv);
  } catch (error) {
    const normalized = error instanceof Error && error.message.startsWith("request failed:")
      ? error
      : userError(error);
    process.stderr.write(`${normalized.message}\n`);
    process.exitCode = 1;
  }
}

if (typeof module !== "undefined" && require.main === module) {
  void run();
}
