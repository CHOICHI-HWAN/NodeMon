import { useCallback, useEffect, useMemo, useState } from "react";
import { AddHost } from "./AddHost";
import { api, Unauthorized, type RangeKey, type User } from "./api";
import { demoState, type DemoMode } from "./demo";
import { DEMO } from "./env";
import { hostColors } from "./format";
import { HostDetail } from "./HostDetail";
import { navigate, useHash, usePoll } from "./hooks";
import { Login } from "./Login";
import { Overview } from "./Overview";

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
  const [adding, setAdding] = useState(false);
  const [mode, setMode] = useState<DemoMode>(demoState.mode);
  const { data, error } = usePoll(() => api.overview(), [mode], 10000);
  const hosts = data?.hosts ?? [];
  const now = data?.now ?? Math.floor(Date.now() / 1000);
  const colors = useMemo(() => hostColors(hosts), [hosts]);
  const closeAdd = useCallback(() => setAdding(false), []);

  const m = hash.match(/^#\/host\/([^/]+)/);
  const hostId = m ? decodeURIComponent(m[1]) : null;
  // With a single computer the list view adds nothing, so open its detail directly.
  const host = hosts.find((h) => h.id === hostId) ?? (hosts.length === 1 ? hosts[0] : undefined);

  // The demo runs inside a frame that only allows plain #name hashes, so route in memory.
  useEffect(() => {
    if (!DEMO) return;
    const f = (e: MouseEvent) => {
      const a = (e.target as HTMLElement).closest("a");
      const href = a?.getAttribute("href");
      if (href?.startsWith("#/")) {
        e.preventDefault();
        navigate(href);
      }
    };
    document.addEventListener("click", f);
    return () => document.removeEventListener("click", f);
  }, []);

  const switchMode = (m: DemoMode) => {
    demoState.mode = m;
    navigate("#/");
    setMode(m);
  };

  const logout = async () => {
    await api.logout();
    window.location.reload();
  };

  return (
    <div className="app">
      {DEMO && <DemoBar mode={mode} onMode={switchMode} />}
      <header className="top">
        <a className="brand" href="#/">NodeMon</a>
        <div className="top-right">
          <span className="sub user" title={user.email}>{user.email}</span>
          <button className="primary" onClick={() => setAdding(true)}>컴퓨터 추가</button>
          {!DEMO && <button className="link" onClick={logout}>로그아웃</button>}
        </div>
      </header>
      <main>
        {error && !data && <p className="err">데이터를 불러오지 못했습니다: {error}</p>}
        {data &&
          (hostId == null && hosts.length !== 1 ? (
            <Overview hosts={hosts} now={now} colors={colors} range={range} onRange={setRange} onAdd={() => setAdding(true)} />
          ) : (
            <HostDetail
              host={host}
              showBack={hosts.length > 1}
              color={colors.get(host?.id ?? "") ?? "var(--series-1)"}
              now={now}
              range={range}
              onRange={setRange}
              onDeleted={() => navigate("#/")}
            />
          ))}
      </main>
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
