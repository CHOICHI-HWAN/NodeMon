import { api, RANGES, type Host, type Point, type RangeKey } from "./api";
import { fmtAgo, fmtPct, fmtRate, fmtBytes, pct } from "./format";
import { usePoll } from "./hooks";
import { LineChart } from "./LineChart";
import { mergeHosts } from "./merge";
import { Sparkline } from "./Sparkline";

export function Status({ online }: { online: boolean }) {
  return (
    <span className={`status ${online ? "on" : "off"}`}>
      <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
        {online ? <path d="M2 6.5l2.5 2.5L10 3" fill="none" stroke="currentColor" strokeWidth="2" /> : <circle cx="6" cy="6" r="3.5" fill="none" stroke="currentColor" strokeWidth="2" />}
      </svg>
      {online ? "온라인" : "오프라인"}
    </span>
  );
}

export function Meter({ label, value, detail }: { label: string; value: number; detail: string }) {
  return (
    <div className="meter">
      <div className="meter-row">
        <span>{label}</span>
        <strong>{fmtPct(value)}</strong>
      </div>
      <div className="track" role="meter" aria-label={label} aria-valuenow={Math.round(value)} aria-valuemin={0} aria-valuemax={100}>
        <div className="fill" style={{ width: `${Math.min(100, value)}%` }} />
      </div>
      <div className="meter-detail">{detail}</div>
    </div>
  );
}

function HostCard({ h, color, now }: { h: Host; color: string; now: number }) {
  const l = h.latest;
  return (
    <a className={`card host-card${h.online ? "" : " offline"}`} href={`#/host/${h.id}`}>
      <header>
        <span className="key" style={{ background: color }} />
        <h3>{h.name}</h3>
        <Status online={h.online} />
      </header>
      <div className="sub">
        {h.os} · {h.online ? "수집 중" : `마지막 수신 ${fmtAgo(h.last_seen, now)}`}
      </div>
      {l ? (
        <>
          <div className="cpu-row">
            <Meter label="CPU" value={l.cpu} detail={`부하 ${l.load1.toFixed(2)}`} />
            <Sparkline values={h.cpu_spark} color={color} />
          </div>
          <Meter label="메모리" value={pct(l.mem_used, l.mem_total)} detail={`${fmtBytes(l.mem_used)} / ${fmtBytes(l.mem_total)}`} />
          <Meter label="디스크" value={pct(l.disk_used, l.disk_total)} detail={`${fmtBytes(l.disk_used)} / ${fmtBytes(l.disk_total)}`} />
          <div className="net">
            <span>↓ {fmtRate(l.net_rx)}</span>
            <span>↑ {fmtRate(l.net_tx)}</span>
          </div>
        </>
      ) : (
        <p className="sub">아직 수신된 데이터가 없습니다. Connector가 실행 중인지 확인하세요.</p>
      )}
    </a>
  );
}

export function RangePicker({ value, onChange }: { value: RangeKey; onChange: (r: RangeKey) => void }) {
  return (
    <div className="seg" role="group" aria-label="기간">
      {RANGES.map((r) => (
        <button key={r.key} className={r.key === value ? "active" : ""} aria-pressed={r.key === value} onClick={() => onChange(r.key)}>
          {r.label}
        </button>
      ))}
    </div>
  );
}

type Props = {
  hosts: Host[];
  now: number;
  colors: Map<number, string>;
  range: RangeKey;
  onRange: (r: RangeKey) => void;
  onAdd: () => void;
};

export function Overview({ hosts, now, colors, range, onRange, onAdd }: Props) {
  const key = hosts.map((h) => h.id).join(",");
  const sec = RANGES.find((r) => r.key === range)!.sec;
  const { data, loading } = usePoll(
    async () => {
      const entries = await Promise.all(hosts.map(async (h) => [h.id, (await api.metrics(h.id, range)).points] as const));
      return { map: new Map<number, Point[]>(entries), to: Math.floor(Date.now() / 1000) };
    },
    [key, range],
  );

  if (hosts.length === 0) {
    return (
      <div className="card empty-state">
        <h2>등록된 컴퓨터가 없습니다</h2>
        <p>모니터링할 컴퓨터에 Netdata와 NodeMon Connector를 설치하고 이 계정에 등록하세요.</p>
        <button className="primary" onClick={onAdd}>컴퓨터 추가</button>
      </div>
    );
  }

  const refs = hosts.map((h) => ({ id: h.id, name: h.name, color: colors.get(h.id) as string }));
  const map = data?.map ?? new Map<number, Point[]>();
  const to = data?.to ?? now;
  const cpu = mergeHosts(refs, map, (p) => p.cpu);
  const mem = mergeHosts(refs, map, (p) => pct(p.mem_used, p.mem_total));
  const rx = mergeHosts(refs, map, (p) => p.net_rx);
  const online = hosts.filter((h) => h.online).length;

  return (
    <>
      <div className="toolbar">
        <div className="summary">
          전체 {hosts.length}대 · 온라인 {online}대 · 오프라인 {hosts.length - online}대
        </div>
        <RangePicker value={range} onChange={onRange} />
      </div>
      <div className="grid hosts">
        {hosts.map((h) => (
          <HostCard key={h.id} h={h} color={colors.get(h.id) as string} now={now} />
        ))}
      </div>
      <h2 className="section">컴퓨터 비교</h2>
      <div className="grid charts">
        <LineChart title="CPU 사용률" {...cpu} fmt={(v) => fmtPct(v)} from={to - sec} to={to} max={100} loading={loading} />
        <LineChart title="메모리 사용률" {...mem} fmt={(v) => fmtPct(v)} from={to - sec} to={to} max={100} loading={loading} />
        <LineChart title="네트워크 수신" {...rx} fmt={fmtRate} from={to - sec} to={to} loading={loading} />
      </div>
    </>
  );
}
