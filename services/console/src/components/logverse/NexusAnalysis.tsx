import { useMemo } from "react";

import { relatedEventCount, riskBuckets, volumeBuckets } from "../../lib/logverse/analysis";
import type { LogVerseEvent } from "../../lib/logverse/types";
import { explainDlqReason } from "../../lib/logverse/failure";

export function NexusAnalysis({ events, selected }: { events: LogVerseEvent[]; selected: LogVerseEvent | null }) {
  const buckets = useMemo(() => riskBuckets(events), [events]);
  const volume = useMemo(() => volumeBuckets(events), [events]);
  const highest = Math.max(1, ...volume);
  const total = Math.max(1, events.length);
  const related = selected ? relatedEventCount(selected, events) : 0;

  return <div className="space-y-3">
    <div className="grid grid-cols-2 gap-2">
      <Metric label="Visible events" value={String(events.length)} tone="text-[var(--color-cyan)]" />
      <Metric label="Threat / IOC" value={String(buckets[2]?.count ?? 0)} tone="text-[var(--color-danger)]" />
    </div>
    <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]/95 p-3 shadow-[0_10px_30px_rgba(0,0,0,0.32)] backdrop-blur">
      <div className="mb-2 text-[10px] font-semibold uppercase tracking-[0.13em] text-[var(--color-text-secondary)]">Event volume · loaded window</div>
      <div role="img" aria-label={`Event volume across ${volume.length} time buckets`} className="flex h-14 items-end gap-1">
        {volume.map((count, index) => <div key={index} className="min-w-0 flex-1 rounded-t-sm bg-[var(--color-accent)]/75" style={{ height: `${Math.max(count ? 8 : 2, (count / highest) * 100)}%` }} title={`${count} events`} />)}
      </div>
      <div className="mt-1 flex justify-between font-mono text-[9px] text-[var(--color-text-muted)]"><span>earliest</span><span>latest</span></div>
    </div>
    <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]/95 p-3 shadow-[0_10px_30px_rgba(0,0,0,0.32)] backdrop-blur">
      <div className="mb-2 text-[10px] font-semibold uppercase tracking-[0.13em] text-[var(--color-text-secondary)]">Observed event states</div>
      <div className="flex h-2 overflow-hidden rounded-full bg-white/5">{buckets.map((bucket) => <div key={bucket.label} style={{ width: `${bucket.count / total * 100}%`, backgroundColor: bucket.color }} title={`${bucket.label}: ${bucket.count}`} />)}</div>
      <div className="mt-2 grid grid-cols-2 gap-x-2 gap-y-1 text-[10px]">{buckets.map((bucket) => <div key={bucket.label} className="flex items-center gap-1.5 text-[var(--color-text-secondary)]"><span className="h-2 w-2 rounded-full" style={{ backgroundColor: bucket.color }} />{bucket.label} <span className="ml-auto font-mono text-white">{bucket.count}</span></div>)}</div>
    </div>
    {selected && <div className="rounded-xl border border-[var(--color-accent)]/25 bg-[var(--color-surface)]/95 p-3 shadow-[0_10px_30px_rgba(0,0,0,0.32)] backdrop-blur">
      <div className="mb-2 text-[10px] font-semibold uppercase tracking-[0.13em] text-[var(--color-accent-hover)]">Selected event analysis</div>
      <div className="grid grid-cols-2 gap-2"><Metric label={selected.isDlq ? "Failure codes" : "Risk score"} value={selected.isDlq ? String(explainDlqReason(selected.dlqReason).length) : selected.riskScore.toFixed(1)} tone="text-[var(--color-warning)]" /><Metric label="Related events" value={String(related)} tone="text-[var(--color-cyan)]" /></div>
      <div className="mt-3 space-y-1 text-[10px] text-[var(--color-text-secondary)]">
        {selected.isDlq ? <p>Validation reasons: {selected.dlqReason || "unavailable"}. Risk and enrichment fields are not present in the DLQ record.</p> : <><p>{selected.iocMatch ? `IOC match: ${selected.iocIndicator || "indicator unavailable"}` : "No IOC match reported for this event."}</p><p>{selected.threatCategory ? `Threat category: ${selected.threatCategory}` : "No threat category reported."}</p><p>Parser: {selected.parserId || "unavailable"} · {selected.parseStatus || "status unavailable"}</p></>}
        <p>Related count uses shared observed IPs in the loaded event window.</p>
      </div>
    </div>}
  </div>;
}

function Metric({ label, value, tone }: { label: string; value: string; tone: string }) {
  return <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)]/95 p-3 text-center shadow-[0_10px_30px_rgba(0,0,0,0.32)] backdrop-blur"><div className="text-[9px] uppercase tracking-[0.1em] text-[var(--color-text-muted)]">{label}</div><div className={`mt-1 font-mono text-[22px] font-semibold ${tone}`}>{value}</div></div>;
}
