import { PIPELINE_STAGES } from "../../lib/logverse/types";
import type { LogVerseEvent, SourceNode } from "../../lib/logverse/types";

const STATE_TONE: Record<LogVerseEvent["visualState"], string> = {
  normal: "text-[var(--color-cyan)]",
  elevated: "text-[var(--color-warning)]",
  threat: "text-[var(--color-danger)]",
  dlq: "text-[#b06bf0]",
};

/** Graceful degradation when WebGL isn't available in this browser/
 * environment: the same real data the 3D scene would show, as a plain
 * table — still useful for investigation, not just an error page. Reuses
 * whatever the parent page already fetched; makes no requests of its own. */
export function WebGLFallback({
  sources,
  events,
  onSelect,
}: {
  sources: SourceNode[];
  events: LogVerseEvent[];
  onSelect: (ev: LogVerseEvent) => void;
}) {
  const recent = [...events].sort((a, b) => b.ingestedAtMs - a.ingestedAtMs).slice(0, 40);

  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-[var(--color-warning)]/30 bg-[var(--color-warning)]/10 px-4 py-3 text-[12px] text-[var(--color-text)]">
        <div className="font-semibold text-[var(--color-warning)]">3D view unavailable</div>
        <p className="mt-1 text-[var(--color-text-secondary)]">
          This browser or environment doesn't expose WebGL, so the 3D pipeline scene can't render. Nothing about the
          underlying data is degraded — the same real events, sources and DLQ state are shown below as a table
          instead.
        </p>
      </div>

      <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
        <div className="flex gap-1 border-b border-[var(--color-border)] bg-white/[0.02] px-3 py-2 font-mono-data text-[10px] uppercase tracking-wide text-[var(--color-text-muted)]">
          {PIPELINE_STAGES.map((s) => (
            <span key={s} className="flex-1 text-center">
              {s}
            </span>
          ))}
        </div>
        <table className="w-full text-left text-[11.5px]">
          <thead>
            <tr className="border-b border-[var(--color-border)] bg-white/[0.02]">
              {["event_id", "vendor", "state", "action", "risk"].map((h) => (
                <th key={h} className="px-3 py-1.5 font-mono-data font-medium text-[var(--color-text-secondary)]">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {recent.map((ev) => (
              <tr
                key={ev.eventId}
                tabIndex={0}
                role="button"
                onClick={() => onSelect(ev)}
                onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && onSelect(ev)}
                className="cursor-pointer border-b border-[var(--color-border)] last:border-0 hover:bg-white/5 focus:bg-[var(--color-accent)]/10 focus:outline-none"
              >
                <td className="px-3 py-1.5 font-mono-data text-[var(--color-text)]">{ev.eventId.slice(0, 18)}…</td>
                <td className="px-3 py-1.5 font-mono-data text-[var(--color-text-secondary)]">{ev.vendor}</td>
                <td className={`px-3 py-1.5 font-mono-data font-medium ${STATE_TONE[ev.visualState]}`}>{ev.visualState}</td>
                <td className="px-3 py-1.5 font-mono-data text-[var(--color-text-secondary)]">{ev.eventAction || "—"}</td>
                <td className="px-3 py-1.5 font-mono-data text-[var(--color-text)]">{ev.riskScore.toFixed(1)}</td>
              </tr>
            ))}
            {recent.length === 0 && (
              <tr>
                <td colSpan={5} className="px-3 py-6 text-center text-[var(--color-text-muted)]">
                  No events buffered yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="rounded-lg border border-[var(--color-border)] p-3">
        <div className="mb-2 font-mono-data text-[10px] uppercase tracking-wide text-[var(--color-text-muted)]">
          Sources ({sources.length})
        </div>
        <div className="flex flex-wrap gap-1.5">
          {sources.map((s) => (
            <span key={s.sourceId} className="rounded border border-[var(--color-border-strong)] px-2 py-1 font-mono-data text-[10.5px] text-[var(--color-text-secondary)]">
              {s.name} ({s.vendor})
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}
