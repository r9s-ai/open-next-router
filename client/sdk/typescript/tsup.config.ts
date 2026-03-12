import { defineConfig } from "tsup";

export default defineConfig([
  {
    entry: {
      index: "src/index.ts",
    },
    format: ["esm", "cjs"],
    dts: true,
    sourcemap: true,
    clean: true,
    splitting: false,
    target: "node20",
    outDir: "dist",
  },
  {
    entry: {
      cli: "src/cli.ts",
    },
    format: ["esm", "cjs"],
    dts: true,
    sourcemap: true,
    clean: false,
    splitting: false,
    target: "node20",
    outDir: "dist",
    banner: {
      js: "#!/usr/bin/env node",
    },
  },
]);
