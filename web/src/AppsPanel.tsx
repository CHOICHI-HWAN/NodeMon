// SPDX-License-Identifier: GPL-3.0-or-later
import type { ComponentProps } from "react";
import type { App, AppsReport, Host } from "./api";
import { SPIKE_FACTOR, SPIKE_MIN_CPU } from "./apps";
import { fmtBytes, fmtDateTime, fmtPct } from "./format";
import { LineChart } from "./LineChart";
import { Section } from "./ui";

const mem = (mib: number) => fmtBytes(mib * 1048576);

// "12분", "3.5시간": time a program kept one CPU core busy, as an estimate.
function fmtCoreTime(sec: number): string {
  if (sec < 60) return `${Math.round(sec)}초`;
  if (sec < 3600) return `${Math.round(sec / 60)}분`;
  return `${(sec / 3600).toFixed(sec < 36000 ? 1 : 0)}시간`;
}

type Props = {
  host: Host;
  report: AppsReport | null;
  xs: number[];
  win: Pick<ComponentProps<typeof LineChart>, "from" | "to" | "loading">;
};

export function AppsPanel({ host, report, xs, win }: Props) {
  const now = new Map<string, App>((host.apps ?? []).map((a) => [a.name, a]));
  const drawn = report?.apps.filter((a) => a.series.length) ?? [];
  const spikes = report?.apps.filter((a) => a.spike) ?? [];

  return (
    <Section title="프로그램별 사용량" hint="CPU·메모리를 많이 쓴 프로그램">
      {!report && now.size === 0 ? (
        <p className="sub apps-empty">
          이 컴퓨터는 프로그램별 사용량을 보내지 않습니다. 이 값은 Netdata의 apps 플러그인이 모으며 <strong>Linux</strong>에서만 제공됩니다(macOS·Windows는 지원하지 않습니다).
          Linux인데도 비어 있다면 Connector를 새 버전으로 바꿔 다시 실행하세요.
        </p>
      ) : (
        <>
          {spikes.length > 0 && (
            <ul className="spikes" aria-label="사용량이 갑자기 오른 프로그램">
              {spikes.map((a) => (
                <li key={a.name}>
                  <strong>{a.name}</strong>: {fmtDateTime(a.peakAt)}경 CPU가 <strong>{fmtPct(a.cpuPeak)}</strong>까지 올랐습니다 (평소 평균 {fmtPct(a.cpuAvg)})
                </li>
              ))}
            </ul>
          )}
          {drawn.length > 0 && (
            <div className="grid charts">
              <LineChart
                title="프로그램별 CPU (코어 1개 = 100%)"
                {...win}
                xs={xs}
                fmt={fmtPct}
                area={false}
                series={drawn.map((a, i) => ({ key: a.name, label: a.name, color: `var(--series-${(i % 8) + 1})`, values: a.series }))}
              />
            </div>
          )}
          {report && (
            <div className="table-card">
              <table className="nodes apps">
                <thead>
                  <tr>
                    <th>프로그램</th>
                    <th>지금 CPU</th>
                    <th>평균 CPU</th>
                    <th>최고 CPU (시각)</th>
                    <th>평균 메모리</th>
                    <th>최고 메모리</th>
                    <th title="평균 CPU × 기간. 코어 1개를 풀로 쓴 시간으로 환산한 추정치입니다.">CPU 사용 시간 ⓘ</th>
                  </tr>
                </thead>
                <tbody>
                  {report.apps.map((a) => (
                    <tr key={a.name}>
                      <td>
                        <strong>{a.name}</strong>
                        {a.spike && <span className="badge-spike" title={`최고치가 평균의 ${SPIKE_FACTOR}배 이상이고 ${SPIKE_MIN_CPU}% 이상`}>급증</span>}
                      </td>
                      <td>{now.has(a.name) ? fmtPct(now.get(a.name)!.cpu) : "–"}</td>
                      <td>{fmtPct(a.cpuAvg)}</td>
                      <td>{fmtPct(a.cpuPeak)} <span className="muted">({fmtDateTime(a.peakAt)})</span></td>
                      <td>{mem(a.memAvg)}</td>
                      <td>{mem(a.memPeak)}</td>
                      <td>{fmtCoreTime((a.cpuAvg / 100) * report.spanSec)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="table-note">
            CPU는 코어 1개를 100%로 센 값입니다(2개를 꽉 쓰면 200%). Connector가 3분마다 보내는 <strong>CPU·메모리 상위 프로그램만</strong> 집계하므로, 상위에 들지 못한 때는 0으로 계산되어 평균이 실제보다 작게 보일 수 있습니다. 3분보다 짧은 순간의 급증은 평균에 묻힙니다.
          </p>
        </>
      )}
    </Section>
  );
}
