import { useEffect, useRef, useState } from "react";

import { ApiError } from "./api";

interface PollState<T> {
  data: T | null;
  error: string | null;
  lastUpdated: number | null;
}

/** Re-fetches fn every intervalMs. Used by the Pipeline page's live counters
 * and anywhere else that needs "auto-refresh every N seconds" rather than
 * fetch-once — see lib/useApi.ts for the one-shot version. */
export function usePolling<T>(fn: () => Promise<T>, intervalMs: number): PollState<T> {
  const [state, setState] = useState<PollState<T>>({ data: null, error: null, lastUpdated: null });
  const fnRef = useRef(fn);
  fnRef.current = fn;

  useEffect(() => {
    let cancelled = false;

    const tick = () => {
      fnRef
        .current()
        .then((data) => {
          if (!cancelled) setState({ data, error: null, lastUpdated: Date.now() });
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setState((prev) => ({
              ...prev,
              error: err instanceof ApiError ? err.message : "unexpected error",
            }));
          }
        });
    };

    tick();
    const id = setInterval(tick, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs]);

  return state;
}
