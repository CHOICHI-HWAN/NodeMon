import { demoApi } from "./demo";

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
};

export type Point = Sample & { t: number };
export type User = { id: number; email: string; name: string };
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
  metrics(id: string, range: string): Promise<{ points: Point[]; bucket: number }>;
  deleteHost(id: string): Promise<void>;
  createCode(): Promise<EnrollCode>;
  logout(): Promise<void>;
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { credentials: "same-origin", ...init });
  if (res.status === 401) unauthorized();
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.status === 204 ? (undefined as T) : res.json();
}

const realApi: Api = {
  me: () => req<User>("/api/me"),
  overview: async () => {
    const o = await req<{ hosts: (Omit<Host, "id" | "created_at"> & { id: number })[]; now: number }>("/api/overview");
    return { now: o.now, hosts: o.hosts.map((h) => ({ ...h, id: String(h.id), created_at: h.id })) };
  },
  metrics: (id, range) =>
    req<{ points: Point[]; bucket: number }>(`/api/hosts/${id}/metrics?range=${range}`),
  deleteHost: (id) => req<void>(`/api/hosts/${id}`, { method: "DELETE" }),
  createCode: async () => {
    const c = await req<{ code: string; expires_at: number; server: string }>("/api/enroll-codes", { method: "POST" });
    return { code: c.code, expires_at: c.expires_at, args: `--server ${c.server} --code ${c.code}` };
  },
  logout: () => req<void>("/auth/logout", { method: "POST" }),
};

// The Firebase SDK is large, so it is only pulled in (lazily) when that backend is selected.
// The env comparisons are written inline so the bundler can drop the unused branches.
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

export const api: Api =
  import.meta.env.VITE_DEMO === "1" ? demoApi : import.meta.env.VITE_BACKEND === "firebase" ? lazyFirebase() : realApi;

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
// With Firebase the connector uploads about once a minute, so polling faster than that finds nothing new.
const FIREBASE = import.meta.env.VITE_BACKEND === "firebase";
export function pollMs(range: RangeKey): number {
  const base = { "15m": 30_000, "1h": 60_000, "6h": 180_000, "24h": 300_000, "7d": 600_000, "30d": 600_000 }[range];
  return FIREBASE ? Math.max(60_000, base) : base;
}
export const OVERVIEW_POLL_MS = FIREBASE ? 30_000 : 10_000;
