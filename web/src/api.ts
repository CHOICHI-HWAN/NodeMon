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
  id: number;
  name: string;
  os: string;
  online: boolean;
  last_seen: number;
  latest: Sample | null;
  cpu_spark: (number | null)[];
};

export type Point = Sample & { t: number };
export type User = { id: number; email: string; name: string };
export type EnrollCode = { code: string; expires_at: number; server: string };

export class Unauthorized extends Error {}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { credentials: "same-origin", ...init });
  if (res.status === 401) {
    window.dispatchEvent(new Event("nm-unauthorized"));
    throw new Unauthorized();
  }
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.status === 204 ? (undefined as T) : res.json();
}

export const api = {
  me: () => req<User>("/api/me"),
  overview: () => req<{ hosts: Host[]; now: number }>("/api/overview"),
  metrics: (id: number, range: string) =>
    req<{ points: Point[]; bucket: number }>(`/api/hosts/${id}/metrics?range=${range}`),
  deleteHost: (id: number) => req<void>(`/api/hosts/${id}`, { method: "DELETE" }),
  createCode: () => req<EnrollCode>("/api/enroll-codes", { method: "POST" }),
  logout: () => req<void>("/auth/logout", { method: "POST" }),
};

export const RANGES = [
  { key: "15m", label: "15분", sec: 15 * 60 },
  { key: "1h", label: "1시간", sec: 3600 },
  { key: "6h", label: "6시간", sec: 6 * 3600 },
  { key: "24h", label: "24시간", sec: 24 * 3600 },
  { key: "7d", label: "7일", sec: 7 * 86400 },
  { key: "30d", label: "30일", sec: 30 * 86400 },
] as const;
export type RangeKey = (typeof RANGES)[number]["key"];
