import { initializeApp } from "firebase/app";
import {
  connectAuthEmulator,
  getAuth,
  GoogleAuthProvider,
  onAuthStateChanged,
  signInWithCredential,
  signInWithPopup,
  signOut,
  type User as FbUser,
} from "firebase/auth";
import {
  connectDatabaseEmulator,
  get,
  getDatabase,
  onValue,
  orderByKey,
  query,
  ref,
  remove,
  set,
  startAt,
} from "firebase/database";
import { RANGES, unauthorized, type Api, type Host, type Point, type Sample } from "./api";

// Reads one value from a Vite env var; a missing one is a build-configuration mistake worth naming.
const env = (k: string): string => (import.meta.env[k] as string | undefined) ?? "";

const config = {
  apiKey: env("VITE_FIREBASE_API_KEY"),
  authDomain: env("VITE_FIREBASE_AUTH_DOMAIN"),
  databaseURL: env("VITE_FIREBASE_DATABASE_URL"),
  projectId: env("VITE_FIREBASE_PROJECT_ID"),
};
const EMULATOR = env("VITE_FIREBASE_EMULATOR") === "1";

const app = initializeApp(config);
const auth = getAuth(app);
const db = getDatabase(app);
if (EMULATOR) {
  connectAuthEmulator(auth, "http://127.0.0.1:9099", { disableWarnings: true });
  connectDatabaseEmulator(db, "127.0.0.1", 9000);
}

// The database clock decides "online"; the browser's own clock may be off.
let skew = 0;
onValue(ref(db, ".info/serverTimeOffset"), (s) => (skew = (s.val() as number) ?? 0));
const serverNowMs = () => Date.now() + skew;

function firstAuthState(): Promise<FbUser | null> {
  return new Promise((resolve) => {
    const off = onAuthStateChanged(auth, (u) => {
      off();
      resolve(u);
    });
  });
}

function requireUser(): FbUser {
  return auth.currentUser ?? unauthorized();
}

const FIELDS = ["cpu", "mem_used", "mem_total", "disk_used", "disk_total", "net_rx", "net_tx", "load1"] as const;
const toSample = (v: Record<string, number>): Sample => ({
  cpu: v.cpu, mem_used: v.mem_used, mem_total: v.mem_total, disk_used: v.disk_used,
  disk_total: v.disk_total, net_rx: v.net_rx, net_tx: v.net_tx, load1: v.load1,
});

const pad = (n: number, w = 2) => String(n).padStart(w, "0");
const dayKey = (sec: number) => { const d = new Date(sec * 1000); return `${d.getUTCFullYear()}${pad(d.getUTCMonth() + 1)}${pad(d.getUTCDate())}`; };
const monthKey = (sec: number) => { const d = new Date(sec * 1000); return `${d.getUTCFullYear()}${pad(d.getUTCMonth() + 1)}`; };

// Reads the buckets of one series (m = minutes by UTC day, h = hours by UTC month) at or after `from`.
async function readBuckets(kind: "m" | "h", id: string, from: number, to: number): Promise<Point[]> {
  const keys: string[] = [];
  if (kind === "m") {
    for (let t = Math.floor(from / 86400) * 86400; t <= to; t += 86400) keys.push(dayKey(t));
  } else {
    const d = new Date(from * 1000);
    let y = d.getUTCFullYear(), m = d.getUTCMonth();
    for (;;) {
      const k = `${y}${pad(m + 1)}`;
      keys.push(k);
      if (k >= monthKey(to)) break;
      if (++m === 12) { m = 0; y++; }
    }
  }
  const snaps = await Promise.all(keys.map((k) => get(query(ref(db, `${kind}/${id}/${k}`), orderByKey(), startAt(String(from))))));
  const out: Point[] = [];
  for (const s of snaps) s.forEach((c) => { out.push({ ...toSample(c.val()), t: c.val().t }); });
  return out.sort((a, b) => a.t - b.t);
}

// Minute and hour buckets never change once written, so a refresh only needs the buckets after the last
// one we already hold. That turns a repeated multi-hour download into a few hundred bytes.
const bucketCache = new Map<string, { from: number; pts: Point[] }>();
const KEEP_SEC = 31 * 86400;

async function readCached(kind: "m" | "h", id: string, from: number, to: number): Promise<Point[]> {
  const key = `${kind}:${id}`;
  const hit = bucketCache.get(key);
  let pts: Point[];
  let coverage = from;
  if (hit && hit.from <= from && hit.pts.length) {
    pts = hit.pts.concat(await readBuckets(kind, id, hit.pts[hit.pts.length - 1].t + 1, to));
    coverage = hit.from;
  } else {
    pts = await readBuckets(kind, id, from, to);
  }
  const floor = to - KEEP_SEC; // bound memory in tabs that stay open for days
  if (coverage < floor) coverage = floor;
  pts = pts.filter((p) => p.t >= coverage);
  bucketCache.set(key, { from: coverage, pts });
  return pts.filter((p) => p.t >= from);
}

