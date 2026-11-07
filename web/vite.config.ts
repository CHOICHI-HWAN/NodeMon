// SPDX-License-Identifier: GPL-3.0-or-later
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

// The dashboard version is the newest released version in CHANGELOG.md, plus the commit it was built from.
function appVersion(): string {
  let v = "0.0.0";
  try {
    v = readFileSync(new URL("../CHANGELOG.md", import.meta.url), "utf8").match(/^## \[(\d+\.\d+\.\d+)\]/m)?.[1] ?? v;
  } catch {
    /* no changelog next to the build: keep the placeholder */
  }
  let commit = "";
  try {
    commit = execFileSync("git", ["rev-parse", "--short", "HEAD"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
  } catch {
    /* not a git checkout */
  }
  return commit ? `${v} (${commit})` : v;
}

export default defineConfig(({ mode }) => ({
  plugins: [react()],
  define: { __APP_VERSION__: JSON.stringify(appVersion()) },
  build:
    mode === "demo"
      ? {
          // A single self-contained script so it can be inlined into one HTML page.
          outDir: "dist-demo",
          modulePreload: false,
          cssCodeSplit: false,
          rollupOptions: { output: { format: "iife", inlineDynamicImports: true } },
        }
      : mode === "firebase"
        ? { outDir: "dist-firebase" }
        : undefined,
}));
