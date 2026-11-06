#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-or-later
// Packs the binaries that build-connector.mjs wrote into one ZIP per computer type (plus SHA256SUMS of the ZIPs),
// ready to attach to a GitHub Release. Each ZIP holds the program, the license and the third-party notices.
//   node scripts/build-connector.mjs --out dist && node scripts/package-release.mjs [--in dist] [--out release]
// Needs the `zip` command (present on GitHub's Linux runners); it keeps the executable bit of Linux/macOS files.
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
const opt = (name, fallback) => path.resolve(root, args.includes(name) ? args[args.indexOf(name) + 1] : fallback);
const input = opt("--in", "dist");
const out = opt("--out", "release");

const binaries = readdirSync(input).filter((n) => n.startsWith("nodemon-connector-"));
if (binaries.length === 0) {
  console.error(`No nodemon-connector-* files in ${input}. Run scripts/build-connector.mjs first.`);
  process.exit(2);
}
mkdirSync(out, { recursive: true });
const sums = [];
for (const bin of binaries.sort()) {
  const zipName = bin.replace(/\.exe$/, "") + ".zip";
  const stage = mkdtempSync(path.join(tmpdir(), "nodemon-pack-"));
  try {
    // The program keeps its full name (nodemon-connector-linux-amd64) so the commands in the dashboard match it.
    for (const f of [bin]) copyFileSync(path.join(input, f), path.join(stage, f));
    for (const f of ["LICENSE", "THIRD_PARTY_LICENSES.md"]) copyFileSync(path.join(root, f), path.join(stage, f));
    const zipPath = path.join(out, zipName);
    rmSync(zipPath, { force: true });
    const r = spawnSync("zip", ["-q", "-X", zipPath, ...readdirSync(stage).sort()], { cwd: stage, stdio: ["ignore", "inherit", "inherit"] });
    if (r.status !== 0) {
      console.error(`zip failed for ${bin} (is the zip command installed?)`);
      process.exit(1);
    }
    sums.push(`${createHash("sha256").update(readFileSync(zipPath)).digest("hex")}  ${zipName}`);
    console.log(`packed ${path.relative(root, zipPath)}`);
  } finally {
    rmSync(stage, { recursive: true, force: true });
  }
}
writeFileSync(path.join(out, "SHA256SUMS"), sums.join("\n") + "\n");
console.log(`wrote ${path.relative(root, path.join(out, "SHA256SUMS"))}`);