function downsample(points: Point[], bucket: number): Point[] {
  const groups = new Map<number, Point[]>();
  for (const p of points) {
    const k = Math.floor(p.t / bucket) * bucket;
    (groups.get(k) ?? groups.set(k, []).get(k)!).push(p);
  }
  return [...groups.entries()].sort((a, b) => a[0] - b[0]).map(([t, g]) => {
    const avg = (f: (typeof FIELDS)[number]) => g.reduce((s, p) => s + p[f], 0) / g.length;
    return { t, cpu: avg("cpu"), mem_used: avg("mem_used"), mem_total: avg("mem_total"), disk_used: avg("disk_used"), disk_total: avg("disk_total"), net_rx: avg("net_rx"), net_tx: avg("net_tx"), load1: avg("load1") };
  });
}

// The sparkline needs 30 minutes of history; refreshing it once a minute is plenty and keeps downloads small.
const sparkCache = new Map<string, { at: number; values: (number | null)[] }>();
async function spark(id: string, nowSec: number): Promise<(number | null)[]> {
  const hit = sparkCache.get(id);
  if (hit && nowSec - hit.at < 60) return hit.values;
  const start = Math.floor(nowSec / 60) * 60 - 29 * 60;
  const pts = await readCached("m", id, start, nowSec);
  const values: (number | null)[] = Array(30).fill(null);
  for (const p of pts) {
    const i = Math.round((p.t - start) / 60);
    if (i >= 0 && i < 30) values[i] = p.cpu;
  }
  sparkCache.set(id, { at: nowSec, values });
  return values;
}

const CODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

export const firebaseApi: Api = {
  async me() {
    const u = auth.currentUser ?? (await firstAuthState());
    if (!u || u.isAnonymous) unauthorized();
    return { id: 0, email: u.email ?? "", name: u.displayName ?? u.email ?? "" };
  },

  async login() {
    await signInWithPopup(auth, new GoogleAuthProvider());
  },

  // Only exists so the UI can be exercised against the local emulator without a real Google popup.
  emulatorLogin: EMULATOR
    ? async (email: string) => {
        const idToken = JSON.stringify({ sub: email, email, email_verified: true });
        await signInWithCredential(auth, GoogleAuthProvider.credential(idToken));
      }
    : undefined,

  async overview() {
    const u = requireUser();
    const idx = ((await get(ref(db, `users/${u.uid}/devices`))).val() ?? {}) as Record<string, { name: string; os: string; createdAt?: number }>;
    const nowMs = serverNowMs();
    const nowSec = Math.floor(nowMs / 1000);
    const hosts: Host[] = await Promise.all(
      Object.entries(idx).map(async ([id, d]) => {
        const latest = (await get(ref(db, `latest/${id}`))).val() as (Record<string, number> & { ts: number }) | null;
        const lastSeen = latest ? Math.floor(latest.ts / 1000) : 0;
        return {
          id,
          created_at: d.createdAt ?? 0,
          name: d.name,
          os: d.os,
          online: latest != null && nowMs - latest.ts < 60_000,
          last_seen: lastSeen,
          latest: latest ? toSample(latest) : null,
          cpu_spark: await spark(id, nowSec),
        };
      }),
    );
    hosts.sort((a, b) => a.name.localeCompare(b.name));
    return { hosts, now: nowSec };
  },

  async metrics(id, range) {
    requireUser();
    const sec = RANGES.find((r) => r.key === range)?.sec ?? 3600;
    const to = Math.floor(serverNowMs() / 1000);
    const from = to - sec;
    const long = sec > 24 * 3600; // week/month views read hourly averages instead of minute ones
    const pts = [...(await readCached(long ? "h" : "m", id, from, to))]; // copy: the cache must not gain the live point below
    // Extend the chart to "now" with the live value, which is ahead of the last finished minute.
    const latest = (await get(ref(db, `latest/${id}`))).val() as (Record<string, number> & { ts: number }) | null;
    if (latest && Math.floor(latest.ts / 1000) > (pts[pts.length - 1]?.t ?? 0) && Math.floor(latest.ts / 1000) >= from) {
      pts.push({ ...toSample(latest), t: Math.floor(latest.ts / 1000) });
    }
    const bucket = Math.max(long ? 3600 : 60, Math.floor(sec / 300));
    return { points: downsample(pts, bucket), bucket };
  },

  async deleteHost(id) {
    const u = requireUser();
    // The device record goes last: the rules use it to decide that you own the other nodes.
    for (const path of [`users/${u.uid}/devices/${id}`, `latest/${id}`, `m/${id}`, `h/${id}`, `devices/${id}`]) {
      await remove(ref(db, path));
    }
  },

  async createCode() {
    const u = requireUser();
    const raw = crypto.getRandomValues(new Uint8Array(12));
    const code = Array.from(raw, (b) => CODE_ALPHABET[b % CODE_ALPHABET.length]).join("");
    const expMs = Math.floor(serverNowMs()) + 15 * 60_000;
    await set(ref(db, `pairings/${code}`), { owner: u.uid, exp: expMs });
    const shown = `${code.slice(0, 4)}-${code.slice(4, 8)}-${code.slice(8)}`;
    return {
      code: shown,
      expires_at: Math.floor(expMs / 1000),
      args: `--firebase-api-key ${config.apiKey} --firebase-db-url ${config.databaseURL} --code ${shown}`,
    };
  },

  async logout() {
    await signOut(auth);
  },
};
