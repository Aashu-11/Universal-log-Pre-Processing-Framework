import { useEffect, useRef, useState } from "react";

import { Card } from "../components/ui/Card";
import { Badge, statusTone } from "../components/ui/Badge";
import { ErrorState } from "../components/ui/States";
import { FullRecordPanel } from "../components/FullRecordPanel";
import { usePolling } from "../lib/usePolling";
import { fetchRecentEvents, fetchTrace, type RecentEvent, type TraceResult } from "../lib/liveEvents";

const POLL_MS = 4000;
const STAGE_STEP_MS = 320;
const TICKER_LENGTH = 10;

// The eight stages CLAUDE.md names for the pipeline, in order — this page's
// whole job is making every one of them individually provable for whichever
// event is currently "on stage," not just an aggregate health chip.
type StageKey = "ingest" | "preserve" | "identify" | "parse" | "normalize" | "enrich" | "validate" | "route";
const STAGES: { key: StageKey; label: string }[] = [
  { key: "ingest", label: "INGEST" },
  { key: "preserve", label: "PRESERVE" },
  { key: "identify", label: "IDENTIFY" },
  { key: "parse", label: "PARSE" },
  { key: "normalize", label: "NORMALIZE" },
  { key: "enrich", label: "ENRICH" },
  { key: "validate", label: "VALIDATE" },
  { key: "route", label: "ROUTE" },
];

