import { createContext, useContext, useMemo, useState, type ReactNode } from "react";

// Charts on one page share a hovered timestamp, so the crosshair and legend values line up across them.
const Ctx = createContext<{ t: number | null; set: (t: number | null) => void }>({ t: null, set: () => {} });

export function HoverProvider({ children }: { children: ReactNode }) {
  const [t, set] = useState<number | null>(null);
  const value = useMemo(() => ({ t, set }), [t]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export const useHover = () => useContext(Ctx);
