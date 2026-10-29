import { useEffect, useRef, useState, type ReactNode } from "react";
import { RANGES, type RangeKey } from "./api";
import { fmtPct } from "./format";

export function Status({ online }: { online: boolean }) {
  return (
    <span className={`status ${online ? "on" : "off"}`}>
      <span className="dot" aria-hidden="true" />
      {online ? "실시간" : "오프라인"}
    </span>
  );
}

export function Bar({ value, label }: { value: number; label: string }) {
  return (
    <div className="track" role="meter" aria-label={label} aria-valuenow={Math.round(value)} aria-valuemin={0} aria-valuemax={100}>
      <div className="fill" style={{ width: `${Math.min(100, Math.max(0, value))}%` }} />
    </div>
  );
}

export function Tile({ label, value, sub, bar }: { label: string; value: ReactNode; sub?: ReactNode; bar?: number }) {
  return (
    <div className="tile">
      <div className="tile-label">{label}</div>
      <div className="tile-value">{value}</div>
      {bar != null && <Bar value={bar} label={label} />}
      {sub && <div className="tile-sub">{sub}</div>}
    </div>
  );
}

export function Meter({ label, value, detail }: { label: string; value: number; detail: string }) {
  return (
    <div className="meter">
      <div className="meter-row">
        <span>{label}</span>
        <strong>{fmtPct(value)}</strong>
      </div>
      <Bar value={value} label={label} />
      <div className="meter-detail">{detail}</div>
    </div>
  );
}

// Section heading that collapses its charts, like the group headers in Netdata's dashboards.
export function Section({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  const [open, setOpen] = useState(true);
  return (
    <section className="section">
      <button className="section-head" onClick={() => setOpen((o) => !o)} aria-expanded={open}>
        <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true" style={{ transform: open ? "rotate(90deg)" : "none" }}>
          <path d="M4 2l4 4-4 4" fill="none" stroke="currentColor" strokeWidth="1.8" />
        </svg>
        <h2>{title}</h2>
        {hint && <span className="section-hint">{hint}</span>}
      </button>
      {open && children}
    </section>
  );
}

export function useOutsideClose(onClose: () => void) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const click = (e: MouseEvent) => ref.current && !ref.current.contains(e.target as Node) && onClose();
    const key = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("mousedown", click);
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("mousedown", click);
      document.removeEventListener("keydown", key);
    };
  }, [onClose]);
  return ref;
}

export function RangeMenu({ value, onChange }: { value: RangeKey; onChange: (r: RangeKey) => void }) {
  const [open, setOpen] = useState(false);
  const ref = useOutsideClose(() => setOpen(false));
  const cur = RANGES.find((r) => r.key === value)!;
  return (
    <div className="menu-wrap" ref={ref}>
      <button className="btn" onClick={() => setOpen((o) => !o)} aria-haspopup="listbox" aria-expanded={open}>
        <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
          <circle cx="7" cy="7" r="5.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M7 4v3.2l2 1.3" fill="none" stroke="currentColor" strokeWidth="1.5" />
        </svg>
        최근 {cur.label}
        <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
          <path d="M2 3.5l3 3 3-3" fill="none" stroke="currentColor" strokeWidth="1.6" />
        </svg>
      </button>
      {open && (
        <ul className="menu" role="listbox" aria-label="기간">
          {RANGES.map((r) => (
            <li key={r.key}>
              <button
                role="option"
                aria-selected={r.key === value}
                className={r.key === value ? "sel" : ""}
                onClick={() => {
                  onChange(r.key);
                  setOpen(false);
                }}
              >
                <span className="check">{r.key === value ? "✓" : ""}</span>최근 {r.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
