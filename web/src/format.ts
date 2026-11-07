// SPDX-License-Identifier: GPL-3.0-or-later
export const pct = (used: number, total: number) => (total > 0 ? (used / total) * 100 : 0);

const UNITS = ["B", "KB", "MB", "GB", "TB"];
export function fmtBytes(v: number): string {
  let i = 0;
  while (v >= 1024 && i < UNITS.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${UNITS[i]}`;
}
export const fmtRate = (v: number) => `${fmtBytes(v)}/s`;
export const fmtPct = (v: number) => `${v === 0 || v >= 10 ? v.toFixed(0) : v.toFixed(1)}%`;

const pad = (n: number) => String(n).padStart(2, "0");
export function fmtTime(t: number, withDate: boolean): string {
  const d = new Date(t * 1000);
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  return withDate ? `${d.getMonth() + 1}/${d.getDate()} ${hm}` : hm;
}

export function fmtAgo(t: number, now: number): string {
  if (!t) return "수신 기록 없음";
  const s = Math.max(0, now - t);
  if (s < 60) return `${s}초 전`;
  if (s < 3600) return `${Math.floor(s / 60)}분 전`;
  if (s < 86400) return `${Math.floor(s / 3600)}시간 전`;
  return `${Math.floor(s / 86400)}일 전`;
}

// Hosts get a categorical slot by registration order, so a host keeps its color when others come and go.
export function hostColors(hosts: { id: string; created_at: number }[]): Map<string, string> {
  const m = new Map<string, string>();
  [...hosts]
    .sort((a, b) => a.created_at - b.created_at || a.id.localeCompare(b.id))
    .forEach((h, i) => m.set(h.id, i < 8 ? `var(--series-${i + 1})` : "var(--muted)"));
  return m;
}

// "3일 4시간", "5시간 12분", "7분": how long a computer has been running since its last boot.
export function fmtUptime(sec: number): string {
  const d = Math.floor(sec / 86400), h = Math.floor((sec % 86400) / 3600), m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d}일 ${h}시간`;
  if (h > 0) return `${h}시간 ${m}분`;
  return m > 0 ? `${m}분` : "1분 미만";
}

export function fmtDateTime(t: number): string {
  const d = new Date(t * 1000);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

const semver = (v: string) => v.match(/(\d+)\.(\d+)\.(\d+)/)?.slice(1, 4).map(Number) ?? null;

// True when a connector's version is clearly older than the dashboard's release. Versions that are not releases
// (a commit id, "dev") cannot be compared, so they never count as old.
export function olderThan(connector: string | null | undefined, dashboard: string): boolean {
  const a = connector ? semver(connector) : null, b = semver(dashboard);
  if (!a || !b) return false;
  for (let i = 0; i < 3; i++) if (a[i] !== b[i]) return a[i] < b[i];
  return false;
}
