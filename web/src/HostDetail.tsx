import { api, RANGES, type Host, type RangeKey } from "./api";
import { fmtAgo, fmtBytes, fmtPct, fmtRate, pct } from "./format";
import { usePoll } from "./hooks";
import { LineChart } from "./LineChart";
import { RangePicker, Status } from "./Overview";

type Props = { host: Host | undefined; color: string; now: number; range: RangeKey; onRange: (r: RangeKey) => void; onDeleted: () => void };

export function HostDetail({ host, color, now, range, onRange, onDeleted }: Props) {
  const id = host?.id ?? -1;
  const sec = RANGES.find((r) => r.key === range)!.sec;
  const { data, loading } = usePoll(
    async () => ({ ...(await api.metrics(id, range)), to: Math.floor(Date.now() / 1000) }),
    [id, range],
    5000,
  );
  if (!host) {
    return (
      <div className="card empty-state">
        <h2>컴퓨터를 찾을 수 없습니다</h2>
        <a href="#/">대시보드로 돌아가기</a>
      </div>
    );
  }
  const pts = data?.points ?? [];
  const to = data?.to ?? now;
  const xs = pts.map((p) => p.t);
  const range2 = { from: to - sec, to, xs, loading };
  const remove = async () => {
    if (!window.confirm(`'${host.name}' 을(를) 삭제할까요? 수집된 데이터도 함께 삭제됩니다.`)) return;
    await api.deleteHost(host.id);
    onDeleted();
  };

  return (
    <>
      <a className="back" href="#/">← 전체 보기</a>
      <div className="toolbar">
        <div className="title-row">
          <span className="key" style={{ background: color }} />
          <h2>{host.name}</h2>
          <Status online={host.online} />
          <span className="sub">
            {host.os} · 마지막 수신 {fmtAgo(host.last_seen, now)}
          </span>
        </div>
        <div className="toolbar-right">
          <RangePicker value={range} onChange={onRange} />
          <button className="danger" onClick={remove}>삭제</button>
        </div>
      </div>
      <div className="grid charts">
        <LineChart title="CPU 사용률" {...range2} fmt={fmtPct} max={100} series={[{ key: "cpu", label: "CPU", color, values: pts.map((p) => p.cpu) }]} />
        <LineChart
          title="메모리 사용률"
          {...range2}
          fmt={fmtPct}
          max={100}
          series={[{ key: "mem", label: "메모리", color, values: pts.map((p) => pct(p.mem_used, p.mem_total)) }]}
        />
        <LineChart
          title="네트워크"
          {...range2}
          fmt={fmtRate}
          series={[
            { key: "rx", label: "수신", color: "var(--series-1)", values: pts.map((p) => p.net_rx) },
            { key: "tx", label: "송신", color: "var(--series-2)", values: pts.map((p) => p.net_tx) },
          ]}
        />
        <LineChart
          title="디스크 사용량"
          {...range2}
          fmt={fmtBytes}
          series={[{ key: "disk", label: "사용 중", color, values: pts.map((p) => p.disk_used) }]}
        />
        <LineChart
          title="시스템 부하 (1분)"
          {...range2}
          fmt={(v) => v.toFixed(2)}
          series={[{ key: "load", label: "load1", color, values: pts.map((p) => p.load1) }]}
        />
      </div>
    </>
  );
}
