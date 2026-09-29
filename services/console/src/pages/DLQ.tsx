import { useState } from "react";

import { Card } from "../components/ui/Card";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { LoadingState, ErrorState, EmptyState } from "../components/ui/States";
import { controlPlane } from "../lib/api";
import { useApi } from "../lib/useApi";
import { canWrite, useAuth } from "../lib/auth";
import type { DLQEventOut } from "../lib/types";

type StepStatus = "queued" | "in-flight" | "resolved" | "failed";
interface ReplayStep {
  eventId: string;
  status: StepStatus;
  detail: string;
}

const STEP_ICON: Record<StepStatus, string> = { queued: "○", "in-flight": "◐", resolved: "✓", failed: "✕" };
const STEP_CLASS: Record<StepStatus, string> = {
  queued: "text-[var(--color-text-muted)]",
  "in-flight": "animate-pulse text-[var(--color-accent)]",
  resolved: "text-[var(--color-success)]",
  failed: "text-[var(--color-danger)]",
};

export function DLQ() {
  const { role } = useAuth();
  const { data, error, loading, reload } = useApi(() => controlPlane.get<DLQEventOut[]>("/v1/dlq"), []);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [replaying, setReplaying] = useState(false);
  const [steps, setSteps] = useState<ReplayStep[] | null>(null);

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const updateStep = (eventId: string, patch: Partial<ReplayStep>) => {
    setSteps((prev) => (prev ? prev.map((s) => (s.eventId === eventId ? { ...s, ...patch } : s)) : prev));
  };

  // Replays one event at a time (rather than the single batch call the API
  // also supports) specifically so this can show real, individual progress
  // as each call actually completes — not a simulated progress bar. Each
  // row's status reflects the real HTTP response for that one event: the
  // backend looks the DLQ row up, republishes its raw_ref to
  // logkrama.raw.refs (the real processor picks it up and re-runs
  // IDENTIFY→PARSE→NORMALIZE→ENRICH→VALIDATE→ROUTE), and only reports it
  // resolved once Kafka actually accepted that publish.
  const replaySelected = async () => {
    const ids = Array.from(selected);
    setSteps(ids.map((eventId) => ({ eventId, status: "queued", detail: "" })));
    setReplaying(true);
    for (const eventId of ids) {
      updateStep(eventId, { status: "in-flight", detail: "publishing raw_ref to logkrama.raw.refs…" });
      try {
        const resp = await controlPlane.post<{ resolved: string[]; not_found: string[] }>("/v1/dlq/replay", {
          event_ids: [eventId],
        });
        if (resp.resolved.includes(eventId)) {
          updateStep(eventId, { status: "resolved", detail: "re-queued — will re-run IDENTIFY→ROUTE" });
        } else {
          updateStep(eventId, { status: "failed", detail: "not found in DLQ (already resolved elsewhere?)" });
        }
      } catch (err) {
        updateStep(eventId, { status: "failed", detail: err instanceof Error ? err.message : "replay request failed" });
      }
    }
    setSelected(new Set());
    setReplaying(false);
    reload();
  };

  const grouped = groupByReason(data ?? []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-[16px] font-semibold text-[var(--color-text)]">Dead Letter Queue</h1>
        {canWrite(role) && selected.size > 0 && (
          <Button variant="primary" onClick={replaySelected} disabled={replaying}>
            {replaying ? "Replaying…" : `Replay ${selected.size} event${selected.size === 1 ? "" : "s"}`}
          </Button>
        )}
      </div>

      {steps && (
        <Card
          title="Replay in progress — one event at a time, each row is a real API call"
          action={
            !replaying && (
              <button
                onClick={() => setSteps(null)}
                className="rounded border border-[var(--color-border-strong)] px-2 py-0.5 text-[11px] text-[var(--color-text-secondary)] hover:bg-white/5"
              >
                Dismiss
              </button>
            )
          }
        >
          <div className="space-y-1.5">
            {steps.map((s) => (
              <div key={s.eventId} className="flex items-center gap-3 rounded px-2 py-1.5 text-[12px]">
                <span className={`w-4 shrink-0 text-center font-mono-data text-[13px] ${STEP_CLASS[s.status]}`} aria-hidden="true">
                  {STEP_ICON[s.status]}
                </span>
                <span className="w-[280px] shrink-0 truncate font-mono-data text-[var(--color-text)]" title={s.eventId}>
                  {s.eventId}
                </span>
                <span className={`truncate ${s.status === "failed" ? "text-[var(--color-danger)]" : "text-[var(--color-text-muted)]"}`}>
                  {s.detail || "waiting…"}
                </span>
              </div>
            ))}
          </div>
          {!replaying && (
            <p className="mt-3 border-t border-[var(--color-border)] pt-3 text-[11px] text-[var(--color-text-muted)]">
              "Resolved" means the event was successfully re-queued for another pass through the pipeline — it
              confirms Kafka accepted the republish, not that the event will now pass validation. If the underlying
              parser issue isn't fixed yet, a replayed event can land back in this DLQ under a new entry.
            </p>
          )}
        </Card>
      )}

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {Object.entries(grouped).map(([reason, items]) => (
          <div key={reason} className="rounded border border-[var(--color-border)] px-3 py-2">
            <div className="font-mono-data text-[12px] text-[var(--color-text-secondary)]">{reason}</div>
            <div className="text-xl font-semibold text-[var(--color-warning)]">{items.length}</div>
          </div>
        ))}
      </div>

      <Card title="Events">
        {loading && <LoadingState />}
        {error && <ErrorState message={error} />}
        {!loading && !error && (!data || data.length === 0) && <EmptyState message="DLQ is empty." />}
        {!loading && data && data.length > 0 && (
          <table className="w-full text-left text-[12px]">
            <thead>
              <tr className="border-b border-[var(--color-border)] text-[11px] uppercase tracking-wide text-[var(--color-text-muted)]">
                <th className="w-8 py-2"></th>
                <th className="py-2 pr-4">Event ID</th>
                <th className="py-2 pr-4">Reason</th>
                <th className="py-2 pr-4">Parser</th>
                <th className="py-2 pr-4">Occurred</th>
                <th className="py-2 pr-4">Status</th>
              </tr>
            </thead>
            <tbody>
              {data.map((e) => (
                <tr key={e.event_id} className="border-b border-[var(--color-border)] last:border-0">
                  <td className="py-2">
                    <input
                      type="checkbox"
                      checked={selected.has(e.event_id)}
                      disabled={e.resolved}
                      onChange={() => toggle(e.event_id)}
                    />
                  </td>
                  <td className="py-2 pr-4 font-mono-data">{e.event_id}</td>
                  <td className="py-2 pr-4">
                    <Badge tone="warning">{e.reason}</Badge>
                  </td>
                  <td className="py-2 pr-4 font-mono-data text-[var(--color-text-secondary)]">{e.parser_id ?? "—"}</td>
                  <td className="py-2 pr-4 text-[var(--color-text-secondary)]">{new Date(e.occurred_at).toLocaleString()}</td>
                  <td className="py-2 pr-4">
                    <Badge tone={e.resolved ? "success" : "neutral"}>{e.resolved ? "resolved" : "open"}</Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}

function groupByReason(events: DLQEventOut[]): Record<string, DLQEventOut[]> {
  const out: Record<string, DLQEventOut[]> = {};
  for (const e of events) {
    out[e.reason] ??= [];
    out[e.reason]!.push(e);
  }
  return out;
}
