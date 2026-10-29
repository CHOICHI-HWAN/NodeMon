import { useMemo, useState } from "react";
import type { Host } from "./api";
import { fmtAgo, fmtPct, fmtRate, pct } from "./format";
import { Sparkline } from "./Sparkline";
import { Bar, Status } from "./ui";

type SortKey = "name" | "cpu" | "mem" | "disk";
type Filter = "all" | "live" | "offline";

const val = (h: Host, k: SortKey) =>
  k === "name" ? h.name.toLowerCase() : !h.latest ? -1 : k === "cpu" ? h.latest.cpu : k === "mem" ? pct(h.latest.mem_used, h.latest.mem_total) : pct(h.latest.disk_used, h.latest.disk_total);

export function Nodes({ hosts, now, colors }: { hosts: Host[]; now: number; colors: Map<string, string> }) {
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [sort, setSort] = useState<{ key: SortKey; dir: 1 | -1 }>({ key: "name", dir: 1 });

  const rows = useMemo(() => {
    const term = q.trim().toLowerCase();
    return hosts
      .filter((h) => (filter === "all" ? true : filter === "live" ? h.online : !h.online))
      .filter((h) => !term || h.name.toLowerCase().includes(term) || h.os.toLowerCase().includes(term))
      .sort((a, b) => {
        const x = val(a, sort.key), y = val(b, sort.key);
        return (x < y ? -1 : x > y ? 1 : 0) * sort.dir;
      });
  }, [hosts, q, filter, sort]);

  const live = hosts.filter((h) => h.online).length;
  const th = (key: SortKey, label: string) => (
    <th aria-sort={sort.key === key ? (sort.dir === 1 ? "ascending" : "descending") : "none"}>
      <button className="th-btn" onClick={() => setSort((s) => ({ key, dir: s.key === key ? (s.dir === 1 ? -1 : 1) : key === "name" ? 1 : -1 }))}>
        {label}
        <span aria-hidden="true">{sort.key === key ? (sort.dir === 1 ? " ▲" : " ▼") : ""}</span>
      </button>
    </th>
  );

  return (
    <>
      <div className="filters">
        <input className="search" type="search" placeholder="노드 검색" aria-label="노드 검색" value={q} onChange={(e) => setQ(e.target.value)} />
        <div className="seg" role="group" aria-label="상태">
          {(
            [
              ["all", `전체 ${hosts.length}`],
              ["live", `실시간 ${live}`],
              ["offline", `오프라인 ${hosts.length - live}`],
            ] as [Filter, string][]
          ).map(([k, label]) => (
            <button key={k} className={filter === k ? "active" : ""} aria-pressed={filter === k} onClick={() => setFilter(k)}>
              {label}
            </button>
          ))}
        </div>
      </div>
      <div className="table-card">
        <table className="nodes">
          <thead>
            <tr>
              <th>상태</th>
              {th("name", "노드")}
              <th>OS</th>
              {th("cpu", "CPU")}
              <th>CPU 추이 (30분)</th>
              {th("mem", "메모리")}
              {th("disk", "디스크")}
              <th>네트워크</th>
              <th>마지막 수신</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((h) => {
              const l = h.latest;
              return (
                <tr key={h.id} className={h.online ? "" : "offline"}>
                  <td><Status online={h.online} /></td>
                  <td>
                    <a className="node-link" href={`#/host/${encodeURIComponent(h.id)}`}>
                      <span className="key" style={{ background: colors.get(h.id) }} />
                      {h.name}
                    </a>
                  </td>
                  <td className="muted">{h.os}</td>
                  <td>{l ? <Cell value={pct(l.cpu, 100)} label="CPU" /> : "–"}</td>
                  <td className="spark-cell"><Sparkline values={h.cpu_spark} color={colors.get(h.id) ?? "var(--series-1)"} /></td>
                  <td>{l ? <Cell value={pct(l.mem_used, l.mem_total)} label="메모리" /> : "–"}</td>
                  <td>{l ? <Cell value={pct(l.disk_used, l.disk_total)} label="디스크" /> : "–"}</td>
                  <td className="net-cell">{l ? <>↓ {fmtRate(l.net_rx)}<br />↑ {fmtRate(l.net_tx)}</> : "–"}</td>
                  <td className="muted">{fmtAgo(h.last_seen, now)}</td>
                </tr>
              );
            })}
            {rows.length === 0 && (
              <tr>
                <td colSpan={9} className="muted center">조건에 맞는 노드가 없습니다</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}

function Cell({ value, label }: { value: number; label: string }) {
  return (
    <div className="cell">
      <strong>{fmtPct(value)}</strong>
      <Bar value={value} label={label} />
    </div>
  );
}
