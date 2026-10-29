import { useCallback, useEffect, useMemo, useState } from "react";
import { AddHost } from "./AddHost";
import { api, OVERVIEW_POLL_MS, Unauthorized, type RangeKey, type User } from "./api";
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
  const load = useCallback(() => api.me().then(setUser).catch((e) => e instanceof Unauthorized || setUser(null)), []);
  useEffect(() => {
    load();
    const f = () => setUser(null);
    window.addEventListener("nm-unauthorized", f);
    return () => window.removeEventListener("nm-unauthorized", f);
  }, [load]);

  if (user === undefined) return null;
  if (user === null) return <Login onLogin={load} />;
  return <Dashboard user={user} />;
}

function Dashboard({ user }: { user: User }) {
  const hash = useHash();
  const [range, setRange] = useState<RangeKey>("1h");
  const [paused, setPaused] = useState(false);
  const [adding, setAdding] = useState(false);
  const [mode, setMode] = useState<DemoMode>(demoState.mode);
  const { data, error } = usePoll(() => api.overview(), [mode], paused ? 0 : OVERVIEW_POLL_MS);
  const hosts = data?.hosts ?? [];
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
      <Sidebar route={page} email={user.email} onAdd={() => setAdding(true)} onLogout={logout} />
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
              <Overview hosts={hosts} now={now} colors={colors} range={range} paused={paused} onAdd={() => setAdding(true)} />
            ) : page === "nodes" ? (
              <Nodes hosts={hosts} now={now} colors={colors} />
            ) : (
              <HostDetail host={host} color={colors.get(host?.id ?? "") ?? "var(--series-1)"} now={now} range={range} paused={paused} onDeleted={() => navigate("#/nodes")} />
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
