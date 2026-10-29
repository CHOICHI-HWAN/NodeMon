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

// Hosts get a categorical slot by id order, so a host keeps its color when others come and go.
export function hostColors(ids: number[]): Map<number, string> {
  const m = new Map<number, string>();
  [...ids].sort((a, b) => a - b).forEach((id, i) => m.set(id, i < 8 ? `var(--series-${i + 1})` : "var(--muted)"));
  return m;
}
