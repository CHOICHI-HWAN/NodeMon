import { api, pollMs, RANGES, type Host, type RangeKey } from "./api";
import { DEMO } from "./env";
import { fmtAgo, fmtBytes, fmtPct, fmtRate, pct } from "./format";
import { HoverProvider } from "./hover";
import { usePoll } from "./hooks";
import { LineChart } from "./LineChart";
import { Section, Status, Tile } from "./ui";

type Props = {
  host: Host | undefined;
  color: string;
  now: number;
  range: RangeKey;
  paused: boolean;
  onDeleted: () => void;
};

export function HostDetail({ host, color, now, range, paused, onDeleted }: Props) {
  const id = host?.id ?? "";
  const sec = RANGES.find((r) => r.key === range)!.sec;
  const { data, loading } = usePoll(
    async () => ({ ...(await api.metrics(id, range)), to: Math.floor(Date.now() / 1000) }),
    [id, range],
    paused ? 0 : pollMs(range),
  );
  if (!host) {
    return (
      <div className="empty-state">
        <h2>노드를 찾을 수 없습니다</h2>
        <a href="#/nodes">노드 목록으로 돌아가기</a>
      </div>
    );
  }
  const pts = data?.points ?? [];
  const to = data?.to ?? now;
  const win = { xs: pts.map((p) => p.t), from: to - sec, to, loading, area: true };
  const l = host.latest;
  const remove = async () => {
    if (!window.confirm(`'${host.name}' 을(를) 삭제할까요? 수집된 데이터도 함께 삭제됩니다.`)) return;
    await api.deleteHost(host.id);
    onDeleted();
  };

  return (
    <HoverProvider>
      <div className="node-head">
        <span className="key" style={{ background: color }} />
        <h2>{host.name}</h2>
        <Status online={host.online} />
        <span className="muted">{host.os} · 마지막 수신 {fmtAgo(host.last_seen, now)}</span>
        <div className="grow" />
        {!DEMO && <button className="danger" onClick={remove}>노드 삭제</button>}
      </div>
      {l && (
        <div className="tiles">
          <Tile label="CPU" value={fmtPct(l.cpu)} bar={l.cpu} sub={`부하 ${l.load1.toFixed(2)}`} />
          <Tile label="메모리" value={fmtPct(pct(l.mem_used, l.mem_total))} bar={pct(l.mem_used, l.mem_total)} sub={`${fmtBytes(l.mem_used)} / ${fmtBytes(l.mem_total)}`} />
          <Tile label="디스크" value={fmtPct(pct(l.disk_used, l.disk_total))} bar={pct(l.disk_used, l.disk_total)} sub={`${fmtBytes(l.disk_used)} / ${fmtBytes(l.disk_total)}`} />
          <Tile label="네트워크" value={<span className="net-val">↓ {fmtRate(l.net_rx)}</span>} sub={`↑ ${fmtRate(l.net_tx)}`} />
        </div>
      )}
      <Section title="시스템">
        <div className="grid charts">
          <LineChart title="CPU 사용률" {...win} fmt={fmtPct} max={100} series={[{ key: "cpu", label: "CPU", color, values: pts.map((p) => p.cpu) }]} />
          <LineChart title="메모리 사용률" {...win} fmt={fmtPct} max={100} series={[{ key: "mem", label: "메모리", color, values: pts.map((p) => pct(p.mem_used, p.mem_total)) }]} />
          <LineChart
            title="네트워크"
            {...win}
            fmt={fmtRate}
            area={false}
            series={[
              { key: "rx", label: "수신", color: "var(--series-1)", values: pts.map((p) => p.net_rx) },
              { key: "tx", label: "송신", color: "var(--series-2)", values: pts.map((p) => p.net_tx) },
            ]}
          />
          <LineChart title="디스크 사용량" {...win} fmt={fmtBytes} series={[{ key: "disk", label: "사용 중", color, values: pts.map((p) => p.disk_used) }]} />
          <LineChart title="시스템 부하 (1분)" {...win} fmt={(v) => v.toFixed(2)} series={[{ key: "load", label: "load1", color, values: pts.map((p) => p.load1) }]} />
        </div>
      </Section>
    </HoverProvider>
  );
}
