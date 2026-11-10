// SPDX-License-Identifier: GPL-3.0-or-later
import { useMemo, useState } from "react";
import { api, type Host } from "./api";
import { APP_VERSION, DEMO } from "./env";
import { fmtAgo, fmtBytes, fmtPct, fmtRate, fmtUptime, olderThan, pct } from "./format";
import { Sparkline } from "./Sparkline";
import { Bar, Status, USAGE_HINT } from "./ui";

type SortKey = "name" | "cpu" | "mem" | "disk";
type Filter = "all" | "live" | "offline";

const val = (h: Host, k: SortKey) =>
  k === "name" ? h.name.toLowerCase() : !h.latest ? -1 : k === "cpu" ? h.latest.cpu : k === "mem" ? pct(h.latest.mem_used, h.latest.mem_total) : pct(h.latest.disk_used, h.latest.disk_total);

export function Nodes({ hosts, now, colors, onDeleted, onRename }: { hosts: Host[]; now: number; colors: Map<string, string>; onDeleted: (id: string) => void; onRename: (h: Host) => void }) {
  const [error, setError] = useState<string | null>(null);
  const remove = async (h: Host) => {
    if (!window.confirm(`'${h.name}' 을(를) 삭제할까요? 수집된 데이터도 함께 삭제됩니다.`)) return;
    setError(null);
    try {
      await api.deleteHost(h.id);
      onDeleted(h.id);
    } catch (e) {
      setError(`삭제하지 못했습니다: ${e instanceof Error ? e.message : String(e)}`);
    }
  };
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
  const showUsage = hosts.some((h) => h.usage);
  const showVersion = hosts.some((h) => h.connector);
  const showUptime = hosts.some((h) => h.uptime != null);
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
        <select className="select sort-select" aria-label="정렬" value={`${sort.key}`} onChange={(e) => { const key = e.target.value as SortKey; setSort({ key, dir: key === "name" ? 1 : -1 }); }}>
          <option value="name">이름순</option>
          <option value="cpu">CPU 높은 순</option>
          <option value="mem">메모리 높은 순</option>
          <option value="disk">디스크 높은 순</option>
        </select>
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
      {error && <p className="err">{error}</p>}
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
              {showUsage && <th title={USAGE_HINT}>업로드 ⓘ</th>}
              {showUptime && <th title="컴퓨터가 마지막으로 켜진 뒤 지난 시간">가동 시간 ⓘ</th>}
              {showVersion && <th title="각 컴퓨터에서 실행 중인 Connector 프로그램의 버전">Connector ⓘ</th>}
              <th>마지막 수신</th>
              {!DEMO && <th><span className="sr-only">삭제</span></th>}
            </tr>
          </thead>
          <tbody>
            {rows.map((h) => {
              const l = h.latest;
              return (
                <tr key={h.id} className={h.online ? "" : "offline"}>
                  <td className="c-status"><Status online={h.online} /></td>
                  <td className="c-name">
                    <a className="node-link" href={`#/host/${encodeURIComponent(h.id)}`}>
                      <span className="key" style={{ background: colors.get(h.id) }} />
                      {h.name}
                    </a>
                    {h.alias && h.hostname && <span className="muted hostname"> ({h.hostname})</span>}
                    {!DEMO && <button className="link small" onClick={() => onRename(h)} aria-label={`${h.name} 이름 바꾸기`} title="표시 이름 바꾸기">✎</button>}
                  </td>
                  <td className="muted c-os">{h.os}</td>
                  <td data-label="CPU">{l ? <Cell value={pct(l.cpu, 100)} label="CPU" /> : "–"}</td>
                  <td className="spark-cell c-spark"><Sparkline values={h.cpu_spark} color={colors.get(h.id) ?? "var(--series-1)"} /></td>
                  <td data-label="메모리">{l ? <Cell value={pct(l.mem_used, l.mem_total)} label="메모리" /> : "–"}</td>
                  <td data-label="디스크">{l ? <Cell value={pct(l.disk_used, l.disk_total)} label="디스크" /> : "–"}</td>
                  <td className="net-cell" data-label="네트워크">{l ? <>↓ {fmtRate(l.net_rx)}<br />↑ {fmtRate(l.net_tx)}</> : "–"}</td>
                  {showUsage && (
                  <td data-label="업로드">
                    {h.usage ? (
                      <div className="usage-cell">
                        <strong>{fmtBytes(h.usage.up)}</strong>
                        <span>24시간 {h.usage.up24h != null ? fmtBytes(h.usage.up24h) : "–"}</span>
                      </div>
                    ) : (
                      "–"
                    )}
                  </td>
                  )}
                  {showUptime && <td data-label="가동 시간">{h.uptime != null ? fmtUptime(h.uptime) : "–"}</td>}
                  {showVersion && (
                    <td className="muted" data-label="Connector">
                      {h.connector ?? "–"}
                      {olderThan(h.connector, APP_VERSION) && <span className="badge-old" title="대시보드보다 오래된 버전입니다. 새 Connector로 바꾸면 최신 기능을 쓸 수 있습니다.">구버전</span>}
                    </td>
                  )}
                  <td className="muted" data-label="마지막 수신">{fmtAgo(h.last_seen, now)}</td>
                  {!DEMO && <td className="c-del"><button className="danger small" onClick={() => remove(h)} aria-label={`${h.name} 삭제`}>삭제</button></td>}
                </tr>
              );
            })}
            {rows.length === 0 && (
              <tr>
                <td colSpan={9 + (DEMO ? 0 : 1) + (showVersion ? 1 : 0) + (showUsage ? 1 : 0) + (showUptime ? 1 : 0)} className="muted center">조건에 맞는 노드가 없습니다</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {hosts.some((h) => h.usage) && <p className="table-note">업로드: {USAGE_HINT}</p>}
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
