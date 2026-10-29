import { useEffect, useRef, useState } from "react";
import { fmtTime } from "./format";
import { useHover } from "./hover";

export type Series = { key: string; label: string; color: string; values: (number | null)[] };

type Props = {
  title: string;
  xs: number[];
  series: Series[];
  fmt: (v: number) => string;
  from: number;
  to: number;
  max?: number; // fixed y max (e.g. 100 for percentages)
  height?: number;
  loading?: boolean;
  area?: boolean; // soft fill under the line; used for single-series charts
};

const M = { l: 52, r: 10, t: 8, b: 22 };

function niceMax(v: number): number {
  if (v <= 0) return 1;
  const p = Math.pow(10, Math.floor(Math.log10(v)));
  const n = v / p;
  return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10) * p;
}

const lastIndex = (vals: (number | null)[]) => {
  for (let i = vals.length - 1; i >= 0; i--) if (vals[i] != null) return i;
  return -1;
};

export function LineChart({ title, xs, series, fmt, from, to, max, height = 180, loading, area }: Props) {
  const wrap = useRef<HTMLDivElement>(null);
  const [w, setW] = useState(600);
  const [local, setLocal] = useState(false); // pointer or keyboard focus is on this chart
  const [table, setTable] = useState(false);
  const { t: sharedT, set: setShared } = useHover();

  useEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setW(el.clientWidth));
    ro.observe(el);
    setW(el.clientWidth);
    return () => ro.disconnect();
  }, [table]);

  const iw = Math.max(10, w - M.l - M.r);
  const ih = height - M.t - M.b;
  const dataMax = Math.max(0, ...series.flatMap((s) => s.values.filter((v): v is number => v != null)));
  const yMax = max ?? niceMax(dataMax * 1.1);
  const x = (t: number) => M.l + ((t - from) / Math.max(1, to - from)) * iw;
  const y = (v: number) => M.t + ih - (Math.min(v, yMax) / yMax) * ih;
  const y0 = M.t + ih;
  const withDate = to - from > 24 * 3600;

  const drawn = series.map((s) => {
    const runs: [number, number][][] = [];
    s.values.forEach((v, i) => {
      if (v == null) {
        runs.push([]);
        return;
      }
      if (!runs.length) runs.push([]);
      runs[runs.length - 1].push([x(xs[i]), y(v)]);
    });
    const live = runs.filter((r) => r.length);
    const line = live.map((r) => r.map(([px, py], i) => `${i ? "L" : "M"}${px.toFixed(1)} ${py.toFixed(1)}`).join("")).join("");
    const fill = live
      .filter((r) => r.length > 1)
      .map((r) => `${r.map(([px, py], i) => `${i ? "L" : "M"}${px.toFixed(1)} ${py.toFixed(1)}`).join("")}L${r[r.length - 1][0].toFixed(1)} ${y0}L${r[0][0].toFixed(1)} ${y0}Z`)
      .join("");
    const dots = live.filter((r) => r.length === 1).map((r) => r[0]);
    return { line, fill, dots };
  });

  // The hovered index comes from the shared time, so every chart on the page lines up.
  const step = xs.length > 1 ? (xs[xs.length - 1] - xs[0]) / (xs.length - 1) : 60;
  let hi: number | null = null;
  if (sharedT != null && xs.length) {
    let best = 0;
    for (let i = 1; i < xs.length; i++) if (Math.abs(xs[i] - sharedT) < Math.abs(xs[best] - sharedT)) best = i;
    if (Math.abs(xs[best] - sharedT) <= step * 1.5) hi = best;
  }

  const onMove = (e: React.PointerEvent<SVGRectElement>) => {
    if (!xs.length) return;
    const px = e.clientX - e.currentTarget.getBoundingClientRect().left;
    const t = from + (px / iw) * (to - from);
    let best = 0;
    for (let i = 1; i < xs.length; i++) if (Math.abs(xs[i] - t) < Math.abs(xs[best] - t)) best = i;
    setLocal(true);
    setShared(xs[best]);
  };
  const leave = () => {
    setLocal(false);
    setShared(null);
  };
  const onKey = (e: React.KeyboardEvent) => {
    if (!xs.length) return;
    const cur = hi ?? (e.key === "ArrowLeft" ? xs.length : -1);
    if (e.key === "ArrowLeft") setShared(xs[Math.max(0, cur - 1)]);
    else if (e.key === "ArrowRight") setShared(xs[Math.min(xs.length - 1, cur + 1)]);
    else if (e.key === "Escape") leave();
    else return;
    setLocal(true);
  };

  const readIdx = (s: Series) => (hi != null ? hi : lastIndex(s.values));
  const readout = (s: Series) => {
    const i = readIdx(s);
    return i >= 0 && s.values[i] != null ? fmt(s.values[i] as number) : "–";
  };

  const ticks = [0, 0.25, 0.5, 0.75, 1];
  const xTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => from + f * (to - from));
  const tipLeft = hi != null ? x(xs[hi]) : 0;
  const flip = tipLeft > w * 0.6;

  return (
    <section className="chart" aria-label={title}>
      <header className="chart-head">
        <h3>{title}</h3>
        {series.length === 1 && <span className="readout">{readout(series[0])}</span>}
        <button className="link" onClick={() => setTable((t) => !t)} aria-pressed={table}>
          {table ? "차트" : "표"}
        </button>
      </header>
      {table ? (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>시각</th>
                {series.map((s) => (
                  <th key={s.key}>{s.label}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {xs
                .map((_, i) => i)
                .reverse()
                .slice(0, 200)
                .map((i) => (
                  <tr key={xs[i]}>
                    <td>{fmtTime(xs[i], true)}</td>
                    {series.map((s) => (
                      <td key={s.key}>{s.values[i] == null ? "–" : fmt(s.values[i] as number)}</td>
                    ))}
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      ) : (
        <>
          <div ref={wrap} className="chart-body" style={{ opacity: loading ? 0.5 : 1 }}>
            <svg width={w} height={height} role="img" aria-label={title} tabIndex={0} onKeyDown={onKey} onBlur={leave}>
              {ticks.map((f) => {
                const v = f * yMax;
                return (
                  <g key={f}>
                    <line className={f === 0 ? "axis" : "gl"} x1={M.l} x2={M.l + iw} y1={y(v)} y2={y(v)} />
                    <text className="tick" x={M.l - 6} y={y(v) + 4} textAnchor="end">
                      {fmt(v)}
                    </text>
                  </g>
                );
              })}
              {xTicks.map((t, i) => (
                <text key={i} className="tick" x={x(t)} y={height - 6} textAnchor={i === 0 ? "start" : i === xTicks.length - 1 ? "end" : "middle"}>
                  {fmtTime(t, withDate)}
                </text>
              ))}
              {series.map((s, i) => (
                <g key={s.key}>
                  {area && series.length === 1 && <path d={drawn[i].fill} fill={s.color} fillOpacity={0.16} stroke="none" />}
                  <path d={drawn[i].line} fill="none" stroke={s.color} strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
                  {drawn[i].dots.map(([cx, cy], j) => (
                    <circle key={j} cx={cx} cy={cy} r={3} fill={s.color} />
                  ))}
                </g>
              ))}
              {hi != null && (
                <g pointerEvents="none">
                  <line className="cross" x1={x(xs[hi])} x2={x(xs[hi])} y1={M.t} y2={y0} />
                  {series.map((s) =>
                    s.values[hi!] == null ? null : <circle key={s.key} cx={x(xs[hi!])} cy={y(s.values[hi!] as number)} r={4} fill={s.color} className="ring" />,
                  )}
                </g>
              )}
              <rect x={M.l} y={M.t} width={iw} height={ih} fill="transparent" onPointerMove={onMove} onPointerLeave={leave} />
            </svg>
            {xs.length === 0 && <div className="empty">{loading ? "불러오는 중…" : "이 기간에 수집된 데이터가 없습니다"}</div>}
            {hi != null && local && (
              <div className="tip" style={{ left: tipLeft, transform: flip ? "translateX(calc(-100% - 12px))" : "translateX(12px)" }}>
                <div className="tip-time">{fmtTime(xs[hi], true)}</div>
                {series.map((s) =>
                  s.values[hi!] == null ? null : (
                    <div className="tip-row" key={s.key}>
                      <span className="key" style={{ background: s.color }} />
                      <strong>{fmt(s.values[hi!] as number)}</strong>
                      <span className="tip-label">{s.label}</span>
                    </div>
                  ),
                )}
              </div>
            )}
          </div>
          {series.length >= 2 && (
            <ul className="legend">
              {series.map((s) => (
                <li key={s.key}>
                  <span className="key" style={{ background: s.color }} />
                  <span className="legend-name">{s.label}</span>
                  <strong>{readout(s)}</strong>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </section>
  );
}