export function LiveTheater() {
  const { data: recent, error: pollError } = usePolling<RecentEvent[]>(() => fetchRecentEvents(TICKER_LENGTH), POLL_MS);

  const [ticker, setTicker] = useState<RecentEvent[]>([]);
  const [current, setCurrent] = useState<RecentEvent | null>(null);
  const [trace, setTrace] = useState<TraceResult | null>(null);
  const [traceError, setTraceError] = useState<string | null>(null);
  const [litUpTo, setLitUpTo] = useState(-1); // -1 = nothing lit; 7 = fully settled
  const [arrivals, setArrivals] = useState(0);

  const seenIds = useRef<Set<string>>(new Set());
  const stepTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  // New data landed from the poll: fold in any event_ids we haven't shown
  // yet, oldest-first, so a burst of several new arrivals still plays one
  // at a time instead of jumping straight to the newest and hiding the
  // rest — the point of this page is watching them go by.
  const queue = useRef<RecentEvent[]>([]);
  useEffect(() => {
    if (!recent || recent.length === 0) return;
    const fresh = recent.filter((ev) => !seenIds.current.has(ev.event_id)).reverse();
    fresh.forEach((ev) => seenIds.current.add(ev.event_id));
    if (fresh.length > 0) {
      queue.current.push(...fresh);
      setArrivals((n) => n + fresh.length);
    }
    if (ticker.length === 0 && recent.length > 0) {
      // First load: seed the ticker with what already exists, newest first,
      // without animating each one individually.
      setTicker(recent);
      queue.current = queue.current.filter((ev) => !recent.some((r) => r.event_id === ev.event_id));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [recent]);

  // Drains the queue one event at a time, each playing the full
  // stage-lighting sequence before the next one starts.
  useEffect(() => {
    const advance = () => {
      if (current !== null) return; // still playing one
      const next = queue.current.shift();
      if (!next) return;
      setCurrent(next);
      setTrace(null);
      setTraceError(null);
      setLitUpTo(-1);
      setTicker((t) => [next, ...t].slice(0, TICKER_LENGTH));
      fetchTrace(next)
        .then(setTrace)
        .catch((err: unknown) => setTraceError(err instanceof Error ? err.message : "vault unreachable"));
    };
    const id = setInterval(advance, 400);
    return () => clearInterval(id);
  }, [current]);

  // Light the stages up one by one for whichever event is currently on
  // stage, then hold on the fully-lit state for a beat before releasing
  // `current` so the next queued event can start.
  useEffect(() => {
    if (!current) return;
    if (stepTimer.current) clearInterval(stepTimer.current);
    let step = -1;
    stepTimer.current = setInterval(() => {
      step += 1;
      setLitUpTo(step);
      if (step >= STAGES.length - 1) {
        clearInterval(stepTimer.current!);
        setTimeout(() => setCurrent(null), 1400);
      }
    }, STAGE_STEP_MS);
    return () => {
      if (stepTimer.current) clearInterval(stepTimer.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current?.event_id]);

  const stageContent: Record<StageKey, { rows: [string, string][]; lit: boolean }> = {
    ingest: { lit: litUpTo >= 0, rows: current ? [["vendor", current.observer_vendor], ["via", "TCP :6601"]] : [] },
    preserve: {
      lit: litUpTo >= 1,
      rows: trace ? [["sha-256", trace.raw.sha256_verified ? "verified" : "MISMATCH"], ["segment", current?.raw_segment_id.slice(-10) ?? ""]] : [["sha-256", "…"]],
    },
    identify: { lit: litUpTo >= 2, rows: current ? [["parser", current.lineage_parser_id]] : [] },
    parse: {
      lit: litUpTo >= 3,
      rows: current ? [["status", current.lineage_parse_status], ["took", `${current.lineage_processing_ms.toFixed(3)}ms`]] : [],
    },
    normalize: {
      lit: litUpTo >= 4,
      rows: current ? [["event.kind", current.event_kind], ["event.action", current.event_action || "—"]] : [],
    },
    enrich: {
      lit: litUpTo >= 5,
      rows: current ? [["risk", current.enrich_risk_score.toFixed(1)], ["geo", current.enrich_src_geo_country || "—"]] : [],
    },
    validate: { lit: litUpTo >= 6, rows: current ? [["quality", current.quality_score.toFixed(2)]] : [] },
    route: { lit: litUpTo >= 7, rows: current ? [["lake", `dt=${current.dt}`], ["stream", "kafka (30min)"]] : [] },
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-[16px] font-semibold text-[var(--color-text)]">Live Theater</h1>
          <p className="text-[11px] text-[var(--color-text-muted)]">
            Every real event, watched through all eight pipeline stages as it happens — nothing to click.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Badge tone="accent">{`${arrivals} events watched this session`}</Badge>
          <span className="flex items-center gap-1.5 text-[11px] text-[var(--color-text-muted)]">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--color-success)] opacity-75" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--color-success)]" />
            </span>
            live · polling every {POLL_MS / 1000}s
          </span>
        </div>
      </div>

      {pollError && <ErrorState message={`Presto unreachable: ${pollError}`} />}

      {ticker.length === 0 && !pollError && (
        <Card title="Waiting for the first event">
          <p className="text-[12px] text-[var(--color-text-secondary)]">
            No events in the lake yet. Send some real traffic through the collector and this page will pick it up
            within {POLL_MS / 1000}s:
          </p>
          <pre className="mt-3 overflow-x-auto rounded border border-[var(--color-border)] bg-black/40 p-3 font-mono-data text-[11.5px] text-[var(--color-accent)]">
            ./bin/loggen.exe --proto=tcp --host=127.0.0.1 --port=6601 --eps=500 --duration=30s --vendors=all
          </pre>
        </Card>
      )}

      {ticker.length > 0 && (
        <>
          <Card title={current ? `On stage: ${current.event_id.slice(0, 20)}…` : "Between events"}>
            {traceError && <ErrorState message={traceError} />}
            <div className="grid grid-cols-4 gap-3 lg:grid-cols-8">
              {STAGES.map((s, i) => {
                const content = stageContent[s.key];
                return (
                  <div key={s.key} className="flex items-center">
                    <div
                      className={`w-full rounded-md border p-2.5 transition-all duration-300 ${
                        content.lit
                          ? "border-[var(--color-accent)] bg-[var(--color-accent)]/10 shadow-[0_0_16px_-4px_var(--color-accent)]"
                          : "border-[var(--color-border-strong)] bg-white/[0.02] opacity-50"
                      }`}
                    >
                      <div
                        className={`mb-1.5 text-[9.5px] font-semibold uppercase tracking-wide ${
                          content.lit ? "text-[var(--color-accent)]" : "text-[var(--color-text-muted)]"
                        }`}
                      >
                        {i + 1}. {s.label}
                      </div>
                      {content.rows.map(([k, v]) => (
                        <div key={k} className="truncate text-[10px]" title={v}>
                          <span className="text-[var(--color-text-muted)]">{k}: </span>
                          <span className="font-mono-data text-[var(--color-text)]">{v}</span>
                        </div>
                      ))}
                    </div>
                    {i < STAGES.length - 1 && (
                      <span
                        className={`mx-1 hidden shrink-0 lg:block ${
                          litUpTo > i ? "text-[var(--color-accent)]" : "text-[var(--color-text-muted)]"
                        }`}
                      >
                        →
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          </Card>

          <Card title={`Arrivals — last ${ticker.length}`}>
            <div className="space-y-1">
              {ticker.map((ev, i) => (
                <div
                  key={ev.event_id + i}
                  className={`flex items-center justify-between rounded px-2.5 py-1.5 text-[11.5px] ${
                    i === 0 && current?.event_id === ev.event_id ? "bg-[var(--color-accent)]/10" : ""
                  }`}
                >
                  <span className="font-mono-data text-[var(--color-text-secondary)]">{ev.event_id.slice(0, 24)}…</span>
                  <span className="font-mono-data text-[var(--color-text-muted)]">{ev.observer_vendor}</span>
                  <Badge tone={statusTone(ev.event_action || "unknown")}>{ev.event_action || "—"}</Badge>
                  <Badge tone={statusTone(ev.lineage_parse_status)}>{ev.lineage_parse_status}</Badge>
                  <span className="font-mono-data text-[var(--color-text-muted)]">risk {ev.enrich_risk_score.toFixed(1)}</span>
                </div>
              ))}
            </div>
          </Card>

          {ticker[0] && <FullRecordPanel eventId={ticker[0].event_id} />}
        </>
      )}
    </div>
  );
}
