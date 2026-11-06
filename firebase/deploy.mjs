#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-or-later
// One-shot deploy: node firebase/deploy.mjs [--emails a@x.com,b@y.com] [--yes] [--dry-run]
// Needs only Node.js 18+. Reads firebase/project.config.json (public Firebase identifiers).
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createInterface } from "node:readline/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { buildRules } from "./gen-rules.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "..");
const web = path.join(root, "web");
const FIREBASE_TOOLS = "firebase-tools@13.35.1"; // pinned so a new major release cannot change behavior
const win = process.platform === "win32";

const args = process.argv.slice(2);
const flag = (n) => args.includes(n);
const opt = (n) => (args.includes(n) ? args[args.indexOf(n) + 1] : undefined);
const dryRun = flag("--dry-run");

const log = (m = "") => console.log(m);
let stepNo = 0;
const step = (m) => log(`\n[${++stepNo}] ${m}`);
const die = (m) => { console.error(`\n✖ ${m}`); process.exit(1); };

function run(cmd, cmdArgs, { cwd = root, quiet = false } = {}) {
  const r = spawnSync(cmd, cmdArgs, { cwd, stdio: quiet ? "pipe" : "inherit", shell: win, encoding: "utf8" });
  return r;
}
const npx = (a, o) => run("npx", ["--yes", FIREBASE_TOOLS, ...a], o);

async function ask(q) {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  try { return (await rl.question(q)).trim(); } finally { rl.close(); }
}

// ---- 0. prerequisites and config
if (Number(process.versions.node.split(".")[0]) < 18) die(`Node.js 18 or newer is required (found ${process.versions.node}).`);
const cfg = JSON.parse(readFileSync(path.join(here, "project.config.json"), "utf8"));
for (const k of ["projectId", "site", "apiKey", "authDomain", "databaseURL"]) if (!cfg[k]) die(`firebase/project.config.json is missing "${k}"`);
cfg.databaseURL = cfg.databaseURL.replace(/\/+$/, "");
if (!/^https:\/\/[a-z0-9-]+(\.[a-z0-9-]+)*\.(firebaseio\.com|firebasedatabase\.app)$/.test(cfg.databaseURL)) die(`databaseURL does not look like a Realtime Database address: ${cfg.databaseURL}`);

// Accounts allowed to sign in: --emails, ALLOWED_EMAILS, the value remembered from last time, or a prompt.
const cache = path.join(here, ".allowed-emails");
let emails = opt("--emails") ?? process.env.ALLOWED_EMAILS ?? (existsSync(cache) ? readFileSync(cache, "utf8") : "");
if (!emails.trim()) emails = await ask("로그인을 허용할 Google 계정 (쉼표로 여러 개): ");
const list = emails.split(",").map((s) => s.trim()).filter(Boolean);
try { buildRules(list); } catch (e) { die(e.message); }
writeFileSync(cache, list.join(",") + "\n");

log(`프로젝트      : ${cfg.projectId}`);
log(`호스팅 사이트 : https://${cfg.site}.web.app`);
log(`데이터베이스  : ${cfg.databaseURL}`);
log(`허용 계정     : ${list.join(", ")}`);
if (dryRun) log("(--dry-run: 로그인과 배포는 건너뜁니다)");

// ---- 1. project files
step("설정 파일 생성");
writeFileSync(path.join(root, ".firebaserc"), JSON.stringify({ projects: { default: cfg.projectId } }, null, 2) + "\n");
const fj = path.join(root, "firebase.json");
const fbJson = JSON.parse(readFileSync(fj, "utf8"));
if (fbJson.hosting.site !== cfg.site) {
  fbJson.hosting.site = cfg.site;
  writeFileSync(fj, JSON.stringify(fbJson, null, 2) + "\n");
}
writeFileSync(
  path.join(web, ".env.firebase.local"),
  [
    `VITE_FIREBASE_API_KEY=${cfg.apiKey}`,
    `VITE_FIREBASE_AUTH_DOMAIN=${cfg.authDomain}`,
    `VITE_FIREBASE_DATABASE_URL=${cfg.databaseURL}`,
    `VITE_FIREBASE_PROJECT_ID=${cfg.projectId}`,
    "",
  ].join("\n"),
);
log("  .firebaserc, web/.env.firebase.local");

// ---- 2. security rules
step("보안 규칙 생성");
writeFileSync(path.join(here, "database.rules.json"), JSON.stringify(buildRules(list), null, 2) + "\n");
log(`  firebase/database.rules.json (${list.length}개 계정)`);

// ---- 3. build the dashboard
step("대시보드 빌드 (처음에는 몇 분 걸릴 수 있습니다)");
if (run("npm", [existsSync(path.join(web, "package-lock.json")) ? "ci" : "install", "--no-audit", "--no-fund"], { cwd: web }).status !== 0) die("npm install 실패");
if (run("npm", ["run", "build:firebase"], { cwd: web }).status !== 0) die("빌드 실패");
const built = readFileSync(path.join(web, "dist-firebase", "index.html"), "utf8");
if (!built.includes("<script")) die("빌드 결과가 비어 있습니다");

if (dryRun) {
  log("\n✔ dry-run 완료. 위 단계가 모두 통과했습니다. 배포하려면 --dry-run 없이 다시 실행하세요.");
  process.exit(0);
}

// ---- 4. sign in to Firebase
step("Firebase 로그인 확인");
const projects = () => {
  const r = npx(["projects:list", "--json"], { quiet: true });
  if (r.status !== 0) return null;
  try { return JSON.parse(r.stdout).result.map((p) => p.projectId); } catch { return null; }
};
let ids = projects();
if (!ids) {
  log("  로그인이 필요합니다. 브라우저가 열리면 Firebase 프로젝트가 있는 Google 계정으로 로그인하세요.");
  if (npx(["login"]).status !== 0) die("로그인 실패");
  ids = projects();
}
if (!ids) die("프로젝트 목록을 읽지 못했습니다. 로그인 상태를 확인하세요.");
if (!ids.includes(cfg.projectId)) die(`이 계정에서 '${cfg.projectId}' 프로젝트에 접근할 수 없습니다. 접근 가능한 프로젝트: ${ids.join(", ") || "(없음)"}`);
log(`  '${cfg.projectId}' 접근 확인`);

// ---- 5. confirm, then deploy
step("배포");
log(`  - Realtime Database 규칙이 이 DB의 기존 규칙을 '교체'합니다: ${cfg.databaseURL}`);
log(`  - 호스팅은 사이트 '${cfg.site}'에만 올라갑니다 (다른 사이트는 건드리지 않음)`);
if (!flag("--yes") && (await ask("계속할까요? [y/N] ")).toLowerCase() !== "y") die("취소했습니다.");
if (npx(["deploy", "--only", "database,hosting", "--project", cfg.projectId]).status !== 0) die("배포 실패. 위 오류 메시지를 확인하세요.");

log(`
✔ 배포 완료

  대시보드 : https://${cfg.site}.web.app

  남은 일 (콘솔에서 한 번만):
   1. Authentication > 로그인 방법: Google 과 익명 을 모두 사용 설정
      https://console.firebase.google.com/project/${cfg.projectId}/authentication/providers
   2. 로그인 팝업에서 auth/unauthorized-domain 오류가 나면
      Authentication > 설정 > 승인된 도메인에 ${cfg.site}.web.app 추가
   3. https://${cfg.site}.web.app 에서 Google로 로그인 > '컴퓨터 추가'에서 나온 명령을 노드 PC에서 실행
`);
