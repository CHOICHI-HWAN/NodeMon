import { useCallback, useEffect, useMemo, useState } from "react";
import { AddHost } from "./AddHost";
import { api, Unauthorized, type RangeKey, type User } from "./api";
import { hostColors } from "./format";
import { HostDetail } from "./HostDetail";
import { useHash, usePoll } from "./hooks";
import { Login } from "./Login";
import { Overview } from "./Overview";

export default function App() {
  const [user, setUser] = useState<User | null | undefined>(undefined);
  useEffect(() => {
    api.me().then(setUser).catch((e) => e instanceof Unauthorized || setUser(null));
    const f = () => setUser(null);
    window.addEventListener("nm-unauthorized", f);
    return () => window.removeEventListener("nm-unauthorized", f);
  }, []);

  if (user === undefined) return null;
  if (user === null) return <Login />;
  return <Dashboard user={user} />;
}

function Dashboard({ user }: { user: User }) {
  const hash = useHash();
  const [range, setRange] = useState<RangeKey>("1h");
  const [adding, setAdding] = useState(false);
  const { data, error } = usePoll(() => api.overview(), [], 10000);
  const hosts = data?.hosts ?? [];
  const now = data?.now ?? Math.floor(Date.now() / 1000);
  const colors = useMemo(() => hostColors(hosts.map((h) => h.id)), [hosts]);
  const closeAdd = useCallback(() => setAdding(false), []);

  const m = hash.match(/^#\/host\/(\d+)/);
  const hostId = m ? Number(m[1]) : null;
  const host = hosts.find((h) => h.id === hostId);

  const logout = async () => {
    await api.logout();
    window.location.href = "/";
  };

  return (
    <div className="app">
      <header className="top">
        <a className="brand" href="#/">NodeMon</a>
        <div className="top-right">
          <span className="sub user" title={user.email}>{user.email}</span>
          <button className="primary" onClick={() => setAdding(true)}>컴퓨터 추가</button>
          <button className="link" onClick={logout}>로그아웃</button>
        </div>
      </header>
      <main>
        {error && !data && <p className="err">데이터를 불러오지 못했습니다: {error}</p>}
        {data &&
          (hostId == null ? (
            <Overview hosts={hosts} now={now} colors={colors} range={range} onRange={setRange} onAdd={() => setAdding(true)} />
          ) : (
            <HostDetail
              host={host}
              color={colors.get(hostId) ?? "var(--series-1)"}
              now={now}
              range={range}
              onRange={setRange}
              onDeleted={() => (window.location.hash = "#/")}
            />
          ))}
      </main>
      {adding && <AddHost onClose={closeAdd} />}
    </div>
  );
}
