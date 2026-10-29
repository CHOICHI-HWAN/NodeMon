import type { Point } from "./api";
import type { Series } from "./LineChart";

export type HostRef = { id: string; name: string; color: string };

// Server buckets share the same grid for one range, so timestamps line up across hosts.
export function mergeHosts(hosts: HostRef[], data: Map<string, Point[]>, pick: (p: Point) => number) {
  const xs = [...new Set(hosts.flatMap((h) => (data.get(h.id) ?? []).map((p) => p.t)))].sort((a, b) => a - b);
  const idx = new Map(xs.map((t, i) => [t, i]));
  const series: Series[] = hosts.map((h) => {
    const values: (number | null)[] = xs.map(() => null);
    for (const p of data.get(h.id) ?? []) values[idx.get(p.t) as number] = pick(p);
    return { key: h.id, label: h.name, color: h.color, values };
  });
  return { xs, series };
}
