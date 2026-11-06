#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-or-later
// The list of computer types we build the connector for lives in three places that cannot import each other:
// the shell build script, the Node build script and the web app's "add computer" dialog. If they disagree, the
// dialog can offer a file that was never built. This fails when they do.
//   node scripts/check-targets.mjs
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const read = (f) => readFileSync(path.join(root, f), "utf8");

function fromShell(text) {
  const m = text.match(/for t in ([^;]+); do/);
  if (!m) throw new Error("build-connector.sh: could not find the target loop");
  return m[1].trim().split(/\s+/).map((t) => t.replace("/", "-"));
}

function fromNode(text) {
  const block = text.match(/const TARGETS = \[([\s\S]*?)\];/);
  if (!block) throw new Error("scripts/build-connector.mjs: could not find TARGETS");
  return [...block[1].matchAll(/\["(\w+)",\s*"(\w+)"\]/g)].map((m) => `${m[1]}-${m[2]}`);
}

function fromWeb(text) {
  const block = text.match(/export const ARCHES[\s\S]*?\n};/);
  if (!block) throw new Error("web/src/connectorTargets.ts: could not find ARCHES");
  const out = [];
  let os = null;
  // Walk the block in order: an OS key sets the current OS, every `arch: "x"` after it belongs to that OS.
  for (const m of block[0].matchAll(/\b(linux|darwin|windows): \[|arch: "(\w+)"/g)) {
    if (m[1]) os = m[1];
    else out.push(`${os}-${m[2]}`);
  }
  return out;
}

const sources = {
  "build-connector.sh": fromShell(read("build-connector.sh")),
  "scripts/build-connector.mjs": fromNode(read("scripts/build-connector.mjs")),
  "web/src/connectorTargets.ts": fromWeb(read("web/src/connectorTargets.ts")),
};

const reference = new Set(sources["build-connector.sh"]);
let bad = 0;
for (const [file, list] of Object.entries(sources)) {
  const set = new Set(list);
  const missing = [...reference].filter((t) => !set.has(t));
  const extra = [...set].filter((t) => !reference.has(t));
  if (set.size !== list.length) {
    console.error(`${file}: lists a target twice`);
    bad++;
  }
  if (missing.length || extra.length) {
    console.error(`${file} disagrees with build-connector.sh: missing [${missing}] extra [${extra}]`);
    bad++;
  }
}
if (bad) process.exit(1);
console.log(`build targets agree in all three places (${reference.size}): ${[...reference].join(", ")}`);
