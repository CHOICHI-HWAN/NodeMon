#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-or-later
// Builds nodemon-tray, the small icon program for the clock area (Linux) and the menu bar (macOS).
//   node scripts/build-tray.mjs [--out dist] [--only linux|darwin]
// The Linux builds need no C compiler, so any computer can cross-compile them. The macOS builds use Apple's
// frameworks through cgo, so they can only be made on a Mac (the release workflow does that on a macOS runner).
import { spawnSync } from "node:child_process";
import { mkdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const TARGETS = [
  ["linux", "amd64"],
  ["linux", "arm64"],
  ["linux", "arm"],
  ["darwin", "arm64"],
  ["darwin", "amd64"],
];

const args = process.argv.slice(2);
const opt = (name, fallback) => (args.includes(name) ? args[args.indexOf(name) + 1] : fallback);
const out = path.resolve(root, opt("--out", "dist"));
const only = opt("--only", null);

const probe = spawnSync("go", ["version"], { encoding: "utf8" });
if (probe.status !== 0) {
  console.error("Go was not found, so the tray icon cannot be built here. Install Go 1.24+ from https://go.dev/dl/");
  process.exit(2);
}
const described = spawnSync("git", ["describe", "--tags", "--always", "--dirty"], { cwd: root, encoding: "utf8" });
const version = described.status === 0 ? described.stdout.trim() : "dev";

mkdirSync(out, { recursive: true });
for (const [os, arch] of TARGETS) {
  if (only && os !== only) continue;
  if (os === "darwin" && process.platform !== "darwin") {
    console.log(`skipped nodemon-tray-${os}-${arch}: macOS builds need a Mac`);
    continue;
  }
  const file = path.join(out, `nodemon-tray-${os}-${arch}`);
  const r = spawnSync("go", ["build", "-trimpath", "-ldflags", `-s -w -X main.version=${version}`, "-o", file, "."], {
    cwd: path.join(root, "tray"),
    stdio: ["ignore", "inherit", "inherit"],
    env: { ...process.env, GOOS: os, GOARCH: arch, GOARM: "7", CGO_ENABLED: os === "darwin" ? "1" : "0" },
  });
  if (r.status !== 0) {
    console.error(`build failed for ${os}/${arch}`);
    process.exit(1);
  }
  console.log(`built ${path.relative(root, file)}  (${(statSync(file).size / 1048576).toFixed(1)} MB)`);
}
