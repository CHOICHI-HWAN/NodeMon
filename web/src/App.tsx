// SPDX-License-Identifier: GPL-3.0-or-later
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AddHost } from "./AddHost";
import { api, OVERVIEW_POLL_MS, Unauthorized, type Host, type RangeKey, type User } from "./api";
import { demoState, type DemoMode } from "./demo";
import { DEMO } from "./env";
import { hostColors } from "./format";
import { HostDetail } from "./HostDetail";
import { navigate, useHash, usePoll } from "./hooks";
import { Login } from "./Login";
import { Nodes } from "./Nodes";
import { Overview } from "./Overview";
import { Sidebar } from "./Sidebar";
import { RangeMenu } from "./ui";

export default function App() {
  const [user, setUser] = useState<User | null | undefined>(undefined);
  const [startupError, setStartupError] = useState<string | null>(null);
  const load = useCallback(
    () =>
      api.me().then(setUser).catch((e) => {
        setUser(null);
        if (!(e instanceof Unauthorized)) setStartupError(e instanceof Error ? e.message : String(e)); // a broken setup, not just "not signed in"
      }),
    [],
  );
  useEffect(() => {
    load();
    const f = () => setUser(null);
    window.addEventListener("nm-unauthorized", f);
    return () => window.removeEventListener("nm-unauthorized", f);
  }, [load]);

  if (user === undefined) return null;
  if (user === null) return <Login onLogin={load} startupError={startupError} />;
  return <Dashboard user={user} />;
}

function Dashboard({ user }: { user: User }) {
  const hash = useHash();
  const [range, setRange] = useState<RangeKey>("1h");
  const [paused, setPaused] = useState(false);
  const [adding, setAdding] = useState(false);
  const [watching, setWatching] = useState(false); // just opened "add computer": expect a new node soon
  const watchTimer = useRef<number>();
  const [mode, setMode] = useState<DemoMode>(demoState.mode);
  const [hostCount, setHostCount] = useState(-1);
  // Normal polling is slow to save data; right after adding a computer (or with none yet) check often.
  const fast = watching || hostCount === 0;
  const { data, error } = usePoll(() => api.overview(), [mode], paused ? 0 : fast ? 5000 : OVERVIEW_POLL_MS);
  const [gone, setGone] = useState<ReadonlySet<string>>(new Set()); // deleted just now; the next refresh no longer lists them
  const [renamed, setRenamed] = useState<Readonly<Record<string, string>>>({}); // nicknames set just now, shown before the next refresh
  const hosts = useMemo(
    () => (data?.hosts ?? []).filter((h) => !gone.has(h.id)).map((h) => (h.id in renamed ? { ...h, alias: renamed[h.id] || null, name: renamed[h.id] || h.hostname || h.name } : h)),
    [data, gone, renamed],
  );
  const rename = useCallback(async (h: Host) => {
    const next = window.prompt(`'${h.hostname ?? h.name}' 의 표시 이름 (비우면 원래 이름으로 돌아갑니다)`, h.alias ?? "");
    if (next == null) return;
    try {
      await api.renameHost(h.id, next);
      setRenamed((r) => ({ ...r, [h.id]: next.trim().slice(0, 60) }));
    } catch (e) {
      window.alert(`이름을 바꾸지 못했습니다: ${e instanceof Error ? e.message : String(e)}`);
    }
  }, []);
  const forget = useCallback((id: string) => setGone((g) => new Set(g).add(id)), []);
  useEffect(() => setHostCount(data ? hosts.length : -1), [data, hosts]);
  const openAdd = () => {
    setAdding(true);
    setWatching(true);
    window.clearTimeout(watchTimer.current);
    watchTimer.current = window.setTimeout(() => setWatching(false), 3 * 60_000);
  };
  const now = data?.now ?? Math.floor(Date.now() / 1000);
  const colors = useMemo(() => hostColors(hosts), [hosts]);
  const closeAdd = useCallback(() => setAdding(false), []);

  const m = hash.match(/^#\/host\/([^/]+)/);
  const hostId = m ? decodeURIComponent(m[1]) : null;
  const host = hosts.find((h) => h.id === hostId);
  const page = hostId != null ? "host" : hash.startsWith("#/nodes") ? "nodes" : "overview";

  // The demo runs inside a frame that only allows plain #name hashes, so route in memory.
  useEffect(() => {
    if (!DEMO) return;
    const f = (e: MouseEvent) => {
      const href = (e.target as HTMLElement).closest("a")?.getAttribute("href");
      if (href?.startsWith("#/")) {
        e.preventDefault();
        navigate(href);
      }
    };
    document.addEventListener("click", f);
    return () => document.removeEventListener("click", f);
  }, []);

  const switchMode = (next: DemoMode) => {
    demoState.mode = next;
    navigate("#/");
    setMode(next);
  };
  const logout = async () => {
    await api.logout();
    window.location.reload();
  };

  const title =
    page === "overview" ? (
      <h1>개요 <span className="crumb">모든 노드</span></h1>
    ) : page === "nodes" ? (
      <h1>노드</h1>
    ) : (
      <h1><a href="#/nodes" className="crumb-link">노드</a><span className="sep">/</span>{host?.name ?? "알 수 없음"}</h1>
    );

  return (
    <div className="shell">
      <Sidebar route={page} email={user.email} onAdd={openAdd} onLogout={logout} />
      <div className="main-col">
        {DEMO && <DemoBar mode={mode} onMode={switchMode} />}
        <header className="topbar">
          {title}
          <div className="grow" />
          {page !== "nodes" && (
            <>
              <button className={`btn live${paused ? " paused" : ""}`} onClick={() => setPaused((p) => !p)} aria-pressed={paused} title={paused ? "실시간 갱신 재개" : "실시간 갱신 일시정지"}>
                <span className="dot" aria-hidden="true" />
                {paused ? "일시정지됨" : "실시간"}
              </button>
              <RangeMenu value={range} onChange={setRange} />
            </>
          )}
        </header>
        <main className="content">
          {error && !data && <p className="err">데이터를 불러오지 못했습니다: {error}</p>}
          {data &&
            (page === "overview" ? (
              <Overview hosts={hosts} now={now} colors={colors} range={range} paused={paused} onAdd={openAdd} />
            ) : page === "nodes" ? (
              <Nodes hosts={hosts} now={now} colors={colors} onDeleted={forget} onRename={rename} />
            ) : (
              <HostDetail host={host} color={colors.get(host?.id ?? "") ?? "var(--series-1)"} now={now} range={range} paused={paused} onDeleted={() => { forget(host!.id); navigate("#/nodes"); }} onRename={() => rename(host!)} />
            ))}
        </main>
      </div>
      {adding && <AddHost onClose={closeAdd} />}
    </div>
  );
}

function DemoBar({ mode, onMode }: { mode: DemoMode; onMode: (m: DemoMode) => void }) {
  return (
    <div className="demo-bar">
      <span>데모 화면 · 가상 데이터입니다</span>
      <div className="seg" role="group" aria-label="구성 선택">
        <button className={mode === "single" ? "active" : ""} aria-pressed={mode === "single"} onClick={() => onMode("single")}>
          싱글 구성 · PC 1대
        </button>
        <button className={mode === "multi" ? "active" : ""} aria-pressed={mode === "multi"} onClick={() => onMode("multi")}>
          멀티 구성 · PC 5대
        </button>
      </div>
    </div>
  );
}
