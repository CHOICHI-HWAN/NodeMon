// SPDX-License-Identifier: GPL-3.0-or-later
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => ({
  plugins: [react()],
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
