import { useState } from "react";
import { DEMO } from "./env";
import { getTheme, setTheme, type Theme } from "./theme";
import { useOutsideClose } from "./ui";

type Route = "overview" | "nodes";

const Icon = {
  overview: <path d="M3 3h7v7H3zM12 3h7v4h-7zM12 9h7v10h-7zM3 12h7v7H3z" />,
  nodes: <path d="M3 4h16v5H3zM3 13h16v5H3zM6 6.5h2M6 15.5h2" />,
  plus: <path d="M11 4v14M4 11h14" />,
};

function Svg({ children }: { children: React.ReactNode }) {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" aria-hidden="true">
      {children}
    </svg>
  );
}

type Props = { route: Route | "host"; email: string; onAdd: () => void; onLogout: () => void };

export function Sidebar({ route, email, onAdd, onLogout }: Props) {
  const [open, setOpen] = useState(false);
  const [theme, setT] = useState<Theme>(getTheme());
  const ref = useOutsideClose(() => setOpen(false));
  const pick = (t: Theme) => {
    setTheme(t);
    setT(t);
  };
  const link = (to: Route, label: string, active: boolean) => (
    <a href={to === "overview" ? "#/" : "#/nodes"} className={`nav-item${active ? " active" : ""}`} aria-current={active ? "page" : undefined} title={label}>
      <Svg>{Icon[to]}</Svg>
      <span>{label}</span>
    </a>
  );
  return (
    <nav className="sidebar" aria-label="주 메뉴">
      <a className="logo" href="#/" aria-label="NodeMon 홈">N</a>
      {link("overview", "개요", route === "overview")}
      {link("nodes", "노드", route === "nodes" || route === "host")}
      <div className="grow" />
      <button className="nav-item" onClick={onAdd} title="컴퓨터 추가">
        <Svg>{Icon.plus}</Svg>
        <span>추가</span>
      </button>
      <div className="menu-wrap" ref={ref}>
        <button className="avatar" onClick={() => setOpen((o) => !o)} aria-haspopup="menu" aria-expanded={open} title={email}>
          {(email[0] ?? "?").toUpperCase()}
        </button>
        {open && (
          <div className="menu account" role="menu">
            <div className="menu-title" title={email}>{email}</div>
            {!DEMO && (
              <div className="seg" role="group" aria-label="테마">
                {(["auto", "light", "dark"] as Theme[]).map((t) => (
                  <button key={t} className={theme === t ? "active" : ""} aria-pressed={theme === t} onClick={() => pick(t)}>
                    {{ auto: "자동", light: "라이트", dark: "다크" }[t]}
                  </button>
                ))}
              </div>
            )}
            {!DEMO && <button className="menu-item" role="menuitem" onClick={onLogout}>로그아웃</button>}
          </div>
        )}
      </div>
    </nav>
  );
}
