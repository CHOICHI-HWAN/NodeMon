import { api, pollMs, RANGES, type Host, type Point, type RangeKey } from "./api";
import { fmtBytes, fmtPct, fmtRate, pct } from "./format";
import { HoverProvider } from "./hover";
import { usePoll } from "./hooks";
import { LineChart } from "./LineChart";
import { mergeHosts } from "./merge";
import { Section, Tile } from "./ui";

type Props = {
  hosts: Host[];
  now: number;
  colors: Map<string, string>;
  range: RangeKey;
  paused: boolean;
  onAdd: () => void;
};

export function Overview({ hosts, now, colors, range, paused, onAdd }: Props) {
  const key = hosts.map((h) => h.id).join(",");
  const sec = RANGES.find((r) => r.key === range)!.sec;
  const { data, loading } = usePoll(
    async () => {
      const entries = await Promise.all(hosts.map(async (h) => [h.id, (await api.metrics(h.id, range)).points] as const));
      return { map: new Map<string, Point[]>(entries), to: Math.floor(Date.now() / 1000) };
    },
    [key, range],
    paused ? 0 : pollMs(range),
  );

  if (hosts.length === 0) {
    return (
      <div className="empty-state">
        <h2>등록된 컴퓨터가 없습니다</h2>
        <p>모니터링할 컴퓨터에 Netdata와 NodeMon Connector를 설치하고 이 계정에 등록하세요.</p>
        <button className="primary" onClick={onAdd}>컴퓨터 추가</button>
      </div>
    );
  }

  const live = hosts.filter((h) => h.online && h.latest);
  const sum = (f: (h: Host) => number) => live.reduce((s, h) => s + f(h), 0);
  const avgCpu = live.length ? sum((h) => h.latest!.cpu) / live.length : 0;
  const memUsed = sum((h) => h.latest!.mem_used), memTotal = sum((h) => h.latest!.mem_total);
  const diskUsed = sum((h) => h.latest!.disk_used), diskTotal = sum((h) => h.latest!.disk_total);
  const offline = hosts.length - hosts.filter((h) => h.online).length;

  const refs = hosts.map((h) => ({ id: h.id, name: h.name, color: colors.get(h.id) as string }));
  const map = data?.map ?? new Map<string, Point[]>();
  const to = data?.to ?? now;
  const win = { from: to - sec, to, loading };
  const one = hosts.length === 1;
  const build = (pick: (p: Point) => number) => mergeHosts(refs, map, pick);

  return (
    <HoverProvider>
      <div className="tiles">
        <Tile label="노드" value={<>{live.length}<span className="unit"> / {hosts.length}</span></>} sub={offline ? `오프라인 ${offline}대` : "모두 실시간 수집 중"} />
        <Tile label="CPU 평균" value={fmtPct(avgCpu)} bar={avgCpu} sub={`실시간 ${live.length}대 기준`} />
        <Tile label="메모리" value={fmtPct(pct(memUsed, memTotal))} bar={pct(memUsed, memTotal)} sub={`${fmtBytes(memUsed)} / ${fmtBytes(memTotal)}`} />
        <Tile label="디스크" value={fmtPct(pct(diskUsed, diskTotal))} bar={pct(diskUsed, diskTotal)} sub={`${fmtBytes(diskUsed)} / ${fmtBytes(diskTotal)}`} />
        <Tile label="네트워크" value={<span className="net-val">↓ {fmtRate(sum((h) => h.latest!.net_rx))}</span>} sub={`↑ ${fmtRate(sum((h) => h.latest!.net_tx))}`} />
      </div>
      <Section title="시스템 개요" hint={one ? "노드 1대" : `노드 ${hosts.length}대 비교`}>
        <div className="grid charts">
          <LineChart title="CPU 사용률" {...build((p) => p.cpu)} fmt={fmtPct} max={100} area={one} {...win} />
          <LineChart title="메모리 사용률" {...build((p) => pct(p.mem_used, p.mem_total))} fmt={fmtPct} max={100} area={one} {...win} />
          <LineChart title="디스크 사용률" {...build((p) => pct(p.disk_used, p.disk_total))} fmt={fmtPct} max={100} area={one} {...win} />
          <LineChart title="네트워크 수신" {...build((p) => p.net_rx)} fmt={fmtRate} area={one} {...win} />
          <LineChart title="네트워크 송신" {...build((p) => p.net_tx)} fmt={fmtRate} area={one} {...win} />
          <LineChart title="시스템 부하 (1분)" {...build((p) => p.load1)} fmt={(v) => v.toFixed(2)} area={one} {...win} />
        </div>
      </Section>
    </HoverProvider>
  );
}
