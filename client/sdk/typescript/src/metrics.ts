export type RequestMetrics = {
  provider: string;
  model: string;
  baseURL: string;
  stream: boolean;
  elapsedSec: number;
  textChars: number;
  imageCount?: number;
  status: string;
  exceptionMessage?: string;
};

export function formatRequestMetrics(metrics: RequestMetrics): string {
  const elapsed = metrics.elapsedSec > 0 ? metrics.elapsedSec : 1e-9;
  const tps = metrics.textChars / elapsed;
  const lines = [
    "",
    "=== Request Metrics ===",
    `provider: ${metrics.provider}`,
    `model: ${metrics.model}`,
    `base_url: ${metrics.baseURL}`,
    `stream: ${String(metrics.stream)}`,
    `elapsed_sec: ${metrics.elapsedSec.toFixed(3)}`,
    `text_chars: ${metrics.textChars}`,
    `text_tps: ${tps.toFixed(2)} chars/sec`,
    `status: ${metrics.status}`,
  ];

  if ((metrics.imageCount ?? 0) > 0) {
    lines.push(`images: ${metrics.imageCount}`);
  }
  if (metrics.exceptionMessage) {
    lines.push(`exception: ${metrics.exceptionMessage}`);
  }
  return `${lines.join("\n")}\n`;
}
