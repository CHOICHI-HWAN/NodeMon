#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-or-later
// Regenerates THIRD_PARTY_LICENSES.md (and the copy served by the web app) from the dependencies that
// actually ship: the web app's production npm packages, the Go modules linked into the server, and the
// Go standard library. It fails if a license is not on the GPL-3.0-compatible allow-list.
//   node scripts/third-party-notices.mjs
import { execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sh = (cmd, args, cwd) => execFileSync(cmd, args, { cwd, encoding: "utf8", maxBuffer: 1 << 28, stdio: ["ignore", "pipe", "pipe"] });

// Licenses that can be combined with a GPL-3.0 work.
const COMPATIBLE = new Set(["MIT", "ISC", "BSD-2-Clause", "BSD-3-Clause", "0BSD", "Apache-2.0", "MPL-2.0", "Unlicense", "CC0-1.0", "Zlib", "BlueOak-1.0.0"]);

function normalize(expr) {
  const s = String(Array.isArray(expr) ? expr.join(" AND ") : expr).replace(/[*()]/g, "").trim();
  return s;
}
// "A OR B": one compatible choice is enough. "A AND B": all must be compatible.
function compatible(expr) {
  const ors = normalize(expr).split(/\s+OR\s+/);
  return ors.some((o) => o.split(/\s+AND\s+/).every((id) => COMPATIBLE.has(id.trim())));
}

function guessSpdx(text) {
  if (/Apache License/i.test(text) && /Version 2\.0/.test(text)) return "Apache-2.0";
  if (/Mozilla Public License/i.test(text) && /2\.0/.test(text)) return "MPL-2.0";
  if (/Permission is hereby granted, free of charge/i.test(text)) return "MIT";
  if (/Redistribution and use in source and binary forms/i.test(text)) return /Neither the name|may be used to endorse/i.test(text) ? "BSD-3-Clause" : "BSD-2-Clause";
  if (/Permission to use, copy, modify, and\/or distribute this software for any purpose with or without fee is hereby granted\.?\s*$/im.test(text) && !/provided that the above copyright/i.test(text)) return "0BSD";
  if (/Permission to use, copy, modify, and\/or distribute/i.test(text)) return "ISC";
  return "UNKNOWN";
}

const pkgs = []; // { ecosystem, name, version, license, source, text, notice }

function noticeNear(dir) {
  if (!dir || !existsSync(dir)) return "";
  const f = readdirSync(dir).find((n) => /^notice(\.(md|txt))?$/i.test(n));
  return f ? readFileSync(path.join(dir, f), "utf8").trim() : "";
}

// ---- web: only the npm packages whose code is actually inside the built bundle.
// Installing "firebase" pulls in many packages (Firestore, Functions, ...) that the app never imports,
// so we read the source map of a real production build instead of the dependency tree.
const tmp = mkdtempSync(path.join(os.tmpdir(), "nodemon-licenses-"));
sh("npx", ["--yes", "vite", "build", "--mode", "firebase", "--sourcemap", "true", "--outDir", tmp, "--emptyOutDir"], path.join(root, "web"));
const webDir = path.join(root, "web");
const pkgDirs = new Set();
const mapsDir = path.join(tmp, "assets");
for (const f of readdirSync(mapsDir).filter((n) => n.endsWith(".map"))) {
  for (const src of JSON.parse(readFileSync(path.join(mapsDir, f), "utf8")).sources) {
    const clean = src.replace(/^\u0000/, "");
    if (/(^|\/)vite\/(preload-helper|modulepreload-polyfill)/.test(clean)) { pkgDirs.add(path.join(webDir, "node_modules", "vite")); continue; }
    const at = clean.lastIndexOf("node_modules/");
    if (at < 0) continue; // our own source
    const first = clean.indexOf("node_modules/");
    const tail = clean.slice(at + "node_modules/".length).split("/");
    const name = tail[0].startsWith("@") ? tail.slice(0, 2).join("/") : tail[0];
    const prefix = clean.slice(first, at + "node_modules/".length); // keeps nested node_modules chains intact
    pkgDirs.add(path.join(webDir, prefix, name));
  }
}
const APACHE_FALLBACK = "/usr/share/common-licenses/Apache-2.0";
for (const dir of [...pkgDirs].sort()) {
  if (!existsSync(path.join(dir, "package.json"))) continue;
  const pj = JSON.parse(readFileSync(path.join(dir, "package.json"), "utf8"));
  const lic = normalize(typeof pj.license === "object" ? pj.license.type : pj.license ?? "UNKNOWN");
  const file = readdirSync(dir).find((n) => /^(licen[cs]e|copying)(\.(md|txt))?$/i.test(n));
  let text = file ? readFileSync(path.join(dir, file), "utf8").trim() : "";
  let note = "";
  if (!text && lic === "Apache-2.0" && existsSync(APACHE_FALLBACK)) {
    text = readFileSync(APACHE_FALLBACK, "utf8").trim();
    note = "the package ships no license file; the standard Apache-2.0 text is supplied" + (/firebase/.test(pj.repository?.url ?? pj.repository ?? "") ? "; copyright Google LLC per its source file headers" : "");
  }
  const repo = typeof pj.repository === "string" ? pj.repository : pj.repository?.url ?? "";
  pkgs.push({ ecosystem: "npm (bundled into the web app)", name: pj.name, version: pj.version, license: lic, source: repo.replace(/^git\+/, "").replace(/^git:\/\//, "https://").replace(/\.git$/, "") || `https://www.npmjs.com/package/${pj.name}`, text, notice: noticeNear(dir), note });
}

// ---- Go modules linked into the server (the connector uses only the standard library)
const mods = sh("go", ["list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}{{end}}", "./..."], path.join(root, "server"))
  .split("\n").filter(Boolean);
for (const line of [...new Set(mods)].sort()) {
  const [name, version, dir] = line.split("|");
  const file = readdirSync(dir).find((n) => /^(licen[cs]e|copying)(\.(md|txt))?$/i.test(n));
  const text = file ? readFileSync(path.join(dir, file), "utf8").trim() : "";
  pkgs.push({ ecosystem: "Go module (server)", name, version, license: guessSpdx(text), source: `https://pkg.go.dev/${name}`, text, notice: noticeNear(dir) });
}

// ---- Go standard library and runtime, which are compiled into every Go binary we build
// Ask from inside the server module so we report the toolchain its go.mod selects, not the system default.
const goroot = sh("go", ["env", "GOROOT"], path.join(root, "server")).trim();
const goVersion = sh("go", ["env", "GOVERSION"], path.join(root, "server")).trim();
pkgs.push({ ecosystem: "Go standard library (server and connector binaries)", name: "go", version: goVersion, license: "BSD-3-Clause", source: "https://go.dev", text: readFileSync(path.join(goroot, "LICENSE"), "utf8").trim(), notice: "" });

// ---- verify
const problems = [];
for (const p of pkgs) {
  if (!compatible(p.license)) problems.push(`${p.name}@${p.version}: license "${p.license}" is not on the GPL-3.0-compatible allow-list`);
  if (!p.text) problems.push(`${p.name}@${p.version}: no license text found in the package`);
}
if (problems.length) {
  console.error("Third-party license check failed:\n - " + problems.join("\n - "));
  process.exit(1);
}

// ---- write: identical license texts are printed once, with the packages that share them
pkgs.sort((a, b) => a.ecosystem.localeCompare(b.ecosystem) || a.name.localeCompare(b.name));
const counts = {};
for (const p of pkgs) counts[p.license] = (counts[p.license] ?? 0) + 1;

const groups = new Map();
for (const p of pkgs) {
  const k = p.license + "\u0000" + p.text;
  if (!groups.has(k)) groups.set(k, { license: p.license, text: p.text, pkgs: [] });
  groups.get(k).pkgs.push(p);
}

let out = `# Third-party licenses

NodeMon is licensed under the GNU General Public License, version 3 or (at your option) any later version
(see [LICENSE](LICENSE)). It includes or links the third-party software listed below, each under its own license.
All of these licenses are compatible with the GPL-3.0. Only software that ships with NodeMon is listed:
the web app's production npm packages, the Go modules linked into the server, and the Go standard library.
Development tools (for example the bundler and type checker) are not distributed and are not listed.

NodeMon does not include any Netdata code, dashboard UI or logo. It reads the local HTTP API of a separately
installed Netdata Agent (GPL-3.0-or-later). Netdata is a trademark of Netdata, Inc.; NodeMon is not affiliated with it.

This file is generated by \`node scripts/third-party-notices.mjs\`. Do not edit it by hand.

## Summary

| License | Packages |
|---|---|
${Object.entries(counts).sort((a, b) => b[1] - a[1]).map(([l, n]) => `| ${l} | ${n} |`).join("\n")}

## Packages

| Package | Version | License | Source |
|---|---|---|---|
${pkgs.map((p) => `| ${p.name} | ${p.version} | ${p.license} | ${p.source}${p.note ? ` (${p.note})` : ""} |`).join("\n")}

## License texts
`;
let i = 0;
for (const g of groups.values()) {
  i++;
  out += `\n### ${i}. ${g.license}\n\nApplies to: ${g.pkgs.map((p) => `${p.name}@${p.version}`).join(", ")}\n\n\`\`\`text\n${g.text}\n\`\`\`\n`;
}
const notices = pkgs.filter((p) => p.notice);
if (notices.length) {
  out += "\n## NOTICE files\n\nThe following packages ship a NOTICE file that must be retained (Apache-2.0, section 4(d)).\n";
  for (const p of notices) out += `\n### ${p.name}@${p.version}\n\n\`\`\`text\n${p.notice}\n\`\`\`\n`;
}
writeFileSync(path.join(root, "THIRD_PARTY_LICENSES.md"), out);
// The deployed web app serves the same text, so the notices travel with the bundled code.
writeFileSync(path.join(root, "web", "public", "third-party-licenses.txt"), out);
console.log(`${pkgs.length} packages, ${groups.size} distinct license texts, ${notices.length} NOTICE files`);
console.log(Object.entries(counts).map(([l, n]) => `${l}: ${n}`).join(", "));
