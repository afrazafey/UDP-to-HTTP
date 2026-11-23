import { useCallback, useEffect, useRef, useState } from "react";
export function usePolling<T>(
  loader: (signal: AbortSignal) => Promise<T>,
  key: string,
  interval = 5,
  paused = false,
) {
  const loadRef = useRef(loader);
  loadRef.current = loader;
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [updated, setUpdated] = useState<Date | null>(null);
  const [generation, setGeneration] = useState(0);
  const refresh = useCallback(() => setGeneration((value) => value + 1), []);
  const previousGeneration = useRef(generation);
  useEffect(() => {
    let live = true;
    let busy = false;
    let current: AbortController | null = null;
    const load = async () => {
      if (busy) return;
      busy = true;
      current = new AbortController();
      const controller = current;
      setLoading(true);
      const timeout = setTimeout(() => controller.abort(), 10000);
      try {
        const result = await loadRef.current(controller.signal);
        if (live) {
          setData(result);
          setError("");
          setUpdated(new Date());
        }
      } catch (failure) {
        if (live)
          setError(
            failure instanceof Error
              ? failure.message
              : "Service request failed",
          );
      } finally {
        clearTimeout(timeout);
        busy = false;
        if (live) setLoading(false);
      }
    };
    const manual = previousGeneration.current !== generation;
    previousGeneration.current = generation;
    if (!paused || manual) void load();
    else setLoading(false);
    const timer = paused
      ? null
      : setInterval(() => {
          void load();
        }, interval * 1000);
    return () => {
      live = false;
      current?.abort();
      if (timer) clearInterval(timer);
    };
  }, [key, interval, paused, generation]);
  return { data, error, loading, updated, refresh, stale: !!error && !!data };
}
export function useRoute() {
  const [route, setRoute] = useState(
    () => window.location.hash.slice(1) || "/overview",
  );
  useEffect(() => {
    const change = () => setRoute(window.location.hash.slice(1) || "/overview");
    window.addEventListener("hashchange", change);
    return () => window.removeEventListener("hashchange", change);
  }, []);
  return route;
}
