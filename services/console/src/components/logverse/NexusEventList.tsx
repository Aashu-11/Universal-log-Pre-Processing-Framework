import { eventsForNode, type NexusNode } from "../../lib/logverse/nexus";
import type { LogVerseEvent } from "../../lib/logverse/types";

export function NexusEventList({ node, events, selectedId, onPick }: {
  node: NexusNode;
  events: LogVerseEvent[];
  selectedId: string | null;
  onPick: (event: LogVerseEvent) => void;
}) {
  const related = eventsForNode(node, events);
  return <section className="rounded-xl border border-[var(--color-accent)]/25 bg-[var(--color-surface)]/95 p-3 shadow-xl" aria-label="Events connected to selected node">
    <div className="flex items-start justify-between gap-2">
      <div className="min-w-0"><div className="text-[9px] uppercase tracking-[0.14em] text-[var(--color-accent-hover)]">{node.kind} investigation</div><h2 className="truncate text-[13px] font-semibold text-white" title={node.label}>{node.label}</h2></div>
      <span className="shrink-0 rounded-full border border-[var(--color-border-strong)] px-2 py-0.5 font-mono text-[10px] text-[var(--color-text-secondary)]">{related.length} events</span>
    </div>
    <p className="mt-1 text-[10px] text-[var(--color-text-muted)]">Select an event to inspect its route and backtrack to raw evidence. Counts use only the loaded window.</p>
    <div className="mt-2 max-h-[180px] space-y-1 overflow-y-auto">
      {related.slice(0, 30).map((event) => <button key={event.eventId} type="button" onClick={() => onPick(event)} className={`flex w-full items-center gap-2 rounded-md border px-2 py-1.5 text-left text-[10px] ${selectedId === event.eventId ? "border-[var(--color-accent)]/55 bg-[var(--color-accent)]/15" : "border-[var(--color-border)] bg-white/[0.02] hover:bg-white/[0.06]"}`}>
        <span className={`h-2 w-2 shrink-0 rounded-full ${event.isDlq ? "bg-[#b06bf0]" : event.visualState === "threat" ? "bg-[var(--color-danger)]" : "bg-[var(--color-cyan)]"}`} />
        <span className="min-w-0 flex-1"><span className="block truncate font-mono text-white">{event.eventId}</span><span className="block truncate text-[var(--color-text-muted)]">{event.isDlq ? `DLQ · ${event.dlqReason || "reason unavailable"}` : `${event.eventAction || "action unavailable"} · ${event.srcIp || "source unavailable"} → ${event.dstIp || "destination unavailable"}`}</span></span>
        <span className="shrink-0 font-mono text-[var(--color-text-muted)]">{Number.isFinite(event.ingestedAtMs) ? new Date(event.ingestedAtMs).toLocaleTimeString() : "—"}</span>
      </button>)}
      {related.length > 30 && <p className="text-[10px] text-[var(--color-text-muted)]">Showing 30 most recent connected events.</p>}
    </div>
  </section>;
}
