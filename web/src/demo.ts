// SPDX-License-Identifier: GPL-3.0-or-later
import { summarizeApps } from "./apps";
import type { Api, App, Host, Point } from "./api";

export type DemoMode = "single" | "multi";
export const demoState: { mode: DemoMode } = { mode: "multi" };

type Spec = { id: number; name: string; os: string; cpu: number; mem: number; disk: number; memGB: number; diskGB: number; offlineMin?: number };
const SPECS: Spec[] = [
  { id: 1, name: "dev-workstation", os: "linux", cpu: 35, mem: 60, disk: 0.55, memGB: 16, diskGB: 1000 },
  { id: 2, name: "macbook-pro", os: "darwin", cpu: 20, mem: 70, disk: 0.8, memGB: 16, diskGB: 500 },
  { id: 3, name: "ubuntu-server", os: "linux", cpu: 60, mem: 45, disk: 0.5, memGB: 32, diskGB: 2000 },
  { id: 4, name: "synology-nas", os: "linux", cpu: 8, mem: 30, disk: 0.4, memGB: 8, diskGB: 4000 },
  { id: 5, name: "work-laptop", os: "windows", cpu: 45, mem: 75, disk: 0.65, memGB: 16, diskGB: 512, offlineMin: 40 },
];
const specs = () => (demoState.mode === "single" ? SPECS.slice(0, 1) : SPECS);
const GB = 2 ** 30;

// Cheap deterministic noise so the same timestamp always yields the same value.
const noise = (a: number, b: number) => {
  const x = Math.sin(a * 12.9898 + b * 78.233) * 43758.5453;
  return x - Math.floor(x);
};

function sample(s: Spec, t: number): Omit<Point, "t"> {
  const ph = t / 900 + s.id;
  const burst = Math.floor(t / 1800) % 5 === s.id % 5 ? 30 : 0;
  const cpu = Math.max(1, Math.min(99, s.cpu + 25 * Math.sin(ph) + (noise(t, s.id) - 0.5) * 12 + burst));
  const memTotal = s.memGB * GB;
  const mem = Math.max(0.1, Math.min(0.95, (s.mem + 8 * Math.sin(ph / 3) + (noise(t, s.id + 9) - 0.5) * 2) / 100));
  const diskTotal = s.diskGB * 1e9;
  return {
    cpu,
    mem_used: memTotal * mem,
    mem_total: memTotal,
    disk_used: diskTotal * s.disk,
    disk_total: diskTotal,
    net_rx: 2e5 * noise(t, s.id + 3) * (1 + Math.sin(ph)) * 1.6,
    net_tx: 8e4 * noise(t, s.id + 5) * (1 + Math.cos(ph)) * 1.6,
    load1: (cpu / 100) * 4 + noise(t, s.id + 7) * 0.3,
  };
}

// Programs, as Netdata's apps plugin groups them. Only Linux computers report them, like the real thing.
const PROGRAMS: { name: string; base: number; mem: number }[] = [
  { name: "postgres", base: 9, mem: 900 },
  { name: "node", base: 6, mem: 420 },
  { name: "nginx", base: 3, mem: 140 },
  { name: "chrome", base: 12, mem: 1400 },
  { name: "sshd", base: 0.4, mem: 12 },
];
function programs(s: Spec, t: number): App[] | undefined {
  if (s.os !== "linux") return undefined;
  // About every 50 minutes one program (different per computer) runs hot for ten minutes: the "sudden jump".
  const slot = Math.floor(t / 600), hot = Math.floor(slot / 5) % PROGRAMS.length === s.id % PROGRAMS.length && slot % 5 === 0;
  return PROGRAMS.map((p, i) => ({
    name: p.name,
    cpu: Math.max(0, p.base * (1 + 0.5 * Math.sin(t / 700 + i + s.id)) + noise(t, i + s.id) * 2 + (hot && i === s.id % PROGRAMS.length ? 70 : 0)),
    mem: p.mem * (0.9 + 0.2 * noise(Math.floor(t / 3600), i)),
  }))
    .sort((a, b) => b.cpu - a.cpu)
    .slice(0, 6);
}

const lastTs = (s: Spec, now: number) => (s.offlineMin ? now - s.offlineMin * 60 : now - ((s.id * 3) % 9));

// What a real database would hold: one reading every three minutes, from which peaks can be read.
function rawPoints(s: Spec, rangeSec: number, end: number): Point[] {
  const out: Point[] = [];
  for (let t = Math.ceil((end - rangeSec) / 180) * 180; t <= end; t += 180) out.push({ t, ...sample(s, t), apps: programs(s, t) });
  return out;
}

function points(s: Spec, rangeSec: number, now: number): { points: Point[]; bucket: number; report: ReturnType<typeof summarizeApps> } {
  const bucket = Math.max(1, Math.floor(rangeSec / 300));
  const end = lastTs(s, now);
  const out: Point[] = [];
  for (let t = Math.ceil((now - rangeSec) / bucket) * bucket; t <= end; t += bucket) out.push({ t, ...sample(s, t), apps: programs(s, t) });
  return { points: out, bucket, report: summarizeApps(rawPoints(s, rangeSec, end), out) };
}

const RANGE_SEC: Record<string, number> = { "15m": 900, "1h": 3600, "6h": 21600, "24h": 86400, "7d": 604800, "30d": 2592000 };
const delay = <T,>(v: T) => new Promise<T>((r) => setTimeout(() => r(v), 120));

export const demoApi: Api = {
  me: () => delay({ email: "demo@example.com", name: "Demo" }),
  overview: () => {
    const now = Math.floor(Date.now() / 1000);
    const hosts: Host[] = specs().map((s) => {
      const last = lastTs(s, now);
      const spark: (number | null)[] = [];
      for (let i = 29; i >= 0; i--) {
        const t = Math.floor(now / 60) * 60 - i * 60;
        spark.push(t > last ? null : sample(s, t).cpu);
      }
      const days = 6 + s.id * 3, perDay = 0.9e6 * (0.8 + (s.id % 3) * 0.2);
      return { uptime: s.offlineMin ? null : (s.id * 3 + 2) * 86400 + 3600 * (s.id + 1) + (now % 3600), usage: { up: Math.round(days * perDay), down: Math.round(days * perDay * 0.12), up24h: s.offlineMin ? 0 : Math.round(perDay) }, id: String(s.id), created_at: s.id, name: s.name, os: s.os, online: now - last < 60, last_seen: last, latest: { ...sample(s, last) }, apps: programs(s, last) ?? null, connector: s.id === 4 ? "v0.0.9" : "v0.1.0", cpu_spark: spark };
    });
    return delay({ hosts, now });
  },
  metrics: (id, range) => {
    const s = SPECS.find((x) => x.id === Number(id)) as Spec;
    return delay(points(s, RANGE_SEC[range] ?? 3600, Math.floor(Date.now() / 1000)));
  },
  deleteHost: () => delay<void>(undefined),
  renameHost: () => delay<void>(undefined),
  createCode: () => delay({ code: "7KQ4-M9XD-2WHP", expires_at: Math.floor(Date.now() / 1000) + 900, args: "--firebase-api-key AIza… --firebase-db-url https://your-project-default-rtdb.firebaseio.com --code 7KQ4-M9XD-2WHP" }),
  logout: () => delay<void>(undefined),
};
