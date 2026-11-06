#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-or-later
// Prints the CHANGELOG.md section of one version, used as the text of a GitHub Release. Fails when the version has
// no section, so a release cannot go out without its changelog entry.
//   node scripts/release-notes.mjs v0.1.0
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const version = (process.argv[2] ?? "").replace(/^v/, "");
if (!/^\d+\.\d+\.\d+$/.test(version)) {
  console.error("usage: release-notes.mjs vMAJOR.MINOR.PATCH");
  process.exit(2);
}
const file = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "CHANGELOG.md");
const lines = readFileSync(file, "utf8").split("\n");
const start = lines.findIndex((l) => l.startsWith(`## [${version}]`));
if (start < 0) {
  console.error(`CHANGELOG.md has no "## [${version}]" section. Add it before releasing.`);
  process.exit(1);
}
let end = lines.findIndex((l, i) => i > start && (l.startsWith("## [") || /^\[[^\]]+\]: /.test(l)));
if (end < 0) end = lines.length;
const body = lines.slice(start + 1, end).join("\n").trim();
if (!body) {
  console.error(`The "## [${version}]" section is empty.`);
  process.exit(1);
}
console.log(body);
