import { useEffect, useState } from "react";

import { fetchForensicTrace } from "../../lib/logverse/data";
import type { ForensicTrace, LogVerseEvent, PipelineStage } from "../../lib/logverse/types";
import { ForensicTracePlayback } from "./ForensicTracePlayback";
import { relatedEventCount } from "../../lib/logverse/analysis";
import { explainDlqReason } from "../../lib/logverse/failure";

const STATE_LABEL: Record<LogVerseEvent["visualState"], string> = {
  normal: "Normal",
  elevated: "Elevated risk",
  threat: "Threat / IOC match",
  dlq: "Dead-letter queue",
};

export function EventInspector({
  event,
  onClose,
  onTraceStart,
  onStageHighlight,
  recentEvents = [],
  autoTrace = false,
}: {
  event: LogVerseEvent;
  onClose: () => void;
  onTraceStart?: () => void;
  onStageHighlight?: (stage: PipelineStage | null) => void;
  recentEvents?: LogVerseEvent[];
  autoTrace?: boolean;
}) {
  const [tracing, setTracing] = useState(false);
  const [trace, setTrace] = useState<ForensicTrace | null>(null);
  const [traceLoading, setTraceLoading] = useState(false);
  const [traceError, setTraceError] = useState<string | null>(null);

  const averageRisk = recentEvents.length > 0 ? recentEvents.reduce((total, item) => total + item.riskScore, 0) / recentEvents.length : null;
  const related = relatedEventCount(event, recentEvents);

  useEffect(() => {
    if (!autoTrace) return;
    let active = true;
    setTracing(true);
    setTraceLoading(true);
    onTraceStart?.();
    void fetchForensicTrace(event).then((result) => {
      if (!active) return;
      setTrace(result);
      setTraceError(result.error);
    }).catch((error: unknown) => {
      if (active) setTraceError(error instanceof Error ? error.message : "trace failed");
    }).finally(() => {
      if (active) setTraceLoading(false);
    });
    return () => { active = false; };
    // The inspector is keyed by event ID; refetch only when the selected
    // event changes, not on each polling refresh of its view model.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [event.eventId, autoTrace]);

  const startTrace = () => {
    setTracing(true);
    onTraceStart?.();
    if (trace || traceLoading) return;
    setTraceLoading(true);
    setTraceError(null);
    fetchForensicTrace(event)
      .then((t) => {
        setTrace(t);
        if (t.error) setTraceError(t.error);
      })
      .catch((err: unknown) => setTraceError(err instanceof Error ? err.message : "trace failed"))
      .finally(() => setTraceLoading(false));
  };

  return (
    <div
      role="dialog"
      aria-label="Event inspector"
      className="pointer-events-auto flex h-full w-full flex-col overflow-y-auto rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)]/97 shadow-2xl backdrop-blur"
    >
      <div className="flex items-center justify-between border-b border-[var(--color-border)] px-3.5 py-2.5">
        <div className="min-w-0">
          <div className="truncate font-mono-data text-[12px] font-semibold text-[var(--color-text)]" title={event.eventId}>
            {event.eventId.slice(0, 28)}
            {event.eventId.length > 28 ? "…" : ""}
          </div>
          <div className="text-[10px] text-[var(--color-text-muted)]">{STATE_LABEL[event.visualState]}</div>
        </div>

        <button
          type="button"
          onClick={onClose}
          aria-label="Close inspector"
          className="rounded border border-[var(--color-border-strong)] px-2 py-1 text-[11px] text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
        >
          ✕
        </button>
      </div>

      <div className="space-y-3 p-3.5">
        <div className="rounded-md border border-[var(--color-border)] bg-white/[0.02] p-2.5">
          <div className="mb-2 font-mono-data text-[10px] font-semibold uppercase tracking-wide text-[var(--color-accent)]">{event.isDlq ? "Failure diagnosis" : "Risk in context"}</div>
          {event.isDlq ? explainDlqReason(event.dlqReason).map((finding) => <div key={finding.code} className="mb-2 border-l-2 border-[var(--color-warning)] pl-2 text-[10px] text-[var(--color-text-secondary)]"><div className="font-mono font-semibold text-[var(--color-warning)]">{finding.code} · {finding.stage}</div><p>{finding.explanation}</p><p className="mt-1">Next check: {finding.nextCheck}</p></div>) : <>
            <RiskBar label="Selected event" value={event.riskScore} color="var(--color-warning)" />
            {averageRisk !== null && <RiskBar label={`Loaded window average (${recentEvents.length})`} value={averageRisk} color="var(--color-accent)" />}
            <p className="mt-2 text-[10px] text-[var(--color-text-muted)]">{related} other loaded event{related === 1 ? "" : "s"} share an observed source or destination IP. This is a relationship count, not an attack verdict.</p>
          </>}
        </div>
        <div className="rounded-md border border-[var(--color-border)] bg-white/[0.02] p-2.5">
          <div className="mb-1 font-mono-data text-[10px] font-semibold uppercase tracking-wide text-[var(--color-accent)]">
            Summary
          </div>
          <SummaryRow label="Vendor" value={event.vendor} />
          <SummaryRow label="Action" value={event.eventAction || "—"} />
          <SummaryRow label="Src" value={event.srcIp ? `${event.srcIp}:${event.srcPort}` : "—"} />
          <SummaryRow label="Dst" value={event.dstIp ? `${event.dstIp}:${event.dstPort}${event.dstIsInternal ? " (internal)" : ""}` : "—"} />
          <SummaryRow label="Risk score" value={event.isDlq ? "Unavailable in DLQ record" : event.riskScore.toFixed(1)} />
          <SummaryRow label="Quality score" value={event.isDlq ? "Unavailable in DLQ record" : event.qualityScore.toFixed(2)} />
          {event.isDlq && <SummaryRow label="DLQ reason" value={event.dlqReason ?? "unspecified"} />}
          <SummaryRow label="Observed" value={new Date(event.ingestedAtMs).toLocaleString()} />
        </div>

        {!tracing && (
          <button
            type="button"
            onClick={startTrace}
            className="w-full rounded-lg border border-[#9c79ff] bg-[linear-gradient(115deg,#7950e8,#3978ee)] px-3 py-2 text-[12px] font-semibold text-white shadow-[0_8px_24px_rgba(86,63,200,0.22)] hover:brightness-110"
          >
            Trace Event — rewind to raw evidence
          </button>
        )}

        <ForensicTracePlayback
          event={event}
          trace={trace}
          traceLoading={traceLoading}
          traceError={traceError}
          playing={tracing}
          onStageHighlight={onStageHighlight}
        />
      </div>
    </div>
  );
}

function RiskBar({ label, value, color }: { label: string; value: number; color: string }) {
  const safeValue = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  return <div className="mb-2"><div className="mb-1 flex justify-between text-[10px] text-[var(--color-text-secondary)]"><span>{label}</span><span className="font-mono">{value.toFixed(1)}</span></div><div className="h-1.5 overflow-hidden rounded-full bg-white/5"><div className="h-full rounded-full" style={{ width: `${safeValue}%`, backgroundColor: color }} /></div></div>;
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 py-0.5 text-[11px]">
      <span className="text-[var(--color-text-muted)]">{label}</span>
      <span className="truncate font-mono-data text-[var(--color-text)]" title={value}>
        {value}
      </span>
    </div>
  );
}
