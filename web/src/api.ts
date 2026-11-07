// SPDX-License-Identifier: GPL-3.0-or-later
import { demoApi } from "./demo";
import { DEMO } from "./env";

export type Sample = {
  cpu: number;
  mem_used: number;
  mem_total: number;
  disk_used: number;
  disk_total: number;
  net_rx: number;
  net_tx: number;
  load1: number;
};

export type Host = {
  id: string;
  created_at: number; // registration order decides a computer's color, so colors never shift when others are added
  name: string;
  os: string;
  online: boolean;
  last_seen: number;
  latest: Sample | null;
  cpu_spark: (number | null)[];
  uptime?: number | null; // seconds since the computer booted, as of now; null while it is offline or does not report it
  connector?: string | null; // the connector's version as it reported it (e.g. v0.1.0); null for connectors that predate this
  apps?: App[] | null; // the programs that used the most CPU or memory at the last upload; null where the computer does not report them
  usage?: Usage | null; // data this computer has sent to / received from the database; null if it does not report it
};

// Bytes, counted by the connector at the network socket: includes encryption and protocol overhead.
export type Usage = { up: number; down: number; up24h: number | null };

export type Point = Sample & { t: number; up?: number; down?: number; apps?: App[] };

// One program (Netdata groups processes by name, e.g. nginx or chrome). cpu: percent of ONE core, so 200 means two
// cores kept busy. mem: resident memory in MiB.
export type App = { name: string; cpu: number; mem: number };
export type AppUsage = {
  name: string;
  cpuAvg: number;
  cpuPeak: number;
  peakAt: number; // unix seconds of the highest CPU reading
  memAvg: number;
  memPeak: number;
  spike: boolean; // the peak was far above the usual level
  series: (number | null)[]; // CPU per chart column; empty for programs that are not drawn
};
export type AppsReport = { apps: AppUsage[]; spanSec: number };
export type User = { email: string; name: string };
// args: the connector command-line arguments that enroll a computer with this code.
export type EnrollCode = { code: string; expires_at: number; args: string };

export class Unauthorized extends Error {}

export function unauthorized(): never {
  window.dispatchEvent(new Event("nm-unauthorized"));
  throw new Unauthorized();
}

export interface Api {
  me(): Promise<User>;
  login?(): Promise<void>;
  emulatorLogin?(email: string): Promise<void>;
  overview(): Promise<{ hosts: Host[]; now: number }>;
  metrics(id: string, range: string): Promise<{ points: Point[]; bucket: number; report: AppsReport | null }>;
  deleteHost(id: string): Promise<void>;
  createCode(): Promise<EnrollCode>;
  logout(): Promise<void>;
}

// The Firebase SDK is large, so it is only pulled in (lazily) when it is actually used: not in the demo build.
function lazyFirebase(): Api {
  const load = () => import("./firebaseApi").then((m) => m.firebaseApi);
  return {
    me: async () => (await load()).me(),
    login: async () => (await load()).login!(),
    emulatorLogin: import.meta.env.VITE_FIREBASE_EMULATOR === "1" ? async (email) => (await load()).emulatorLogin!(email) : undefined,
    overview: async () => (await load()).overview(),
    metrics: async (id, range) => (await load()).metrics(id, range),
    deleteHost: async (id) => (await load()).deleteHost(id),
    createCode: async () => (await load()).createCode(),
    logout: async () => (await load()).logout(),
  };
}

export const api: Api = DEMO ? demoApi : lazyFirebase();

export const RANGES = [
  { key: "15m", label: "15분", sec: 15 * 60 },
  { key: "1h", label: "1시간", sec: 3600 },
  { key: "6h", label: "6시간", sec: 6 * 3600 },
  { key: "24h", label: "24시간", sec: 24 * 3600 },
  { key: "7d", label: "7일", sec: 7 * 86400 },
  { key: "30d", label: "30일", sec: 30 * 86400 },
] as const;
export type RangeKey = (typeof RANGES)[number]["key"];

// Long ranges change slowly, so refetching them often only burns bandwidth (it matters on the free Firebase plan).
// The connector uploads every few minutes, so polling faster than that finds nothing new. The demo's fake data is
// computed from the clock, so it can refresh quickly and look alive.
export function pollMs(range: RangeKey): number {
  const base = { "15m": 30_000, "1h": 60_000, "6h": 180_000, "24h": 300_000, "7d": 600_000, "30d": 600_000 }[range];
  return DEMO ? base : Math.max(120_000, base);
}
export const OVERVIEW_POLL_MS = DEMO ? 10_000 : 60_000;
