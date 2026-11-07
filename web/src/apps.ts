// SPDX-License-Identifier: GPL-3.0-or-later
// Per-program usage. The connector sends the programs that used the most CPU or memory as one short text,
// "name:cpu:mem|name:cpu:mem" (CPU in percent of ONE core, memory in MiB); this turns it into numbers and
// summarises a time range.
import type { App, AppsReport, AppUsage, Point } from "./api";

export function parseApps(text: unknown): App[] | undefined {
  if (typeof text !== "string" || !text) return undefined;
  const out: App[] = [];
  for (const part of text.split("|")) {
    const last = part.lastIndexOf(":");
    const mid = last > 0 ? part.lastIndexOf(":", last - 1) : -1;
    if (mid <= 0) continue; // a name may not be empty
    const cpu = Number(part.slice(mid + 1, last));
    const mem = Number(part.slice(last + 1));
    if (Number.isFinite(cpu) && Number.isFinite(mem)) out.push({ name: part.slice(0, mid), cpu, mem });
  }
  return out.length ? out : undefined;
}

// The mean over a group of points. A program that was not among the biggest in a point counts as zero there,
// the same rule the connector uses when it averages.
export function meanApps(group: Point[]): App[] | undefined {
  const have = group.filter((p) => p.apps);
  if (!have.length) return undefined;
  const sums = new Map<string, App>();
  for (const p of have) {
    for (const a of p.apps!) {
      const s = sums.get(a.name) ?? { name: a.name, cpu: 0, mem: 0 };
      s.cpu += a.cpu;
      s.mem += a.mem;
      sums.set(a.name, s);
    }
  }
  return [...sums.values()].map((s) => ({ name: s.name, cpu: s.cpu / have.length, mem: s.mem / have.length }));
}

// A program "spiked" when its peak is far above its usual level, and big enough to matter.
export const SPIKE_MIN_CPU = 20;
export const SPIKE_FACTOR = 3;
const CHART_APPS = 5; // how many programs get a line in the chart
const MAX_APPS = 12; // how many rows the table lists

// raw: the buckets as stored (peaks are only visible there); shown: the points drawn, one value per chart column.
export function summarizeApps(raw: Point[], shown: Point[]): AppsReport | null {
  const withApps = raw.filter((p) => p.apps);
  if (!withApps.length) return null;
  const n = withApps.length;
  const acc = new Map<string, { cpu: number; mem: number; peak: number; peakAt: number; memPeak: number }>();
  for (const p of withApps) {
    for (const a of p.apps!) {
      const s = acc.get(a.name) ?? { cpu: 0, mem: 0, peak: 0, peakAt: p.t, memPeak: 0 };
      s.cpu += a.cpu;
      s.mem += a.mem;
      if (a.cpu > s.peak) (s.peak = a.cpu), (s.peakAt = p.t);
      s.memPeak = Math.max(s.memPeak, a.mem);
      acc.set(a.name, s);
    }
  }
  const all: AppUsage[] = [...acc.entries()]
    .map(([name, s]) => {
      const cpuAvg = s.cpu / n;
      return { name, cpuAvg, cpuPeak: s.peak, peakAt: s.peakAt, memAvg: s.mem / n, memPeak: s.memPeak, spike: s.peak >= SPIKE_MIN_CPU && s.peak >= SPIKE_FACTOR * cpuAvg, series: [] as (number | null)[] };
    })
    .sort((a, b) => b.cpuAvg - a.cpuAvg || a.name.localeCompare(b.name));
  // The chart shows the heaviest programs and any that spiked.
  const drawn = new Set([...all.slice(0, CHART_APPS), ...all.filter((a) => a.spike).slice(0, 3)].map((a) => a.name));
  for (const a of all) {
    if (drawn.has(a.name)) a.series = shown.map((p) => (p.apps ? (p.apps.find((x) => x.name === a.name)?.cpu ?? 0) : null));
  }
  // Each stored bucket stands for the time between uploads; the span estimates how long the computer was observed.
  const ts = withApps.map((p) => p.t);
  const gap = n > 1 ? (ts[n - 1] - ts[0]) / (n - 1) : 0;
  return { apps: all.slice(0, MAX_APPS), spanSec: ts[n - 1] - ts[0] + gap };
}
