import { useEffect, useRef, useState } from "react";

import { ApiError, controlPlane } from "../api";
import {
  MAX_EVENT_BUFFER,
  computeEps,
  fetchDlqEvents,
  fetchLogVerseEvents,
  fetchSources,
  fetchVaultChain,
  EVENT_POLL_MS,
  MAX_VAULT_BLOCKS_VISIBLE,
} from "./data";
import type { ConnectionStatus, DerivedEps, LogVerseEvent, SourceNode, VaultBlock } from "./types";

const SLOW_POLL_MS = 8000; // sources / DLQ / vault chain change far less often than events

export interface LogVerseData {
  sources: SourceNode[];
  events: LogVerseEvent[]; // bounded ring buffer, ascending by ingestedAtMs
  dlqEvents: LogVerseEvent[];
  vaultChain: VaultBlock[];
  eps: DerivedEps | null;
  /** Real ulpf_udp_drops_total from the collector's own Prometheus
   * counter — actual packets dropped at ingest, not a rendering limit. */
  droppedEventsTotal: number | null;
  status: ConnectionStatus;
  errors: {
    events: string | null;
    sources: string | null;
    dlq: string | null;
    vault: string | null;
  };
}

function errMessage(err: unknown): string {
  return err instanceof ApiError ? err.message : err instanceof Error ? err.message : "unexpected error";
}

/** Polls every real data source LogVerse needs and merges them into one
 * bounded, deduplicated state object. Reuses the same REST/Presto endpoints
 * every other console page already calls — no new backend endpoint. The
 * event buffer is a true ring buffer (MAX_EVENT_BUFFER): once full, the
 * oldest event is dropped for each new arrival, so memory is bounded
 * regardless of how long the page stays open. */
export function useLogVerseData(): LogVerseData {
  const [sources, setSources] = useState<SourceNode[]>([]);
  const [events, setEvents] = useState<LogVerseEvent[]>([]);
  const [dlqEvents, setDlqEvents] = useState<LogVerseEvent[]>([]);
  const [vaultChain, setVaultChain] = useState<VaultBlock[]>([]);
  const [eps, setEps] = useState<DerivedEps | null>(null);
  const [droppedEventsTotal, setDroppedEventsTotal] = useState<number | null>(null);
  const [status, setStatus] = useState<ConnectionStatus>("connecting");
  const [errors, setErrors] = useState<LogVerseData["errors"]>({ events: null, sources: null, dlq: null, vault: null });

  const everLive = useRef(false);
  const prevEpsSample = useRef<{ atMs: number; total: number } | null>(null);
  const seenEventIds = useRef<Set<string>>(new Set());
  // Guards against overlapping in-flight requests: if the backend is slow
  // (a real one — see docs/LOGVERSE.md — took 15s+ for a plain query
  // against this dev lake), firing a new poll every EVENT_POLL_MS on top
  // of one still in flight piles up concurrent queries that starve each
  // other and can leave the page stuck on "Connecting..." indefinitely
  // even though the backend isn't actually down. Skipping a tick while
  // the previous one hasn't resolved fixes that regardless of how slow
  // any single query turns out to be.
  const eventsInFlight = useRef(false);
  const slowInFlight = useRef(false);
  const epsInFlight = useRef(false);

  useEffect(() => {
    let cancelled = false;

    const pollEvents = async () => {
      if (eventsInFlight.current) return;
      eventsInFlight.current = true;
      try {
        const fresh = await fetchLogVerseEvents(60);
        if (cancelled) return;
        setEvents((prev) => {
          const unseen = fresh.filter((e) => !seenEventIds.current.has(e.eventId));
          unseen.forEach((e) => seenEventIds.current.add(e.eventId));
          if (unseen.length === 0 && prev.length > 0) return prev;
          const merged = [...prev, ...unseen].sort((a, b) => a.ingestedAtMs - b.ingestedAtMs);
          const bounded = merged.slice(-MAX_EVENT_BUFFER);
          // Keep the id set in sync with what's actually still buffered,
          // otherwise it grows forever even though the buffer itself doesn't.
          if (bounded.length < merged.length) {
            seenEventIds.current = new Set(bounded.map((e) => e.eventId));
          }
          return bounded;
        });
        setErrors((e) => ({ ...e, events: null }));
        everLive.current = true;
        setStatus("live");
      } catch (err) {
        if (cancelled) return;
        setErrors((e) => ({ ...e, events: errMessage(err) }));
        setStatus(everLive.current ? "reconnecting" : "error");
      } finally {
        eventsInFlight.current = false;
      }
    };

    const pollSlow = async () => {
      if (slowInFlight.current) return;
      slowInFlight.current = true;
      const [sourcesResult, dlqResult, vaultResult] = await Promise.allSettled([
        fetchSources(),
        fetchDlqEvents(),
        fetchVaultChain(MAX_VAULT_BLOCKS_VISIBLE),
      ]);
      slowInFlight.current = false;
      if (cancelled) return;
      if (sourcesResult.status === "fulfilled") {
        setSources(sourcesResult.value);
        setErrors((e) => ({ ...e, sources: null }));
      } else {
        setErrors((e) => ({ ...e, sources: errMessage(sourcesResult.reason) }));
      }
      if (dlqResult.status === "fulfilled") {
        setDlqEvents(dlqResult.value);
        setErrors((e) => ({ ...e, dlq: null }));
      } else {
        setErrors((e) => ({ ...e, dlq: errMessage(dlqResult.reason) }));
      }
      if (vaultResult.status === "fulfilled") {
        setVaultChain(vaultResult.value);
        setErrors((e) => ({ ...e, vault: null }));
      } else {
        setErrors((e) => ({ ...e, vault: errMessage(vaultResult.reason) }));
      }
    };

    const pollEps = async () => {
      if (epsInFlight.current) return;
      epsInFlight.current = true;
      try {
        const stats = await controlPlane.get<{ events_received_total: number; udp_drops_total: number }>(
          "/v1/stats/pipeline",
        );
        if (cancelled) return;
        setDroppedEventsTotal(stats.udp_drops_total);
        const sample = { atMs: Date.now(), total: stats.events_received_total };
        const derived = computeEps(prevEpsSample.current, sample);
        prevEpsSample.current = sample;
        if (derived) setEps(derived);
      } catch {
        // EPS is a supplementary stat; a failure here shouldn't flip overall
        // connection status away from what the events poll (the primary
        // signal) reports.
      } finally {
        epsInFlight.current = false;
      }
    };

    void pollEvents();
    void pollSlow();
    void pollEps();
    const eventsId = setInterval(pollEvents, EVENT_POLL_MS);
    const slowId = setInterval(pollSlow, SLOW_POLL_MS);
    const epsId = setInterval(pollEps, EVENT_POLL_MS);

    return () => {
      cancelled = true;
      clearInterval(eventsId);
      clearInterval(slowId);
      clearInterval(epsId);
    };
  }, []);

  return { sources, events, dlqEvents, vaultChain, eps, droppedEventsTotal, status, errors };
}
