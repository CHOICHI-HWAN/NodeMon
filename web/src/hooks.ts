import { useEffect, useState } from "react";
import { DEMO } from "./env";

let demoHash = "#/";
const ROUTE_EVENT = "nm-route";

export function navigate(h: string) {
  if (DEMO) {
    demoHash = h;
    window.dispatchEvent(new Event(ROUTE_EVENT));
  } else {
    window.location.hash = h;
  }
}

export function usePoll<T>(fn: () => Promise<T>, deps: unknown[], ms = 10000) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    let stop = false;
    const run = async () => {
      try {
        const d = await fn();
        if (!stop) {
          setData(d);
          setError(null);
        }
      } catch (e) {
        if (!stop) setError(e instanceof Error ? e.message : String(e));
      } finally {
        if (!stop) setLoading(false);
      }
    };
    setLoading(true);
    run();
    const t = setInterval(run, ms);
    return () => {
      stop = true;
      clearInterval(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return { data, error, loading };
}

export function useHash(): string {
  const read = () => (DEMO ? demoHash : window.location.hash || "#/");
  const [h, setH] = useState(read());
  useEffect(() => {
    const f = () => setH(read());
    window.addEventListener(DEMO ? ROUTE_EVENT : "hashchange", f);
    return () => window.removeEventListener(DEMO ? ROUTE_EVENT : "hashchange", f);
  }, []);
  return h;
}
