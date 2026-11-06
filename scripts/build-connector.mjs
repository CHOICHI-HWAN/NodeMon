#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-or-later
// Cross-compiles the connector for every supported computer type and writes a SHA256SUMS file next to the
// binaries. Works on Windows, macOS and Linux (it only needs Go).
//   node scripts/build-connector.mjs [--out dist]
// Keep TARGETS in sync with build-connector.sh and with web/src/connectorTargets.ts.
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const TARGETS = [
  ["linux", "amd64"],
  ["linux", "arm64"],
  ["linux", "arm"],
  ["windows", "amd64"],
  ["darwin", "arm64"],
  ["darwin", "amd64"],
];

const args = process.argv.slice(2);
const out = path.resolve(root, args.includes("--out") ? args[args.indexOf("--out") + 1] : "dist");

// go and git are real executables, so no shell is needed. (On Windows a shell would also join the arguments without
// quoting, which breaks the -ldflags value because it contains spaces.)
const probe = spawnSync("go", ["version"], { encoding: "utf8" });
if (probe.status !== 0) {
  console.error("Go was not found, so the connector cannot be built here. Install Go 1.24+ from https://go.dev/dl/");
  process.exit(2);
}

// The version shown by `nodemon-connector --version`: the git tag or commit, or "dev" outside a checkout.
const described = spawnSync("git", ["describe", "--tags", "--always", "--dirty"], { cwd: root, encoding: "utf8" });
const version = described.status === 0 ? described.stdout.trim() : "dev";

mkdirSync(out, { recursive: true });
const sums = [];
for (const [os, arch] of TARGETS) {
  const name = `nodemon-connector-${os}-${arch}${os === "windows" ? ".exe" : ""}`;
  const file = path.join(out, name);
  // -s -w drops debug tables, about a third smaller; -trimpath keeps build-machine paths out of the binary.
  const r = spawnSync("go", ["build", "-trimpath", "-ldflags", `-s -w -X main.version=${version}`, "-o", file, "."], {
    cwd: path.join(root, "connector"),
    stdio: ["ignore", "inherit", "inherit"],
    env: { ...process.env, GOOS: os, GOARCH: arch, GOARM: "7", CGO_ENABLED: "0" },
  });
  if (r.status !== 0) {
    console.error(`build failed for ${os}/${arch}`);
    process.exit(1);
  }
  const hash = createHash("sha256").update(readFileSync(file)).digest("hex");
  sums.push(`${hash}  ${name}`);
  console.log(`built ${path.relative(root, file)}  (${(statSync(file).size / 1048576).toFixed(1)} MB)`);
}
writeFileSync(path.join(out, "SHA256SUMS"), sums.join("\n") + "\n");
console.log(`wrote ${path.relative(root, path.join(out, "SHA256SUMS"))}`);
